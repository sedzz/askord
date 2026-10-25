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
	KeyEnc  []byte
	// Interruptor general del asistente en el servidor y sus pautas generales. No los toca
	// SaveAISettings (la conexión): se cambian con SaveAIBehavior.
	Enabled    bool
	Guidelines string
}

// GetAISettings devuelve la conexión del servidor, o nil si no hay.
func (s *Store) GetAISettings(ctx context.Context, guildID string) (*AISettings, error) {
	var a AISettings
	err := s.pool.QueryRow(ctx,
		`SELECT base_url, model, enabled, guidelines, api_key_enc FROM ai_settings WHERE guild_id = $1`, guildID).
		Scan(&a.BaseURL, &a.Model, &a.Enabled, &a.Guidelines, &a.KeyEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SaveAISettings guarda la conexión (URL, modelo y API key). Con KeyEnc a nil conserva la API
// key que ya hubiera. No cambia el interruptor ni las pautas de una conexión existente.
func (s *Store) SaveAISettings(ctx context.Context, guildID string, a AISettings) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_settings (guild_id, base_url, model, api_key_enc)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (guild_id) DO UPDATE
		SET base_url = EXCLUDED.base_url, model = EXCLUDED.model,
		    api_key_enc = COALESCE(EXCLUDED.api_key_enc, ai_settings.api_key_enc), updated_at = now()`,
		guildID, a.BaseURL, a.Model, a.KeyEnc)
	return err
}

// SaveAIBehavior guarda el interruptor general y las pautas. Devuelve ErrNotFound si el
// servidor aún no tiene conexión.
func (s *Store) SaveAIBehavior(ctx context.Context, guildID string, enabled bool, guidelines string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE ai_settings SET enabled = $2, guidelines = $3, updated_at = now() WHERE guild_id = $1`,
		guildID, enabled, guidelines)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAISettings borra la conexión, API key incluida.
func (s *Store) DeleteAISettings(ctx context.Context, guildID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_settings WHERE guild_id = $1`, guildID)
	return err
}
