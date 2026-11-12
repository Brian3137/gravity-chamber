package scouter

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// Retry runs fn with exponential backoff and FULL jitter.
//
// Full jitter (sleep = rand(0, base*2^n)) beats both no-jitter and
// half-jitter: without it, every client that failed at the same moment
// retries at the same moment, and you have built a self-inflicted DDoS
// that fires exactly when the dependency is trying to recover.
//
// Only ever wrap IDEMPOTENT operations in this.
func Retry(ctx context.Context, p RetryPolicy, fn func(ctx context.Context) error) error {
	var err error
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(p.BaseDelay) * math.Pow(2, float64(attempt-1)))
			if backoff > p.MaxDelay {
				backoff = p.MaxDelay
			}
			jittered := time.Duration(rand.Int63n(int64(backoff) + 1))

			select {
			case <-time.After(jittered):
			case <-ctx.Done():
				// The caller's deadline is gone. Retrying now is pure waste.
				return ctx.Err()
			}
		}

		if err = fn(ctx); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return err
}