-- +goose Up
-- Sin FK a guilds: si el bot sale y vuelve a entrar, la configuración se conserva.
CREATE TABLE ticket_settings (
    guild_id       TEXT PRIMARY KEY,
    category_id    TEXT NOT NULL,           -- categoría donde se crean los tickets
    staff_role_id  TEXT NOT NULL,           -- rol que puede ver los tickets
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE ticket_settings;
