package scouter

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sync/singleflight"
	"log/slog"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type localCacheEntry struct {
    reading   Reading
    found     bool
    expiresAt time.Time
}

// CachedStore wraps any Store with cache-aside caching in Redis.
// It is a decorator: the wrapped Store has no idea a cache exists.
type CachedStore struct {
	inner Store
	rdb   *redis.Client
	log   *slog.Logger
	ttl   time.Duration
	sf    singleflight.Group
	coalesced atomic.Uint64
	local sync.Map

	hits, misses, errs atomic.Uint64
}

var _ Store = (*CachedStore)(nil)

func NewCachedStore(inner Store, addr string, ttl time.Duration, log *slog.Logger) *CachedStore {
	return &CachedStore{
		inner: inner,
		rdb: redis.NewClient(&redis.Options{
			Addr:         addr,
			DialTimeout:  1 * time.Second,
			ReadTimeout:  200 * time.Millisecond, // a slow cache must not become
			WriteTimeout: 200 * time.Millisecond, // slower than the database
			PoolSize:     20,
		}),
		log: log,
		ttl: ttl,
	}
}

func (c *CachedStore) Close() error { return c.rdb.Close() }

// jitter spreads expiries so keys created together don't expire together.
func (c *CachedStore) jittered() time.Duration {
	return c.ttl + time.Duration(rand.Int63n(int64(c.ttl/10)))
}

func fighterKey(name string) string { return "scouter:fighter:" + name }
func boardKey(n int) string         { return "scouter:board:" + strconv.Itoa(n) }



func (c *CachedStore) Latest(fighter string) (Reading, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	key := fighterKey(fighter)

	// 1. Check local cache first.
	if value, ok := c.local.Load(key); ok {
		entry := value.(localCacheEntry)

		if time.Now().Before(entry.expiresAt) {
			c.hits.Add(1)
			return entry.reading, entry.found
		}

		c.local.Delete(key)
	}

	// 2. Check Redis.
	raw, err := c.rdb.Get(ctx, key).Bytes()
	switch {
	case err == nil:
		if string(raw) == "null" {
			c.hits.Add(1)
			return Reading{}, false
		}

		var r Reading
		if json.Unmarshal(raw, &r) == nil {
			c.hits.Add(1)
			return r, true
		}
		// Corrupt entry — treat as a miss and overwrite below.

	case errors.Is(err, redis.Nil):
		c.misses.Add(1)

	default:
		c.errs.Add(1)
		c.log.Warn("cache read failed", "key", key, "err", err)
	}

	// 3. If Redis misses, check the database.
	r, ok := c.inner.Latest(fighter)
	if !ok {
		c.local.Store(key, localCacheEntry{
			found:     false,
			expiresAt: time.Now().Add(time.Second),
		})

		if err := c.rdb.Set(ctx, key, "null", 30*time.Second).Err(); err != nil {
			c.log.Warn("negative cache write failed", "key", key, "err", err)
		}
		return Reading{}, false
	}

	// 4. Save in local cache.
	c.local.Store(key, localCacheEntry{
		reading:   r,
		found:     true,
		expiresAt: time.Now().Add(time.Second),
	})

	// 5. Save in Redis.
	if b, err := json.Marshal(r); err == nil {
		if err := c.rdb.Set(ctx, key, b, c.jittered()).Err(); err != nil {
			c.log.Warn("cache write failed", "key", key, "err", err)
		}
	}

	return r, true
}


func (c *CachedStore) Leaderboard(n int) []Reading {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	key := boardKey(n)

	if raw, err := c.rdb.Get(ctx, key).Bytes(); err == nil {
		var out []Reading
		if json.Unmarshal(raw, &out) == nil {
			c.hits.Add(1)
			return out
		}
	} else if !errors.Is(err, redis.Nil) {
		c.errs.Add(1)
		c.log.Warn("cache read failed", "key", key, "err", err)
	} else {
		c.misses.Add(1)
	}

	v, err, shared := c.sf.Do(key, func() (any, error) {
		var out []Reading

		if slow, ok := c.inner.(interface {
			SlowLeaderboard(int) []Reading
		}); ok {
			out = slow.SlowLeaderboard(n)
		} else {
			out = c.inner.Leaderboard(n)
		}

		if b, err := json.Marshal(out); err == nil {
			bg, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_ = c.rdb.Set(bg, key, b, c.jittered()).Err()
		}

		return out, nil
	})

	if shared {
		c.coalesced.Add(1)
	}

	if err != nil {
		return nil
	}

	return v.([]Reading)
}


func (c *CachedStore) Record(r Reading) {
	c.inner.Record(r) // database first: it is the source of truth

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// INVALIDATE, don't update. Two concurrent writers updating the cache can
	// interleave and leave the older value in place forever. Deletion can't.
	if err := c.rdb.Del(ctx, fighterKey(r.Fighter)).Err(); err != nil {
		c.log.Warn("invalidate failed", "fighter", r.Fighter, "err", err)
	}

	// The leaderboard is derived from all fighters, so any write invalidates
	// every variant. Scanning keys is fine at this scale; at real scale you
	// would keep the ranking in a Redis sorted set instead (see below).
	iter := c.rdb.Scan(ctx, 0, "scouter:board:*", 100).Iterator()
	for iter.Next(ctx) {
		_ = c.rdb.Del(ctx, iter.Val()).Err()
	}
}

// Stats reports hit ratio. Export these as metrics in Capsule 25.
func (c *CachedStore) Stats() (hits, misses, errs uint64, ratio float64) {
	h, m, e := c.hits.Load(), c.misses.Load(), c.errs.Load()
	if h+m > 0 {
		ratio = float64(h) / float64(h+m)
	}
	return h, m, e, ratio
}
