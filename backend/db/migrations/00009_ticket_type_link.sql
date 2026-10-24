-- +goose Up
-- Qué tipo de ticket es cada ticket. Sin clave foránea a propósito: si se borra el tipo, el
-- ticket conserva el número y no choca con la regla de un ticket abierto por tipo.
ALTER TABLE tickets ADD COLUMN type_id BIGINT;

-- Un ticket abierto por usuario *y tipo* (antes, uno por usuario). Los tickets sin tipo
-- (anteriores a esto) cuentan como un tipo más.
DROP INDEX tickets_one_open_per_user;
CREATE UNIQUE INDEX tickets_one_open_per_user_type
    ON tickets (guild_id, opener_id, COALESCE(type_id, 0)) WHERE status = 'open';

-- +goose Down
DROP INDEX tickets_one_open_per_user_type;
CREATE UNIQUE INDEX tickets_one_open_per_user
    ON tickets (guild_id, opener_id) WHERE status = 'open';
ALTER TABLE tickets DROP COLUMN type_id;
