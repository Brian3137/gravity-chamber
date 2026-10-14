package scouter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ Store = (*PostgresStore)(nil) // the interface from Capsule 06 pays off

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// Pool sizing is a capacity decision, not a default to accept.
	// total connections = replicas x MaxConns, and it must stay well under
	// the server's max_connections (100 by default).
	cfg.MaxConns = 10
	cfg.MinConns = 2                        // keep a few warm; connecting costs ms
	cfg.MaxConnLifetime = 30 * time.Minute  // rotate so failover isn't sticky
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Fail fast at startup rather than on the first request.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() { s.pool.Close() }

// Ping backs the readiness probe. Liveness must NOT call this (Capsule 01).
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) Record(r Reading) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Upsert the fighter, then insert the reading, atomically.
	// ON CONFLICT makes this idempotent — the same pattern Capsule 19 needs.
	const q = `
		WITH f AS (
			INSERT INTO fighters (name) VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		)
		INSERT INTO readings (fighter_id, power, recorded_at)
		SELECT f.id, $2, $3 FROM f`

	if _, err := s.pool.Exec(ctx, q, r.Fighter, r.Power, r.RecordedAt); err != nil {
		// Store's signature has no error return (Capsule 01's shape).
		// Capsule 17 fixes that; for now, at least don't swallow it silently.
		fmt.Printf(`{"level":"ERROR","msg":"record failed","err":%q}`+"\n", err)
	}
}

func (s *PostgresStore) Latest(fighter string) (Reading, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	const q = `
		SELECT f.name, r.power, r.recorded_at
		FROM readings r
		JOIN fighters f ON f.id = r.fighter_id
		WHERE f.name = $1
		ORDER BY r.recorded_at DESC
		LIMIT 1`

	var out Reading
	err := s.pool.QueryRow(ctx, q, fighter).Scan(&out.Fighter, &out.Power, &out.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reading{}, false
	}
	if err != nil {
		return Reading{}, false
	}
	return out, true
}

// Leaderboard: the latest reading per fighter, top n by power.
// DISTINCT ON is a Postgres-specific "first row per group" — it pairs with
// ORDER BY and is dramatically faster than a self-join or a window function
// when an index covers the ordering.
func (s *PostgresStore) Leaderboard(n int) []Reading {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `
		SELECT name, power, recorded_at FROM (
			SELECT DISTINCT ON (r.fighter_id)
			       f.name, r.power, r.recorded_at
			FROM readings r
			JOIN fighters f ON f.id = r.fighter_id
			ORDER BY r.fighter_id, r.recorded_at DESC
		) latest
		ORDER BY power DESC
		LIMIT $1`

	rows, err := s.pool.Query(ctx, q, n)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]Reading, 0, n)
	for rows.Next() {
		var r Reading
		if err := rows.Scan(&r.Fighter, &r.Power, &r.RecordedAt); err != nil {
			return out
		}
		out = append(out, r)
	}
	return out
}