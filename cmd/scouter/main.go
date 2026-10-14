package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brian3137/gravity-chamber/internal/scouter"
)

func main() {
	// A self-check mode so container healthchecks work on a distroless image
	// with no shell and no curl. Exits 0 if the service answers, 1 otherwise.
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		resp, err := http.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, err := scouter.LoadConfig()

	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}

	log := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: level},
		),
	)

		// Every log line now carries the instance identity. In a container the
	// hostname is the container ID; on Cloud Run, set INSTANCE_ID from the
	// metadata server or just let the hostname be the revision instance.
	host, _ := os.Hostname()
	log = log.With("instance", host)

	//srv := scouter.NewServer(scouter.NewMemStore(), log)
		store, err := scouter.NewPostgresStore(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("database unavailable at startup", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	srv := scouter.NewServer(store, log)

	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: srv.Routes(),

		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		os.Interrupt,
	)
	defer stop()

	go func() {
		log.Info("scouter online", "port", cfg.Port)

		if err := httpSrv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	log.Info("shutdown signal received, draining")
	srv.NotReady()

	time.Sleep(cfg.DrainDelay)

	shutCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Error("graceful shutdown failed, forcing close", "err", err)
		_ = httpSrv.Close()
	}

	log.Info("scouter offline")
}
