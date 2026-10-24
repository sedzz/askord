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

// UserGuild es un servidor que el usuario puede administrar.
type UserGuild struct {
	ID    string
	Name  string
	Icon  *string
	Owner bool
}

// GuildView es un servidor del usuario y si el bot ya está dentro.
type GuildView struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Icon         *string `json:"icon"`
	Owner        bool    `json:"owner"`
	BotInstalled bool    `json:"botInstalled"`
}

func (s *Store) ReplaceUserGuilds(ctx context.Context, userID string, guilds []UserGuild) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM user_guilds WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, g := range guilds {
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_guilds (user_id, guild_id, name, icon, owner) VALUES ($1, $2, $3, $4, $5)`,
			userID, g.ID, g.Name, g.Icon, g.Owner); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListUserGuilds devuelve los servidores del usuario, con el bot instalado primero.
func (s *Store) ListUserGuilds(ctx context.Context, userID string) ([]GuildView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ug.guild_id, ug.name, ug.icon, ug.owner, g.id IS NOT NULL
		FROM user_guilds ug LEFT JOIN guilds g ON g.id = ug.guild_id
		WHERE ug.user_id = $1
		ORDER BY (g.id IS NOT NULL) DESC, ug.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []GuildView{}
	for rows.Next() {
		var g GuildView
		if err := rows.Scan(&g.ID, &g.Name, &g.Icon, &g.Owner, &g.BotInstalled); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) UpsertGuild(ctx context.Context, id, name string, icon *string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guilds (id, name, icon) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name, icon = EXCLUDED.icon, updated_at = now()`,
		id, name, icon)
	return err
}

func (s *Store) DeleteGuild(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM guilds WHERE id = $1`, id)
	return err
}

// DeleteGuildsNotIn borra los servidores que el bot ya no tiene (por ejemplo,
// si lo expulsaron mientras estaba apagado). keep debe ser no nil.
func (s *Store) DeleteGuildsNotIn(ctx context.Context, keep []string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM guilds WHERE id <> ALL($1)`, keep)
	return err
}
