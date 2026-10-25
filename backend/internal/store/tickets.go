package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrTicketExists = errors.New("el usuario ya tiene un ticket abierto")

// ErrTranscriptPurged indica que el contenido del ticket ya se borró y no se puede volver a guardar.
var ErrTranscriptPurged = errors.New("el contenido del ticket ya se eliminó")

// TranscriptRetention es cuánto se conserva el contenido de un ticket desde que se cierra.
const TranscriptRetention = 90 * 24 * time.Hour

type Ticket struct {
	ID        int64
	GuildID   string
	Number    int
	ChannelID string
	OpenerID  string
	// 0 en los tickets anteriores a los tipos o si se borró su tipo.
	TypeID int64
	// Copia del nombre y el emoji del tipo al abrirse, para el historial.
	TypeName  string
	TypeEmoji string
	// Respuestas al formulario del tipo, si lo tenía.
	Answers []Answer
	// Cierre del ticket (nil si sigue abierto) y si su contenido ya se borró.
	ClosedAt        *time.Time
	ContentPurgedAt *time.Time
	// Estado del asistente de IA en este ticket (AIState*) y cuántas respuestas ha dado.
	AIState   string
	AIReplies int
}

// TranscriptExpired indica si el contenido del ticket ya no se puede guardar: se borró o
// pasó el plazo de retención (aunque la tarea de limpieza aún no haya llegado a él).
func (t Ticket) TranscriptExpired(now time.Time) bool {
	if t.ContentPurgedAt != nil {
		return true
	}
	return t.ClosedAt != nil && now.Sub(*t.ClosedAt) >= TranscriptRetention
}

// Answer es la respuesta a una pregunta del formulario, con la pregunta tal como era.
type Answer struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func (s *Store) SavePanel(ctx context.Context, guildID, channelID, messageID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE ticket_settings
		SET panel_channel_id = $2, panel_message_id = $3, updated_at = now()
		WHERE guild_id = $1`, guildID, channelID, messageID)
	return err
}

// NextTicketNumber reserva el siguiente correlativo del servidor.
func (s *Store) NextTicketNumber(ctx context.Context, guildID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		UPDATE ticket_settings SET ticket_counter = ticket_counter + 1
		WHERE guild_id = $1 RETURNING ticket_counter`, guildID).Scan(&n)
	return n, err
}

// CreateTicket guarda el ticket; devuelve ErrTicketExists si el usuario ya tiene uno abierto.
func (s *Store) CreateTicket(ctx context.Context, t Ticket) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tickets (guild_id, number, channel_id, opener_id, type_id, type_name, type_emoji, answers)
		VALUES ($1, $2, $3, $4, NULLIF($5::bigint, 0), $6, $7, $8)`,
		t.GuildID, t.Number, t.ChannelID, t.OpenerID, t.TypeID, t.TypeName, t.TypeEmoji, answersOrEmpty(t.Answers))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tickets_one_open_per_user_type" {
		return ErrTicketExists
	}
	return err
}

// OpenTicketByUser devuelve el ticket abierto del usuario de ese tipo, o nil si no tiene.
func (s *Store) OpenTicketByUser(ctx context.Context, guildID, userID string, typeID int64) (*Ticket, error) {
	var t Ticket
	err := s.pool.QueryRow(ctx, `
		SELECT id, guild_id, number, channel_id, opener_id, COALESCE(type_id, 0) FROM tickets
		WHERE guild_id = $1 AND opener_id = $2 AND status = 'open' AND COALESCE(type_id, 0) = $3`,
		guildID, userID, typeID).
		Scan(&t.ID, &t.GuildID, &t.Number, &t.ChannelID, &t.OpenerID, &t.TypeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CloseTicketByChannel marca como cerrado el ticket de ese canal (si está abierto).
func (s *Store) CloseTicketByChannel(ctx context.Context, channelID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tickets SET status = 'closed', closed_at = now()
		WHERE channel_id = $1 AND status = 'open'`, channelID)
	return err
}

// TicketByChannel devuelve el ticket de un canal y su estado, o nil si no es un ticket.
func (s *Store) TicketByChannel(ctx context.Context, channelID string) (*Ticket, string, error) {
	var (
		t      Ticket
		status string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, guild_id, number, channel_id, opener_id, COALESCE(type_id, 0), status, closed_at, content_purged_at,
		       answers, ai_state, ai_replies
		FROM tickets WHERE channel_id = $1`, channelID).
		Scan(&t.ID, &t.GuildID, &t.Number, &t.ChannelID, &t.OpenerID, &t.TypeID, &status, &t.ClosedAt, &t.ContentPurgedAt,
			&t.Answers, &t.AIState, &t.AIReplies)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return &t, status, nil
}

// CloseTicket cierra el ticket de forma atómica; false si ya estaba cerrado.
func (s *Store) CloseTicket(ctx context.Context, channelID, closedBy string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tickets SET status = 'closed', closed_at = now(), closed_by = $2
		WHERE channel_id = $1 AND status = 'open'`, channelID, closedBy)
	return tag.RowsAffected() == 1, err
}

// TicketMessage es un mensaje del transcript de un ticket.
type TicketMessage struct {
	ID           string
	AuthorID     string
	AuthorName   string
	AuthorAvatar string // hash del avatar en Discord; vacío si usa el de por defecto
	Content      string
	Attachments  []string
	CreatedAt    time.Time
}

// SaveTicketMessages guarda (o actualiza, si se editaron) los mensajes del transcript.
// Devuelve ErrTranscriptPurged si el contenido del ticket ya se borró.
func (s *Store) SaveTicketMessages(ctx context.Context, ticketID int64, msgs []TicketMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// FOR SHARE bloquea la fila frente a una purga simultánea hasta confirmar.
	var purged bool
	err = tx.QueryRow(ctx, `SELECT content_purged_at IS NOT NULL FROM tickets WHERE id = $1 FOR SHARE`, ticketID).Scan(&purged)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if purged {
		return ErrTranscriptPurged
	}

	batch := &pgx.Batch{}
	for _, m := range msgs {
		attachments := m.Attachments
		if attachments == nil {
			attachments = []string{}
		}
		batch.Queue(`
			INSERT INTO ticket_messages (message_id, ticket_id, author_id, author_name, author_avatar, content, attachments, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (message_id) DO UPDATE
			SET content = EXCLUDED.content, attachments = EXCLUDED.attachments,
			    author_name = EXCLUDED.author_name, author_avatar = EXCLUDED.author_avatar`,
			m.ID, ticketID, m.AuthorID, m.AuthorName, m.AuthorAvatar, m.Content, attachments, m.CreatedAt)
	}

	results := tx.SendBatch(ctx, batch)
	for range msgs {
		if _, err := results.Exec(); err != nil {
			results.Close() //nolint:errcheck
			return err
		}
	}
	if err := results.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteGuildData borra todo lo de un servidor: configuración, tickets y, en cascada,
// sus transcripciones. No toca la fila de guilds ni la lista de servidores de los
// usuarios: así, si algo falla después, la operación se puede repetir.
func (s *Store) DeleteGuildData(ctx context.Context, guildID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// ticket_messages cae en cascada con tickets.
	if _, err := tx.Exec(ctx, `DELETE FROM tickets WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ticket_types WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ticket_settings WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ai_settings WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// answersOrEmpty evita guardar null: sin respuestas, una lista vacía.
func answersOrEmpty(a []Answer) []Answer {
	if a == nil {
		return []Answer{}
	}
	return a
}
