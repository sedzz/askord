-- +goose Up
-- Hash del avatar del autor (el de Discord), para pintarlo en el transcript.
ALTER TABLE ticket_messages ADD COLUMN author_avatar TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE ticket_messages DROP COLUMN author_avatar;
