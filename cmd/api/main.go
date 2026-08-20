package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"woason-api/internal/auth"
	"woason-api/internal/config"
	"woason-api/internal/db"
	api "woason-api/internal/http"
	"woason-api/internal/payment"
	"woason-api/internal/seed"
	"woason-api/internal/store"
	"woason-api/internal/ws"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("конфиг", "err", err.Error())
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.MigrateUp(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		slog.Error("миграции", "err", err.Error())
		os.Exit(1)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	st := store.New(pool)
	if err := seed.Run(ctx, st, cfg); err != nil {
		slog.Error("сиды", "err", err.Error())
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		slog.Error("uploads", "err", err.Error())
		os.Exit(1)
	}

	hub := ws.NewHub()
	handler := api.NewRouter(api.Deps{
		Config: cfg,
		Store:  st,
		Tokens: auth.NewTokens(cfg.JWTSecret),
		Hub:    hub,
		Pay:    payment.New(cfg),
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("WOAson API", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http", "err", err.Error())
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
