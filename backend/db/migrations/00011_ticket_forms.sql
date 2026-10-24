-- +goose Up
-- Preguntas que se hacen al abrir un ticket de cada tipo (hasta 5, las de un formulario de
-- Discord) y las respuestas que dio quien lo abrió. Cada pregunta: {label, required, long}.
ALTER TABLE ticket_types ADD COLUMN questions JSONB NOT NULL DEFAULT '[]';

-- Respuestas con el texto de la pregunta tal como era: {question, answer}.
ALTER TABLE tickets ADD COLUMN answers JSONB NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE tickets DROP COLUMN answers;
ALTER TABLE ticket_types DROP COLUMN questions;
