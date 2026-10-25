-- +goose Up
-- Pautas generales del asistente para todo el servidor (markdown o texto plano).
ALTER TABLE ai_settings ADD COLUMN guidelines TEXT NOT NULL DEFAULT '';

-- Por tipo de ticket: si lo atiende el asistente, cómo, y cuándo pasarlo a una persona.
ALTER TABLE ticket_types
    ADD COLUMN ai_enabled      BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_instructions TEXT NOT NULL DEFAULT '',
    ADD COLUMN ai_escalation   TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE ticket_types
    DROP COLUMN ai_escalation,
    DROP COLUMN ai_instructions,
    DROP COLUMN ai_enabled;
ALTER TABLE ai_settings DROP COLUMN guidelines;
