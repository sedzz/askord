-- +goose Up
-- Conexión del servidor con un LLM compatible con la API de OpenAI. La API key va cifrada
-- (AES-GCM, ver internal/ai); nunca se guarda ni se devuelve en claro.
CREATE TABLE ai_settings (
    guild_id    TEXT PRIMARY KEY,
    base_url    TEXT NOT NULL,
    model       TEXT NOT NULL,
    api_key_enc BYTEA,
    enabled     BOOLEAN NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE ai_settings;
