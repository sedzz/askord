package store

import (
	"context"
	"errors"

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
