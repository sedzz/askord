package store

import (
	"context"
	"time"
)

// Borra mensajes y respuestas de los tickets elegidos y los marca como purgados. El registro
// del ticket se conserva para que las estadísticas no cambien.
const purgeContent = `
	gone AS (DELETE FROM ticket_messages WHERE ticket_id IN (SELECT id FROM expired))
	UPDATE tickets
	SET answers = '[]'::jsonb, content_purged_at = COALESCE(content_purged_at, now())
	WHERE id IN (SELECT id FROM expired)`

// PurgeExpiredTranscripts borra el contenido de hasta limit tickets cerrados antes de
// olderThan y devuelve cuántos limpió. Es repetible, y FOR UPDATE SKIP LOCKED evita que dos
// copias de la API trabajen sobre los mismos tickets.
func (s *Store) PurgeExpiredTranscripts(ctx context.Context, olderThan time.Time, limit int) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		WITH expired AS (
			SELECT id FROM tickets
			WHERE status = 'closed' AND content_purged_at IS NULL AND closed_at < $1
			ORDER BY closed_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		),`+purgeContent, olderThan, limit)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// DeleteTranscript borra a mano el contenido de un ticket cerrado del servidor. Devuelve
// ErrNotFound si no existe, es de otro servidor o sigue abierto. Repetirlo no falla.
func (s *Store) DeleteTranscript(ctx context.Context, guildID string, ticketID int64) error {
	tag, err := s.pool.Exec(ctx, `
		WITH expired AS (
			SELECT id FROM tickets
			WHERE id = $1 AND guild_id = $2 AND status = 'closed'
			FOR UPDATE
		),`+purgeContent, ticketID, guildID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
