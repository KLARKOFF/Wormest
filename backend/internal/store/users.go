package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *Store) CreateUser(ctx context.Context, email, displayName, passwordHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name, created_at`,
		email, displayName, passwordHash,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return u, err
}

// UserByEmail возвращает пользователя вместе с хешем пароля — нужен только при входе.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, created_at, password_hash
		FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	return u, hash, err
}
