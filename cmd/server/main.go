package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"afterglow.local/tv/internal/httpapi"
	_ "time/tzdata"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	var level slog.Level
	if err = level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Error("invalid LOG_LEVEL")
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	db, err := database.Open(cfg.Dir)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	api := httpapi.New(db, cfg, logger, version)
	srv := &http.Server{Addr: cfg.Address, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	}()
	logger.Info("server listening", "address", cfg.Address, "version", version)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
