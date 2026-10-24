package bot

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

// Tope de mensajes por ticket, para no quedarnos colgados con canales enormes.
const maxTranscriptMessages = 5000

// saveTranscript copia los mensajes del canal a la base de datos y devuelve cuántos
// guardó. Es idempotente: se puede repetir y solo actualiza lo editado o nuevo.
func (b *Bot) saveTranscript(ctx context.Context, ticket *store.Ticket) (int, error) {
	messages, err := b.fetchMessages(ticket.ChannelID)
	if err != nil {
		return 0, fmt.Errorf("leer mensajes: %w", err)
	}

	saved := make([]store.TicketMessage, 0, len(messages))
	humanWithoutContent := 0
	humans := 0
	for _, m := range messages {
		// Solo mensajes normales y respuestas; se ignoran avisos del sistema.
		if m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply {
			continue
		}

		attachments := make([]string, 0, len(m.Attachments))
		for _, a := range m.Attachments {
			attachments = append(attachments, a.URL)
		}
		saved = append(saved, store.TicketMessage{
			ID:          m.ID,
			AuthorID:    m.Author.ID,
			AuthorName:  cmp.Or(m.Author.GlobalName, m.Author.Username),
			Content:     m.Content,
			Attachments: attachments,
			CreatedAt:   m.Timestamp,
		})

		if !m.Author.Bot {
			humans++
			if m.Content == "" && len(m.Attachments) == 0 {
				humanWithoutContent++
			}
		}
	}

	// Sin el "Message Content Intent" activado en el portal, Discord devuelve el
	// texto vacío: se avisa para no guardar transcripts en blanco sin enterarnos.
	if humans > 0 && humans == humanWithoutContent {
		slog.Warn("transcript sin contenido: activa Message Content Intent en el Developer Portal",
			"channel", ticket.ChannelID)
	}

	// Leer un historial largo puede agotar el plazo del llamador: la escritura lleva el suyo.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := b.store.SaveTicketMessages(ctx, ticket.ID, saved); err != nil {
		return 0, fmt.Errorf("guardar mensajes: %w", err)
	}
	slog.Info("transcript guardado", "guild", ticket.GuildID, "number", ticket.Number, "messages", len(saved))
	return len(saved), nil
}

// fetchMessages lee el historial del canal, paginando de 100 en 100 (del más nuevo al más viejo).
func (b *Bot) fetchMessages(channelID string) ([]*discordgo.Message, error) {
	var all []*discordgo.Message
	before := ""
	for len(all) < maxTranscriptMessages {
		page, err := b.session.ChannelMessages(channelID, 100, before, "", "")
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < 100 {
			break
		}
		before = page[len(page)-1].ID
	}
	return all, nil
}
