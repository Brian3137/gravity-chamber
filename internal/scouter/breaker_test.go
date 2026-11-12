package scouter

import (
	"errors"
	"testing"
	"time"
)

func TestBreakerOpensAndRecovers(t *testing.T) {
	b := NewBreaker(3, 50*time.Millisecond)
	boom := errors.New("boom")

	for i := 0; i < 3; i++ {
		if err := b.Do(func() error { return boom }); !errors.Is(err, boom) {
			t.Fatalf("call %d: got %v", i, err)
		}
	}
	if b.State() != "open" {
		t.Fatalf("state = %s, want open", b.State())
	}

	// While open, calls fail instantly without invoking fn.
	called := false
	if err := b.Do(func() error { called = true; return nil }); !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("got %v, want ErrCircuitOpen", err)
	}
	if called {
		t.Error("fn was called while the circuit was open")
	}

	time.Sleep(60 * time.Millisecond) // cooldown

	for i := 0; i < 3; i++ {
		if err := b.Do(func() error { return nil }); err != nil {
			t.Fatalf("recovery call %d: %v", i, err)
		}
	}
	if b.State() != "closed" {
		t.Errorf("state = %s, want closed", b.State())
	}
}