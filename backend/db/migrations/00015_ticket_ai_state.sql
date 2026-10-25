-- +goose Up
-- Estado del asistente de IA en cada ticket: 'active' responde, 'escalated' pasó a una persona
-- (por su cuenta, por el botón o por un fallo del proveedor) y 'paused' se calló porque escribió
-- el staff. ai_replies cuenta sus respuestas, para frenar conversaciones largas.
ALTER TABLE tickets
    ADD COLUMN ai_state   TEXT NOT NULL DEFAULT 'active' CHECK (ai_state IN ('active', 'escalated', 'paused')),
    ADD COLUMN ai_replies INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE tickets DROP COLUMN ai_replies, DROP COLUMN ai_state;
