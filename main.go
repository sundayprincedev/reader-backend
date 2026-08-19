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

	"github.com/sundayprincedev/reader-backend/internal/api"
	"github.com/sundayprincedev/reader-backend/internal/config"
	"github.com/sundayprincedev/reader-backend/internal/repository"
	"github.com/sundayprincedev/reader-backend/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config.LoadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStartup()

	client, err := storage.Connect(startupCtx, cfg.MongoURI, cfg.DatabaseName)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(shutdownCtx)
	}()

	if err := client.EnsureIndexes(startupCtx); err != nil {
		return err
	}

	router := api.NewRouter(api.Options{
		Books: api.NewHandler(
			repository.NewBookRepository(client),
			repository.NewFileRepository(client),
			cfg.MaxUploadBytes,
		),
		AccessPIN:      cfg.AccessPIN,
		AllowedOrigins: cfg.AllowedOrigins,
		Timeout:        cfg.RequestTimeout,
		StaticDir:      os.Getenv("STATIC_DIR"),
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		if cfg.AccessPIN == "" {
			slog.Warn("ACCESS_PIN is not set, every request is allowed through")
		}
		slog.Info("listening", "port", cfg.Port, "database", cfg.DatabaseName)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdown:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
