-- +goose Up
-- Tipos de ticket que cada servidor define (Soporte, Pagos, Bug…).
CREATE TABLE ticket_types (
    id             BIGSERIAL PRIMARY KEY,
    guild_id       TEXT NOT NULL,
    name           TEXT NOT NULL,
    emoji          TEXT NOT NULL DEFAULT '',
    description    TEXT NOT NULL DEFAULT '',
    category_id    TEXT,                       -- NULL: usa la categoría por defecto del servidor
    staff_role_id  TEXT,                       -- NULL: usa el rol de staff por defecto
    position       INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sin dos tipos con el mismo nombre en un servidor (sin distinguir mayúsculas).
CREATE UNIQUE INDEX ticket_types_name_idx ON ticket_types (guild_id, lower(name));
CREATE INDEX ticket_types_guild_idx ON ticket_types (guild_id, position);

-- Los servidores ya configurados reciben un tipo "General", para que todo siga igual.
INSERT INTO ticket_types (guild_id, name, emoji, description)
SELECT guild_id, 'General', '📩', 'Abre un ticket y el equipo te atenderá.'
FROM ticket_settings;

-- +goose Down
DROP TABLE ticket_types;
