-- +goose Up
-- Nombre y emoji del tipo tal como eran al abrir el ticket. Así el historial (transcripciones,
-- exportaciones) sigue diciendo "Pagos" aunque ese tipo se borre después.
ALTER TABLE tickets
    ADD COLUMN type_name  TEXT NOT NULL DEFAULT '',
    ADD COLUMN type_emoji TEXT NOT NULL DEFAULT '';

UPDATE tickets t
SET type_name = ty.name, type_emoji = ty.emoji
FROM ticket_types ty
WHERE ty.id = t.type_id AND ty.guild_id = t.guild_id;

-- +goose Down
ALTER TABLE tickets DROP COLUMN type_name, DROP COLUMN type_emoji;
