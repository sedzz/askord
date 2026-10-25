package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// AISettings es la conexión de un servidor con su LLM. KeyEnc es la API key cifrada (nil si
// aún no hay): el store no sabe cifrar ni descifrar, solo la guarda.
type AISettings struct {
	BaseURL string
	Model   string
	Enabled bool
	KeyEnc  []byte
}

// GetAISettings devuelve la conexión del servidor, o nil si no hay.
func (s *Store) GetAISettings(ctx context.Context, guildID string) (*AISettings, error) {
	var a AISettings
	err := s.pool.QueryRow(ctx,
		`SELECT base_url, model, enabled, api_key_enc FROM ai_settings WHERE guild_id = $1`, guildID).
		Scan(&a.BaseURL, &a.Model, &a.Enabled, &a.KeyEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SaveAISettings guarda la conexión. Con KeyEnc a nil conserva la API key que ya hubiera.
func (s *Store) SaveAISettings(ctx context.Context, guildID string, a AISettings) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_settings (guild_id, base_url, model, enabled, api_key_enc)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id) DO UPDATE
		SET base_url = EXCLUDED.base_url, model = EXCLUDED.model, enabled = EXCLUDED.enabled,
		    api_key_enc = COALESCE(EXCLUDED.api_key_enc, ai_settings.api_key_enc), updated_at = now()`,
		guildID, a.BaseURL, a.Model, a.Enabled, a.KeyEnc)
	return err
}

// DeleteAISettings borra la conexión, API key incluida.
func (s *Store) DeleteAISettings(ctx context.Context, guildID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_settings WHERE guild_id = $1`, guildID)
	return err
}
