// Package store — все SQL-запросы приложения. Хендлеры HTTP не пишут SQL сами.
package store

import (
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("не найдено")
	ErrForbidden  = errors.New("нет прав")
	ErrConflict   = errors.New("версия устарела")
	ErrEmailTaken = errors.New("email уже занят")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidID проверяет id до запроса: иначе Postgres вернёт ошибку синтаксиса uuid вместо «не найдено».
func ValidID(id string) bool {
	return uuidRe.MatchString(id)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
