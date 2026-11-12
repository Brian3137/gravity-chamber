package scouter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySucceedsOnThirdAttempt(t *testing.T) {
	calls := 0
	err := Retry(context.Background(),
		RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond},
		func(context.Context) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want nil/3", err, calls)
	}
}

func TestRetryStopsWhenContextExpires(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	calls := 0
	err := Retry(ctx,
		RetryPolicy{MaxAttempts: 100, BaseDelay: 10 * time.Millisecond, MaxDelay: time.Second},
		func(context.Context) error { calls++; return errors.New("always") })

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want DeadlineExceeded", err)
	}
	if calls > 5 {
		t.Errorf("made %d calls after the deadline — retries ignored the context", calls)
	}
}