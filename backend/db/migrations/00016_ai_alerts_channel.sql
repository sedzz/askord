-- +goose Up
-- Canal privado del staff donde el asistente avisa de los tickets que pasa a una persona, con el
-- motivo. Así el usuario solo ve un mensaje neutro en su ticket. Vacío: sin canal.
ALTER TABLE ai_settings ADD COLUMN alerts_channel_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE ai_settings DROP COLUMN alerts_channel_id;
