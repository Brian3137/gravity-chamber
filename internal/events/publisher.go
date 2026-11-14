// Package events is the messaging boundary. Nothing outside it should know
// which broker is in use.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/pubsub"
)

// PowerRecorded is an EVENT: a statement of fact about something that already
// happened, in the past tense. Not a command ("UpdateLeaderboard") — the
// publisher must not know or care who reacts to it.
type PowerRecorded struct {
	EventID    string    `json:"event_id"`   // for idempotency (Capsule 19)
	Fighter    string    `json:"fighter"`
	Power      int64     `json:"power"`
	RecordedAt time.Time `json:"recorded_at"`
	Version    int       `json:"version"`    // schema version, from day one
}

type Publisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
	log    *slog.Logger
}

func NewPublisher(ctx context.Context, projectID, topicID string, log *slog.Logger) (*Publisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client: %w", err)
	}

	topic := client.Topic(topicID)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("topic exists: %w", err)
	}
	if !ok {
		if topic, err = client.CreateTopic(ctx, topicID); err != nil {
			return nil, fmt.Errorf("create topic: %w", err)
		}
	}

	// REQUIRED because we set an OrderingKey below. Without it, every single
	// Publish fails with "EnableMessageOrdering=false, but an OrderingKey was
	// set" — and since we deliberately log-and-continue on publish errors, the
	// API still returns 201 and you get NO events with no obvious cause.
	// This is a publisher-side client setting; creating the topic is not enough.
	topic.EnableMessageOrdering = true

	// Batching trades a little latency for a lot of throughput.
	topic.PublishSettings.DelayThreshold = 20 * time.Millisecond
	topic.PublishSettings.CountThreshold = 100

	return &Publisher{client: client, topic: topic, log: log}, nil
}

func (p *Publisher) Close() {
	p.topic.Stop() // flushes buffered messages — do not skip this
	_ = p.client.Close()
}

func (p *Publisher) PublishPowerRecorded(ctx context.Context, e PowerRecorded) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}

	res := p.topic.Publish(ctx, &pubsub.Message{
		Data: body,
		// Attributes are filterable by subscriptions WITHOUT decoding the
		// body — put routing metadata here, not in the payload.
		Attributes: map[string]string{
			"event_type": "power.recorded",
			"fighter":    e.Fighter,
			"version":    "1",
		},
		// Same key => same order. Different fighters stay parallel.
		OrderingKey: e.Fighter,
	})

	// Publish is async; Get blocks for the server ack. Doing this inline
	// makes the publish synchronous from the caller's point of view — which
	// is what you want when the caller must know the event was accepted.
	id, err := res.Get(ctx)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	p.log.Debug("published", "message_id", id, "fighter", e.Fighter)
	return nil
}