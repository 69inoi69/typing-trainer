// Entry point: loads the data files, wires the packages together and
// starts the web server.
package main

import (
	"embed"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"typingtrainer/internal/api"
	"typingtrainer/internal/config"
	"typingtrainer/internal/engine"
	"typingtrainer/internal/layout"
	"typingtrainer/internal/storage"
)

// The web/ and data/ folders are baked into the program at build time,
// so it works no matter which folder you start it from.
//
//go:embed web data
var files embed.FS

func main() {
	cfg := config.Default()

	layoutJSON, err := files.ReadFile("data/layouts/qwerty-en.json")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := layout.Parse(layoutJSON); err != nil { // fail early on a broken layout
		log.Fatal(err)
	}

	wordData, err := files.ReadFile("data/words-en.txt")
	if err != nil {
		log.Fatal(err)
	}
	words := uniqueWords(string(wordData))

	// Swap this line for storage.NewPostgresStore(...) later.
	var store storage.Store = storage.NewMemoryStore()

	gen := engine.NewGenerator(words, rand.New(rand.NewSource(time.Now().UnixNano())), cfg.Engine)
	server := api.NewServer(cfg, store, gen, layoutJSON)

	mux := http.NewServeMux()
	server.Register(mux)
	webFS, _ := fs.Sub(files, "web")
	mux.Handle("/", http.FileServerFS(webFS))

	log.Printf("Typing trainer running. Open http://%s in your browser", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

// uniqueWords splits the word list (most frequent first), lowercases it,
// keeps only words made of the letters a-z and removes duplicates.
// The order is kept: it is the frequency order of the source list.
func uniqueWords(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if !seen[w] && onlyLetters(w) {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func onlyLetters(w string) bool {
	for _, r := range w {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
