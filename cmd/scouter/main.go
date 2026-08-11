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

	srv := scouter.NewServer(scouter.NewMemStore(), log)

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
