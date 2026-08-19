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

	"github.com/sundayprincedev/mereader/internal/api"
	"github.com/sundayprincedev/mereader/internal/config"
	"github.com/sundayprincedev/mereader/internal/repository"
	"github.com/sundayprincedev/mereader/internal/storage"
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

	handler := api.NewHandler(repository.NewBookRepository(client))
	router := api.NewRouter(handler, cfg.AllowedOrigins, cfg.RequestTimeout, os.Getenv("STATIC_DIR"))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
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
