// Package storage hides WHERE data is kept behind the Store interface.
// Right now there is an in-memory version (memory.go). Later you add
// postgres.go implementing the same interface and change one line in main.go.
package storage

import (
	"time"

	"typingtrainer/internal/engine"
)

// Session is the summary of one finished exercise.
type Session struct {
	WPM        float64   `json:"wpm"`
	Accuracy   float64   `json:"accuracy"` // 0..100
	Chars      int       `json:"chars"`
	FinishedAt time.Time `json:"finishedAt"`
}

// Store is every data operation the app needs. Nothing outside this
// package should know how the data is actually stored.
type Store interface {
	KeyStats() (map[string]engine.KeyStat, error)
	SaveKeyStats(stats map[string]engine.KeyStat) error

	// Unlocked is how many letters of the unlock order are open (0 = new user).
	Unlocked() (int, error)
	SetUnlocked(n int) error

	AddSession(s Session) error
	RecentSessions(limit int) ([]Session, error) // newest first
}
