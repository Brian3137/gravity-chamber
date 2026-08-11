package scouter

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	LogLevel        string
	ShutdownTimeout time.Duration
	DrainDelay      time.Duration
}

func LoadConfig() (Config, error) {
	c := Config{
		Port:            env("PORT", "8080"),
		LogLevel:        env("LOG_LEVEL", "info"),
		ShutdownTimeout: envDur("SHUTDOWN_TIMEOUT", 15*time.Second),
		DrainDelay:      envDur("DRAIN_DELAY", 2*time.Second),
	}

	if _, err := strconv.Atoi(c.Port); err != nil {
		return c, fmt.Errorf("PORT %q is not a number", c.Port)
	}

	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envDur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
