package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrTicketExists = errors.New("el usuario ya tiene un ticket abierto")

type Ticket struct {
	ID        int64
	GuildID   string
	Number    int
	ChannelID string
	OpenerID  string
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
		INSERT INTO tickets (guild_id, number, channel_id, opener_id) VALUES ($1, $2, $3, $4)`,
		t.GuildID, t.Number, t.ChannelID, t.OpenerID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tickets_one_open_per_user" {
		return ErrTicketExists
	}
	return err
}

// OpenTicketByUser devuelve el ticket abierto del usuario, o nil si no tiene.
func (s *Store) OpenTicketByUser(ctx context.Context, guildID, userID string) (*Ticket, error) {
	var t Ticket
	err := s.pool.QueryRow(ctx, `
		SELECT id, guild_id, number, channel_id, opener_id FROM tickets
		WHERE guild_id = $1 AND opener_id = $2 AND status = 'open'`, guildID, userID).
		Scan(&t.ID, &t.GuildID, &t.Number, &t.ChannelID, &t.OpenerID)
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
		SELECT id, guild_id, number, channel_id, opener_id, status FROM tickets
		WHERE channel_id = $1`, channelID).
		Scan(&t.ID, &t.GuildID, &t.Number, &t.ChannelID, &t.OpenerID, &status)
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
	ID          string
	AuthorID    string
	AuthorName  string
	Content     string
	Attachments []string
	CreatedAt   time.Time
}

// SaveTicketMessages guarda (o actualiza, si se editaron) los mensajes del transcript.
func (s *Store) SaveTicketMessages(ctx context.Context, ticketID int64, msgs []TicketMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, m := range msgs {
		attachments := m.Attachments
		if attachments == nil {
			attachments = []string{}
		}
		batch.Queue(`
			INSERT INTO ticket_messages (message_id, ticket_id, author_id, author_name, content, attachments, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (message_id) DO UPDATE
			SET content = EXCLUDED.content, attachments = EXCLUDED.attachments`,
			m.ID, ticketID, m.AuthorID, m.AuthorName, m.Content, attachments, m.CreatedAt)
	}

	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range msgs {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
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
	if _, err := tx.Exec(ctx, `DELETE FROM ticket_settings WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
