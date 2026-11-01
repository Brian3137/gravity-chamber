package scouter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicatedStore sends writes to the primary and reads to replicas,
// with read-your-writes protection: for a short window after a fighter is
// written, that fighter's reads are pinned to the primary.
type ReplicatedStore struct {
	primary  *PostgresStore
	replicas []*PostgresStore
	next     atomic.Uint64

	// recentWrites: fighter -> deadline until which reads must hit the primary.
	// A per-key map is a deliberate choice: pinning ALL reads to the primary
	// after ANY write would defeat the purpose of having replicas.
	mu           sync.RWMutex
	recentWrites map[string]time.Time
	window       time.Duration
}

var _ Store = (*ReplicatedStore)(nil)

func NewReplicatedStore(primary *PostgresStore, replicas []*PostgresStore, window time.Duration) *ReplicatedStore {
	s := &ReplicatedStore{
		primary:      primary,
		replicas:     replicas,
		recentWrites: make(map[string]time.Time),
		window:       window,
	}
	go s.reap()
	return s
}

// readFor picks primary or a replica, round-robin.
func (s *ReplicatedStore) readFor(fighter string) *PostgresStore {
	if len(s.replicas) == 0 {
		return s.primary
	}
	s.mu.RLock()
	until, pinned := s.recentWrites[fighter]
	s.mu.RUnlock()
	if pinned && time.Now().Before(until) {
		return s.primary // read-your-writes
	}
	i := s.next.Add(1)
	return s.replicas[int(i)%len(s.replicas)]
}

func (s *ReplicatedStore) Record(r Reading) {
	s.primary.Record(r)
	s.mu.Lock()
	s.recentWrites[r.Fighter] = time.Now().Add(s.window)
	s.mu.Unlock()
}

func (s *ReplicatedStore) Latest(fighter string) (Reading, bool) {
	return s.readFor(fighter).Latest(fighter)
}

// Leaderboard is not tied to one fighter, so it always reads a replica.
// That is a deliberate consistency trade: a leaderboard may be a second
// stale, and nobody will ever notice.
func (s *ReplicatedStore) Leaderboard(n int) []Reading {
	if len(s.replicas) == 0 {
		return s.primary.Leaderboard(n)
	}
	i := s.next.Add(1)
	return s.replicas[int(i)%len(s.replicas)].Leaderboard(n)
}

func (s *ReplicatedStore) Ping(ctx context.Context) error { return s.primary.Ping(ctx) }

func (s *ReplicatedStore) reap() {
	t := time.NewTicker(time.Minute)
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for k, v := range s.recentWrites {
			if now.After(v) {
				delete(s.recentWrites, k)
			}
		}
		s.mu.Unlock()
	}
}
