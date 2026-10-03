// Package api is the HTTP layer: it turns JSON requests into calls to the
// engine and the store, and turns the answers back into JSON.
// All routes start with /api/v1/.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	"typingtrainer/internal/config"
	"typingtrainer/internal/engine"
	"typingtrainer/internal/storage"
)

type Server struct {
	cfg        config.Config
	store      storage.Store
	gen        *engine.Generator
	layoutJSON []byte

	// genMu: the generator's random source is not safe for parallel use.
	// writeMu: makes "read stats -> update -> save" one atomic step.
	// (A PostgreSQL store would do this inside a transaction instead.)
	genMu   sync.Mutex
	writeMu sync.Mutex
}

func NewServer(cfg config.Config, store storage.Store, gen *engine.Generator, layoutJSON []byte) *Server {
	return &Server{cfg: cfg, store: store, gen: gen, layoutJSON: layoutJSON}
}

// Register adds all API routes to the mux (Go 1.22+ "METHOD /path" patterns).
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/layout", s.handleLayout)
	mux.HandleFunc("GET /api/v1/exercise", s.handleExercise)
	mux.HandleFunc("POST /api/v1/results", s.handleResults)
	mux.HandleFunc("POST /api/v1/progress", s.handleProgress)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
}

// GET /api/v1/layout -> the keyboard layout JSON, as is.
func (s *Server) handleLayout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.layoutJSON)
}

// GET /api/v1/exercise -> text + which letters are open / locked / in focus.
func (s *Server) handleExercise(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.KeyStats()
	if err != nil {
		serverError(w, err)
		return
	}
	unlocked, err := s.store.Unlocked()
	if err != nil {
		serverError(w, err)
		return
	}
	s.genMu.Lock()
	ex := s.gen.Next(stats, unlocked, s.cfg.Engine)
	s.genMu.Unlock()
	writeJSON(w, http.StatusOK, ex)
}

type resultsRequest struct {
	Samples  []engine.Sample `json:"samples"`
	WPM      float64         `json:"wpm"`
	Accuracy float64         `json:"accuracy"`
}

// POST /api/v1/results -> stores a finished exercise, maybe unlocks a letter.
// Answer: {"unlocked": 8, "newLetter": "s"} (newLetter is "" if nothing opened).
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB is plenty
	var req resultsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if msg := validate(req, s.cfg.MaxEvents); msg != "" {
		badRequest(w, msg)
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	stats, err := s.store.KeyStats()
	if err != nil {
		serverError(w, err)
		return
	}
	stats = engine.Apply(stats, req.Samples, s.cfg.Engine)
	if err := s.store.SaveKeyStats(stats); err != nil {
		serverError(w, err)
		return
	}

	before, err := s.store.Unlocked()
	if err != nil {
		serverError(w, err)
		return
	}
	before = s.gen.Clamp(before)
	after := s.gen.NextUnlocked(stats, before, s.cfg.Engine)
	if err := s.store.SetUnlocked(after); err != nil {
		serverError(w, err)
		return
	}

	err = s.store.AddSession(storage.Session{
		WPM: req.WPM, Accuracy: req.Accuracy, Chars: len(req.Samples), FinishedAt: time.Now(),
	})
	if err != nil {
		serverError(w, err)
		return
	}
	newLetter := ""
	if after > before {
		newLetter = s.gen.Order()[after-1]
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": after, "newLetter": newLetter})
}

// POST /api/v1/progress {"unlockAll": true} -> for people who can already
// touch type: skip the lessons and open every letter. {"unlockAll": false}
// goes back to the first lesson (statistics are kept).
func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UnlockAll bool `json:"unlockAll"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	n := s.gen.Start()
	if req.UnlockAll {
		n = len(s.gen.Order())
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.store.SetUnlocked(n); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unlocked": n})
}

func validate(req resultsRequest, maxEvents int) string {
	if len(req.Samples) == 0 || len(req.Samples) > maxEvents {
		return "samples: wrong number of samples"
	}
	for _, smp := range req.Samples {
		if utf8.RuneCountInString(smp.Char) != 1 || smp.Ms < 0 || smp.Errors < 0 {
			return "samples: each needs exactly one char, errors >= 0 and ms >= 0"
		}
	}
	if req.WPM < 0 || req.WPM > 400 || req.Accuracy < 0 || req.Accuracy > 100 {
		return "wpm or accuracy out of range"
	}
	return ""
}

type keyInfo struct {
	Char       string  `json:"char"`
	Count      int     `json:"count"`
	ErrorRate  float64 `json:"errorRate"` // smoothed (EMA)
	AvgMs      float64 `json:"avgMs"`     // smoothed (EMA)
	Difficulty float64 `json:"difficulty"`
	Mastered   bool    `json:"mastered"`
	Unlocked   bool    `json:"unlocked"`
}

// GET /api/v1/stats -> every letter (hardest first) + recent sessions.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.KeyStats()
	if err != nil {
		serverError(w, err)
		return
	}
	unlocked, err := s.store.Unlocked()
	if err != nil {
		serverError(w, err)
		return
	}
	unlocked = s.gen.Clamp(unlocked)
	p := s.cfg.Engine
	keys := []keyInfo{}
	for i, c := range s.gen.Order() {
		st := stats[c]
		keys = append(keys, keyInfo{
			Char: c, Count: st.Count, ErrorRate: st.ErrEMA, AvgMs: st.TimeEMA,
			Difficulty: engine.Difficulty(st, p), Mastered: engine.Mastered(st, p),
			Unlocked: i < unlocked,
		})
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].Difficulty > keys[j].Difficulty })
	sessions, err := s.store.RecentSessions(10)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys, "sessions": sessions, "targetWpm": p.TargetWPM})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("error:", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
