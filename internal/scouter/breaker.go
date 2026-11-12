package scouter

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker open")

type breakerState int

const (
	closed breakerState = iota
	open
	halfOpen
)

// Breaker stops calling a dependency that is clearly failing.
// The point is not to protect the dependency — it is to stop YOU from
// holding thousands of goroutines waiting on something that will not answer.
type Breaker struct {
	mu sync.Mutex

	state        breakerState
	failures     int
	successes    int
	openedAt     time.Time

	threshold    int           // consecutive failures that trip it
	cooldown     time.Duration // how long to stay open
	halfOpenNeed int           // consecutive successes to close again
}

func NewBreaker(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		state:        closed,
		threshold:    threshold,
		cooldown:     cooldown,
		halfOpenNeed: 3,
	}
}

func (b *Breaker) Do(fn func() error) error {
	if !b.allow() {
		return ErrCircuitOpen // fails INSTANTLY: no timeout paid
	}
	err := fn()
	b.record(err == nil)
	return err
}

func (b *Breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case open:
		if time.Since(b.openedAt) > b.cooldown {
			b.state = halfOpen
			b.successes = 0
			return true // let one through to probe
		}
		return false
	default:
		return true
	}
}

func (b *Breaker) record(ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ok {
		switch b.state {
		case halfOpen:
			b.successes++
			if b.successes >= b.halfOpenNeed {
				b.state = closed
				b.failures = 0
			}
		default:
			b.failures = 0
		}
		return
	}

	b.failures++
	if b.state == halfOpen || b.failures >= b.threshold {
		b.state = open
		b.openedAt = time.Now()
	}
}

func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return [...]string{"closed", "open", "half-open"}[b.state]
}