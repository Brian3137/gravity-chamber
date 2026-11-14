package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/brian3137/gravity-chamber/internal/events"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	projectID := os.Getenv("PUBSUB_PROJECT_ID")
	subID := os.Getenv("SUBSCRIPTION_ID")
	if projectID == "" || subID == "" {
		log.Error("PUBSUB_PROJECT_ID and SUBSCRIPTION_ID are required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Error("pubsub client", "err", err)
		os.Exit(1)
	}
	defer client.Close()

	sub := client.Subscription(subID)

	// Flow control bounds how much work is in flight. Without it, a large
	// backlog is pulled into memory all at once and you OOM — the async
	// equivalent of an unbounded queue (Capsule 17).
	sub.ReceiveSettings.MaxOutstandingMessages = 100
	sub.ReceiveSettings.MaxOutstandingBytes = 32 * 1024 * 1024
	sub.ReceiveSettings.NumGoroutines = 4

	var processed atomic.Uint64
	log.Info("ranker online", "subscription", subID)

	// Receive blocks until ctx is cancelled, running the callback
	// concurrently for each message.
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		var e events.PowerRecorded
		if err := json.Unmarshal(m.Data, &e); err != nil {
			// A message we can NEVER parse must not be retried forever.
			// Ack it so it leaves the subscription, and log loudly.
			// Capsule 19 sends these to a dead-letter topic instead.
			log.Error("undecodable message, dropping",
				"id", m.ID, "err", err, "data", string(m.Data))
			m.Ack()
			return
		}

		if err := handle(ctx, e); err != nil {
			log.Warn("handler failed, nacking for redelivery",
				"event_id", e.EventID, "err", err)
			m.Nack() // explicit nack = immediate redelivery, not a 10s stall
			return
		}

		n := processed.Add(1)
		log.Info("ranked",
			"event_id", e.EventID,
			"fighter", e.Fighter,
			"power", e.Power,
			"delivery_attempt", deliveryAttempt(m),
			"publish_lag_ms", time.Since(m.PublishTime).Milliseconds(),
			"total", n,
		)
		m.Ack()
	})
	if err != nil {
		log.Error("receive stopped", "err", err)
		os.Exit(1)
	}
	log.Info("ranker offline", "processed", processed.Load())
}

func handle(ctx context.Context, e events.PowerRecorded) error {
	// Real work goes here: update a Redis sorted set, recompute a rank, etc.
	// It MUST be idempotent — this may be the second time you've seen this
	// exact event. That is Capsule 19.
	select {
	case <-time.After(50 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func deliveryAttempt(m *pubsub.Message) int {
	if a := m.DeliveryAttempt; a != nil {
		return *a
	}
	return 1
}
