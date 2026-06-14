package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nine/internal/app/httpapi"
	"nine/internal/app/store"
)

const drainTimeout = 10 * time.Second

func Run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	st := store.New()
	startedAt := time.Now().UTC()
	server := &http.Server{
		Addr:              listenAddr(),
		Handler:           httpapi.NewHandler(st, startedAt),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited unexpectedly", "error", err)
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err, "timeout", drainTimeout.String())
		_ = server.Close()
		return err
	}

	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited unexpectedly", "error", err)
		return err
	}
	logger.Info("server stopped")
	return nil
}

func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}
