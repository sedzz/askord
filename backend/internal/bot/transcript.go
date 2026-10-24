package bot

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

// Tope de mensajes por ticket, para no quedarnos colgados con canales enormes.
const maxTranscriptMessages = 5000

// saveTranscript copia los mensajes del canal a la base de datos y devuelve cuántos
// guardó. contentHidden es true cuando Discord ocultó el texto de todos los mensajes de
// personas (falta el Message Content Intent). Es idempotente: se puede repetir y solo
// actualiza lo editado o nuevo.
func (b *Bot) saveTranscript(ctx context.Context, ticket *store.Ticket) (saved int, contentHidden bool, err error) {
	messages, err := b.fetchMessages(ticket.ChannelID)
	if err != nil {
		return 0, false, fmt.Errorf("leer mensajes: %w", err)
	}
	roles := b.roleNames(ticket.GuildID)

	rows := make([]store.TicketMessage, 0, len(messages))
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
		rows = append(rows, store.TicketMessage{
			ID:           m.ID,
			AuthorID:     m.Author.ID,
			AuthorName:   cmp.Or(m.Author.GlobalName, m.Author.Username),
			AuthorAvatar: m.Author.Avatar,
			Content:      readableContent(m, roles),
			Attachments:  attachments,
			CreatedAt:    m.Timestamp,
		})

		if !m.Author.Bot {
			humans++
			if m.Content == "" && len(m.Attachments) == 0 {
				humanWithoutContent++
			}
		}
	}

	// Sin el "Message Content Intent" activado en el portal, Discord devuelve el texto
	// (y los adjuntos) vacío: se avisa para no guardar transcripts en blanco sin enterarnos.
	contentHidden = humans > 0 && humans == humanWithoutContent
	if contentHidden {
		slog.Warn("transcript sin contenido: activa Message Content Intent en el Developer Portal",
			"channel", ticket.ChannelID)
	}

	// Leer un historial largo puede agotar el plazo del llamador: la escritura lleva el suyo.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := b.store.SaveTicketMessages(ctx, ticket.ID, rows); err != nil {
		return 0, false, fmt.Errorf("guardar mensajes: %w", err)
	}
	slog.Info("transcript guardado", "guild", ticket.GuildID, "number", ticket.Number, "messages", len(rows))
	return len(rows), contentHidden, nil
}

// roleNames devuelve el nombre de cada rol del servidor, para pintar las menciones.
func (b *Bot) roleNames(guildID string) map[string]string {
	names := map[string]string{}
	roles, err := b.guildRoles(guildID)
	if err != nil {
		slog.Warn("leer roles para el transcript", "guild", guildID, "err", err)
		return names
	}
	for _, r := range roles {
		names[r.ID] = r.Name
	}
	return names
}

// readableContent sustituye las menciones (<@id>, <@&id>) por @nombre, que es lo que
// se vería en Discord; guardadas tal cual serían un número ilegible.
func readableContent(m *discordgo.Message, roles map[string]string) string {
	if m.Content == "" {
		return m.Content
	}
	pairs := make([]string, 0, 2*(len(m.Mentions)+len(m.MentionRoles)))
	for _, u := range m.Mentions {
		name := "@" + cmp.Or(u.GlobalName, u.Username)
		pairs = append(pairs, "<@"+u.ID+">", name, "<@!"+u.ID+">", name)
	}
	for _, id := range m.MentionRoles {
		if name, ok := roles[id]; ok {
			pairs = append(pairs, "<@&"+id+">", "@"+name)
		}
	}
	if len(pairs) == 0 {
		return m.Content
	}
	return strings.NewReplacer(pairs...).Replace(m.Content)
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
