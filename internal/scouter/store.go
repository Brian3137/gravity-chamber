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

// Store is the persistence boundary. MemStore implements it today and is
// wrong for more than one replica; Capsule 10 adds a Postgres implementation
// and nothing else in the service has to change.
type Store interface {
	Record(r Reading)
	Latest(fighter string) (Reading, bool)
	Leaderboard(n int) []Reading
}

// Compile-time proof that MemStore satisfies Store. If it ever stops
// matching, the build breaks here rather than at some call site.
var _ Store = (*MemStore)(nil)

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
