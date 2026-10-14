-- Fighters: the entities we measure.
CREATE TABLE fighters (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    race        TEXT        NOT NULL DEFAULT 'unknown',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Readings: one power-level measurement. This is the table that grows.
CREATE TABLE readings (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fighter_id  BIGINT      NOT NULL REFERENCES fighters(id) ON DELETE CASCADE,
    power       BIGINT      NOT NULL CHECK (power >= 0),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The access pattern is "latest readings for a fighter", so the index is
-- (fighter_id, recorded_at DESC). Column ORDER matters — Capsule 11.
CREATE INDEX idx_readings_fighter_time ON readings (fighter_id, recorded_at DESC);

-- Seed data so the service does something on first run.
INSERT INTO fighters (name, race) VALUES
    ('goku', 'saiyan'), ('vegeta', 'saiyan'), ('piccolo', 'namekian'),
    ('krillin', 'human'), ('frieza', 'frost demon'), ('gohan', 'half-saiyan');