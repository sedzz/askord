-- +goose Up
ALTER TABLE tickets ADD COLUMN closed_by TEXT;   -- Discord user ID de quien lo cerró

-- +goose Down
ALTER TABLE tickets DROP COLUMN closed_by;
