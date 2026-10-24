-- +goose Up
-- Servidores donde el bot está instalado (los mantiene el bot).
CREATE TABLE guilds (
    id          TEXT PRIMARY KEY,           -- Discord guild ID
    name        TEXT NOT NULL,
    icon        TEXT,
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Servidores que el usuario puede administrar (se refresca en cada login).
CREATE TABLE user_guilds (
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id  TEXT NOT NULL,
    name      TEXT NOT NULL,
    icon      TEXT,
    owner     BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, guild_id)
);

-- +goose Down
DROP TABLE user_guilds;
DROP TABLE guilds;
