package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("no encontrado")

type Store struct {
	pool *pgxpool.Pool
}

type User struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Avatar   *string `json:"avatar"`
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) UpsertUser(ctx context.Context, u User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, username, avatar)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET username = EXCLUDED.username, avatar = EXCLUDED.avatar, updated_at = now()`,
		u.ID, u.Username, u.Avatar)
	return err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID string, expires time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expires)
	return err
}

// UserBySession devuelve el usuario de una sesión vigente y cuándo caduca.
func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (User, time.Time, error) {
	var (
		u       User
		expires time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.avatar, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, tokenHash).
		Scan(&u.ID, &u.Username, &u.Avatar, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, time.Time{}, ErrNotFound
	}
	return u, expires, err
}

func (s *Store) ExtendSession(ctx context.Context, tokenHash []byte, expires time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET expires_at = $2 WHERE token_hash = $1`, tokenHash, expires)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}
