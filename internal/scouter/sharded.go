package scouter

import (
	"context"
	"hash/fnv"
	"sort"
	"sync"
)

// ShardedStore routes by fighter name. Note what this class does NOT offer:
// cross-shard transactions, cross-shard joins, or a way to reshard without
// moving data. That is the deal you sign when you shard.
type ShardedStore struct {
	shards []*PostgresStore
}

var _ Store = (*ShardedStore)(nil)

func NewShardedStore(ctx context.Context, dsns []string) (*ShardedStore, error) {
	s := &ShardedStore{}
	for _, dsn := range dsns {
		p, err := NewPostgresStore(ctx, dsn)
		if err != nil {
			return nil, err
		}
		s.shards = append(s.shards, p)
	}
	return s, nil
}

func (s *ShardedStore) Close() {
	for _, p := range s.shards {
		p.Close()
	}
}

// shardFor is naive modulo hashing: changing len(shards) remaps almost every
// key. Capsule 15 replaces this with a consistent hash ring.
func (s *ShardedStore) shardFor(key string) *PostgresStore {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return s.shards[int(h.Sum32())%len(s.shards)]
}

// Single-shard operations: cheap. The key is in the query.
func (s *ShardedStore) Record(r Reading) { s.shardFor(r.Fighter).Record(r) }

func (s *ShardedStore) Latest(fighter string) (Reading, bool) {
	return s.shardFor(fighter).Latest(fighter)
}

// Leaderboard is a SCATTER-GATHER: no shard key, so every shard is queried.
// Correctness note: to get a global top-n you must take the top-n from EACH
// shard. Taking top-n/len(shards) each would be wrong — one shard may hold
// all the strongest fighters.
func (s *ShardedStore) Leaderboard(n int) []Reading {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		all []Reading
	)
	for _, sh := range s.shards {
		wg.Add(1)
		go func(p *PostgresStore) {
			defer wg.Done()
			part := p.Leaderboard(n) // full n from every shard
			mu.Lock()
			all = append(all, part...)
			mu.Unlock()
		}(sh)
	}
	wg.Wait() // total latency = the SLOWEST shard, not the average

	sort.Slice(all, func(i, j int) bool { return all[i].Power > all[j].Power })
	if len(all) > n {
		all = all[:n]
	}
	return all
}