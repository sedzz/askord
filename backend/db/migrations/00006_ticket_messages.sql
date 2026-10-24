-- +goose Up
-- Transcript: copia de los mensajes del ticket, para conservarlos al borrar el canal.
CREATE TABLE ticket_messages (
    message_id   TEXT PRIMARY KEY,           -- Discord message ID
    ticket_id    BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id    TEXT NOT NULL,
    author_name  TEXT NOT NULL,
    content      TEXT NOT NULL,
    attachments  JSONB NOT NULL DEFAULT '[]', -- URLs de los adjuntos
    created_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX ticket_messages_ticket_idx ON ticket_messages (ticket_id, created_at);

-- +goose Down
DROP TABLE ticket_messages;
