-- +goose Up
-- Cuándo se borró el contenido del ticket (mensajes y respuestas del formulario), a mano o
-- por caducidad. Impide volver a transcribir un ticket cuyo canal sigue existiendo.
ALTER TABLE tickets ADD COLUMN content_purged_at TIMESTAMPTZ;

-- Tickets cerrados que aún conservan contenido: lo que recorre la tarea de retención.
CREATE INDEX tickets_retention_idx ON tickets (closed_at)
    WHERE status = 'closed' AND content_purged_at IS NULL;

-- +goose Down
DROP INDEX tickets_retention_idx;
ALTER TABLE tickets DROP COLUMN content_purged_at;
