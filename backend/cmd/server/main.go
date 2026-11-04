// Точка входа сервера: конфиг → база → миграции → HTTP.
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

	"wormest/internal/api"
	"wormest/internal/config"
	"wormest/internal/db"
	"wormest/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("сервер остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	// Ctrl+C или docker stop → аккуратное завершение
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.StorageDir, 0o750); err != nil {
		return err
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		return err
	}

	st := store.New(pool)
	go cleanupSessions(ctx, st, log)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(cfg, st, log).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("сервер запущен", "addr", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("останавливаюсь…")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func cleanupSessions(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := st.DeleteExpiredSessions(ctx); err != nil {
				log.Warn("не удалось почистить сессии", "err", err)
			} else if n > 0 {
				log.Info("удалены истёкшие сессии", "count", n)
			}
		}
	}
}
