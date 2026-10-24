-- +goose Up
ALTER TABLE ticket_settings
    ADD COLUMN panel_channel_id TEXT,
    ADD COLUMN panel_message_id TEXT,
    ADD COLUMN ticket_counter   INTEGER NOT NULL DEFAULT 0;

CREATE TABLE tickets (
    id          BIGSERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    number      INTEGER NOT NULL,           -- correlativo por servidor
    channel_id  TEXT NOT NULL UNIQUE,
    opener_id   TEXT NOT NULL,              -- Discord user ID
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at   TIMESTAMPTZ,
    UNIQUE (guild_id, number)
);

-- Un solo ticket abierto por usuario y servidor (también frena clics dobles).
CREATE UNIQUE INDEX tickets_one_open_per_user
    ON tickets (guild_id, opener_id) WHERE status = 'open';

-- +goose Down
DROP TABLE tickets;
ALTER TABLE ticket_settings
    DROP COLUMN panel_channel_id,
    DROP COLUMN panel_message_id,
    DROP COLUMN ticket_counter;
