package storage

import (
	"sync"

	"typingtrainer/internal/engine"
)

// MemoryStore keeps everything in RAM. Data is lost when the server stops.
// The mutex makes it safe when several requests arrive at the same time.
type MemoryStore struct {
	mu       sync.Mutex
	stats    map[string]engine.KeyStat
	unlocked int
	sessions []Session
}

func (m *MemoryStore) Unlocked() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unlocked, nil
}

func (m *MemoryStore) SetUnlocked(n int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unlocked = n
	return nil
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{stats: map[string]engine.KeyStat{}}
}

// KeyStats returns a COPY, so callers cannot change our data by accident.
func (m *MemoryStore) KeyStats() (map[string]engine.KeyStat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]engine.KeyStat, len(m.stats))
	for k, v := range m.stats {
		out[k] = v
	}
	return out, nil
}

func (m *MemoryStore) SaveKeyStats(stats map[string]engine.KeyStat) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = make(map[string]engine.KeyStat, len(stats))
	for k, v := range stats {
		m.stats[k] = v
	}
	return nil
}

func (m *MemoryStore) AddSession(s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = append(m.sessions, s)
	return nil
}

func (m *MemoryStore) RecentSessions(limit int) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Session{}
	for i := len(m.sessions) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.sessions[i])
	}
	return out, nil
}
