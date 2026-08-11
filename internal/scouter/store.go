package scouter

import (
	"sort"
	"sync"
	"time"
)

// Reading is one power-level measurement of one fighter.
type Reading struct {
	Fighter    string    `json:"fighter"`
	Power      int64     `json:"power"`
	RecordedAt time.Time `json:"recorded_at"`
}

// MemStore is an in-memory store.
// it dies with the process and cannot be shared between replicas.
// Capsule 06 will make that hurt, and Capsule 10 will fix it.
type MemStore struct {
	mu      sync.RWMutex
	byFight map[string][]Reading
}

func NewMemStore() *MemStore {
	return &MemStore{byFight: make(map[string][]Reading)}
}

func (s *MemStore) Record(r Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byFight[r.Fighter] = append(s.byFight[r.Fighter], r)
}
func (s *MemStore) Latest(fighter string) (Reading, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rs := s.byFight[fighter]
	if len(rs) == 0 {
		return Reading{}, false
	}
	return rs[len(rs)-1], true
}

func (s *MemStore) Leaderboard(n int) []Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Reading, 0, len(s.byFight))
	for _, rs := range s.byFight {
		out = append(out, rs[len(rs)-1])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Power > out[j].Power })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
