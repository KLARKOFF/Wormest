// Package db открывает пул соединений с Postgres и накатывает миграции.
package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Миграции вшиваются в бинарник, поэтому в контейнер не нужно копировать .sql-файлы.
//
//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("подключение к базе: %w", err)
	}
	// База может подниматься дольше сервера: ждём до 30 секунд
	for i := 0; i < 30; i++ {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		time.Sleep(time.Second)
	}
	pool.Close()
	return nil, fmt.Errorf("база не отвечает: %w", err)
}

func Migrate(pool *pgxpool.Pool) error {
	// goose работает через database/sql, поэтому оборачиваем пул pgx
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, "migrations")
}
