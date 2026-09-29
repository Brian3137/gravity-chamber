package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brian3137/gravity-chamber/internal/gateway"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	upstream := os.Getenv("SCOUTER_URL")
	if upstream == "" {
		upstream = "http://localhost:8080"
	}

	// Keys live in the environment, never in code:  KEYS="key1:web,key2:mobile"
	keys := map[string]string{}
	for _, pair := range strings.Split(os.Getenv("API_KEYS"), ",") {
		if k, v, ok := strings.Cut(pair, ":"); ok {
			keys[k] = v
		}
	}
	if len(keys) == 0 {
		log.Error("API_KEYS is required, e.g. API_KEYS=kamehameha:web")
		os.Exit(1)
	}

	g, err := gateway.New(gateway.Config{
		Routes: []gateway.Route{
			{Prefix: "/healthz", Target: upstream, Public: true},
			{Prefix: "/api/", Target: upstream, Strip: true},
		},
		APIKeys:      keys,
		RateLimit:    30,
		UpstreamWait: 10 * time.Second,
	}, log)
	if err != nil {
		log.Error("bad gateway config", "err", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           g.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	go func() {
		log.Info("king yemma's desk is open", "port", port, "upstream", upstream)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("gateway failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Info("desk closed")
}