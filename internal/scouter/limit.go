
package scouter

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter holds one token bucket per caller.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*entry
	rate    rate.Limit
	burst   int
	idleFor time.Duration
}

type entry struct {
	lim  *rate.Limiter
	seen time.Time
}

func NewLimiter(perSecond float64, burst int) *Limiter {
	l := &Limiter{
		buckets: make(map[string]*entry),
		rate:    rate.Limit(perSecond),
		burst:   burst,
		idleFor: 10 * time.Minute,
	}
	go l.reap()
	return l
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	e, ok := l.buckets[key]
	if !ok {
		e = &entry{lim: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[key] = e
	}
	e.seen = time.Now()
	l.mu.Unlock()
	return e.lim.Allow()
}

func (l *Limiter) reap() {
	t := time.NewTicker(time.Minute)
	for range t.C {
		cut := time.Now().Add(-l.idleFor)
		l.mu.Lock()
		for k, e := range l.buckets {
			if e.seen.Before(cut) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) Middleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if !l.Allow(key) {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(l.rate)))
				writeErr(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
