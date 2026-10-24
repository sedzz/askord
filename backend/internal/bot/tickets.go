package bot

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

const (
	customIDCloseTicket      = "ticket:close"
	customIDTranscribeTicket = "ticket:transcribe"
	customIDDeleteTicket     = "ticket:delete"
)

const (
	// Lo que ve y puede hacer quien participa en un ticket.
	ticketMemberPerms int64 = discordgo.PermissionViewChannel |
		discordgo.PermissionSendMessages |
		discordgo.PermissionReadMessageHistory |
		discordgo.PermissionEmbedLinks |
		discordgo.PermissionAttachFiles
	ticketBotPerms = ticketMemberPerms | discordgo.PermissionManageChannels
)

const (
	msgNotConfigured = "Los tickets aún no están configurados en este servidor."
	msgNoPerms       = "No tengo permisos para crear el ticket. Avisa a un administrador."
	msgInternal      = "No se pudo crear el ticket. Inténtalo de nuevo en un momento."
)

// TextChannels devuelve los canales de texto del servidor, en su orden.
func (b *Bot) TextChannels(guildID string) ([]Option, error) {
	channels, err := b.guildChannels(guildID)
	if err != nil {
		return nil, err
	}
	texts := []*discordgo.Channel{}
	for _, c := range channels {
		if c.Type == discordgo.ChannelTypeGuildText {
			texts = append(texts, c)
		}
	}
	sort.Slice(texts, func(i, j int) bool { return texts[i].Position < texts[j].Position })

	out := make([]Option, 0, len(texts))
	for _, c := range texts {
		out = append(out, Option{ID: c.ID, Name: c.Name})
	}
	return out, nil
}

func (b *Bot) onInteraction(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent || i.GuildID == "" || i.Member == nil {
		return
	}
	data := i.MessageComponentData()
	switch {
	case data.CustomID == customIDOpenTicket || strings.HasPrefix(data.CustomID, customIDOpenTicket+":"):
		b.openTicket(i, parseTypeID(data.CustomID))
	case data.CustomID == customIDSelectType:
		if len(data.Values) == 1 {
			id, _ := strconv.ParseInt(data.Values[0], 10, 64)
			b.openTicket(i, id)
		}
	case data.CustomID == customIDCloseTicket:
		b.closeTicket(i)
	case data.CustomID == customIDTranscribeTicket:
		b.transcribeTicket(i)
	case data.CustomID == customIDDeleteTicket:
		b.deleteTicket(i)
	}
}

// openTicket responde en privado al usuario que pulsó el botón.
// typeID es el tipo elegido; 0 en los paneles antiguos, que abren el primer tipo.
func (b *Bot) openTicket(i *discordgo.InteractionCreate, typeID int64) {
	// Crear el canal puede tardar más que los 3 s que da Discord: se difiere la respuesta.
	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		slog.Error("responder interaccion", "err", err)
		return
	}

	msg := b.createTicket(i.GuildID, i.Member.User, typeID)
	if _, err := b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg}); err != nil {
		slog.Error("editar respuesta", "err", err)
	}
}

// createTicket crea el canal privado y devuelve el mensaje para el usuario.
func (b *Bot) createTicket(guildID string, user *discordgo.User, typeID int64) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	settings, err := b.store.GetTicketSettings(ctx, guildID)
	if err != nil {
		slog.Error("leer configuracion de tickets", "guild", guildID, "err", err)
		return msgInternal
	}
	if settings == nil {
		return msgNotConfigured
	}

	types, err := b.store.ListTicketTypes(ctx, guildID)
	if err != nil {
		slog.Error("listar tipos de ticket", "guild", guildID, "err", err)
		return msgInternal
	}
	ticketType, ok := pickType(types, typeID)
	if !ok {
		return "Este tipo de ticket ya no existe. Avisa al staff para que actualice el panel."
	}
	// Cada tipo lleva su categoría y su equipo; si no los tiene, los del servidor.
	categoryID := cmp.Or(ticketType.CategoryID, settings.CategoryID)
	staffRoleID := cmp.Or(ticketType.StaffRoleID, settings.StaffRoleID)

	existing, err := b.store.OpenTicketByUser(ctx, guildID, user.ID, ticketType.ID)
	if err != nil {
		slog.Error("buscar ticket abierto", "guild", guildID, "err", err)
		return msgInternal
	}
	if existing != nil {
		if b.channelExists(existing.ChannelID) {
			return fmt.Sprintf("Ya tienes un ticket abierto%s: <#%s>", ofType(ticketType, len(types)), existing.ChannelID)
		}
		// Borraron el canal a mano: se cierra el ticket y se deja abrir otro.
		if err := b.store.CloseTicketByChannel(ctx, existing.ChannelID); err != nil {
			slog.Error("cerrar ticket huerfano", "err", err)
			return msgInternal
		}
	}

	number, err := b.store.NextTicketNumber(ctx, guildID)
	if err != nil {
		slog.Error("reservar numero de ticket", "guild", guildID, "err", err)
		return msgInternal
	}

	channel, err := b.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:     fmt.Sprintf("ticket-%04d", number),
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: categoryID,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			// @everyone usa como ID el del propio servidor.
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
			{ID: user.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketMemberPerms},
			{ID: staffRoleID, Type: discordgo.PermissionOverwriteTypeRole, Allow: ticketMemberPerms},
			{ID: b.session.State.User.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketBotPerms},
		},
	})
	if err != nil {
		if errors.Is(wrapPermissionError(err), ErrMissingPermissions) {
			return msgNoPerms
		}
		slog.Error("crear canal de ticket", "guild", guildID, "err", err)
		return msgInternal
	}

	err = b.store.CreateTicket(ctx, store.Ticket{
		GuildID: guildID, Number: number, ChannelID: channel.ID, OpenerID: user.ID, TypeID: ticketType.ID,
		TypeName: ticketType.Name, TypeEmoji: ticketType.Emoji,
	})
	if err != nil {
		_, _ = b.session.ChannelDelete(channel.ID)
		if errors.Is(err, store.ErrTicketExists) {
			return "Ya tienes un ticket abierto."
		}
		slog.Error("guardar ticket", "guild", guildID, "err", err)
		return msgInternal
	}

	_, err = b.session.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Content: fmt.Sprintf(
			"Hola <@%s>, gracias por abrir un ticket%s. Cuéntanos tu problema y <@&%s> te atenderá en cuanto pueda.",
			user.ID, ofType(ticketType, len(types)), staffRoleID),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: []string{user.ID},
			Roles: []string{staffRoleID},
		},
		Components: []discordgo.MessageComponent{buttonRow(discordgo.Button{
			Label:    "Cerrar ticket",
			Style:    discordgo.SecondaryButton,
			CustomID: customIDCloseTicket,
			Emoji:    &discordgo.ComponentEmoji{Name: "🔒"},
		})},
	})
	if err != nil {
		slog.Error("enviar bienvenida", "channel", channel.ID, "err", err)
	}

	slog.Info("ticket creado", "guild", guildID, "number", number, "user", user.ID)
	return fmt.Sprintf("Tu ticket está listo: <#%s>", channel.ID)
}

// channelExists es false solo si Discord confirma que el canal no existe.
func (b *Bot) channelExists(channelID string) bool {
	_, err := b.session.Channel(channelID)
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
		return false
	}
	return true
}

// Si borran el canal de un ticket a mano, el ticket se marca como cerrado.
func (b *Bot) onChannelDelete(_ *discordgo.Session, e *discordgo.ChannelDelete) {
	ctx, cancel := timeout()
	defer cancel()
	if err := b.store.CloseTicketByChannel(ctx, e.ID); err != nil {
		slog.Error("cerrar ticket de canal borrado", "channel", e.ID, "err", err)
	}
}

// CreateTextChannel crea un canal de texto visible para todos pero de solo lectura,
// pensado para el panel de tickets.
func (b *Bot) CreateTextChannel(guildID, name string) (Option, error) {
	const readOnly = discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory
	c, err := b.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildText,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Allow: readOnly, Deny: discordgo.PermissionSendMessages},
			{ID: b.session.State.User.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketBotPerms},
		},
	})
	if err != nil {
		return Option{}, wrapPermissionError(err)
	}
	b.rememberChannel(c)
	return Option{ID: c.ID, Name: c.Name}, nil
}

// pickType elige el tipo pedido; con 0 (paneles antiguos) el primero. Sin ningún tipo
// devuelve uno vacío, que usa los valores del servidor.
func pickType(types []store.TicketType, id int64) (store.TicketType, bool) {
	if id == 0 {
		if len(types) == 0 {
			return store.TicketType{}, true
		}
		return types[0], true
	}
	for _, t := range types {
		if t.ID == id {
			return t, true
		}
	}
	return store.TicketType{}, false
}

// ofType nombra el tipo en un mensaje, solo si el servidor tiene varios: " de tipo **💳 Pagos**".
func ofType(t store.TicketType, count int) string {
	if count < 2 || t.Name == "" {
		return ""
	}
	return fmt.Sprintf(" de tipo **%s**", strings.TrimSpace(t.Emoji+" "+t.Name))
}

// staffRoleFor es el rol que atiende un ticket: el de su tipo o, si no tiene, el del servidor.
func (b *Bot) staffRoleFor(ctx context.Context, t *store.Ticket, settings *store.TicketSettings) string {
	if t.TypeID != 0 {
		if ty, err := b.store.GetTicketType(ctx, t.GuildID, t.TypeID); err == nil && ty.StaffRoleID != "" {
			return ty.StaffRoleID
		}
	}
	return settings.StaffRoleID
}

func buttonRow(buttons ...discordgo.Button) discordgo.MessageComponent {
	row := discordgo.ActionsRow{}
	for _, b := range buttons {
		row.Components = append(row.Components, b)
	}
	return row
}

// canManageTicket: quien abrió el ticket, el rol de staff o un administrador.
func canManageTicket(m *discordgo.Member, t *store.Ticket, staffRoleID string) bool {
	return m.User.ID == t.OpenerID || isStaff(m, staffRoleID)
}

// isStaff: miembros del rol de staff o administradores.
func isStaff(m *discordgo.Member, staffRoleID string) bool {
	return slices.Contains(m.Roles, staffRoleID) || m.Permissions&discordgo.PermissionAdministrator != 0
}

// replyEphemeral responde solo al usuario que pulsó el botón.
func (b *Bot) replyEphemeral(i *discordgo.InteractionCreate, msg string) {
	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: msg, Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		slog.Error("responder interaccion", "err", err)
	}
}

// closeTicket cierra el ticket: el usuario pierde el acceso, el canal se renombra
// y el staff conserva el historial hasta eliminarlo con el botón.
func (b *Bot) closeTicket(i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ticket, status, err := b.store.TicketByChannel(ctx, i.ChannelID)
	if err != nil {
		slog.Error("buscar ticket", "channel", i.ChannelID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if ticket == nil || status != "open" {
		b.replyEphemeral(i, "Este ticket ya está cerrado.")
		return
	}
	settings, err := b.store.GetTicketSettings(ctx, ticket.GuildID)
	if err != nil || settings == nil {
		slog.Error("leer configuracion de tickets", "guild", ticket.GuildID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if !canManageTicket(i.Member, ticket, b.staffRoleFor(ctx, ticket, settings)) {
		b.replyEphemeral(i, "No puedes cerrar este ticket.")
		return
	}

	// Se difiere para poder editar el mensaje del botón cuando todo esté hecho.
	if err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		slog.Error("responder interaccion", "err", err)
		return
	}

	closed, err := b.store.CloseTicket(ctx, ticket.ChannelID, i.Member.User.ID)
	if err != nil {
		slog.Error("cerrar ticket", "channel", ticket.ChannelID, "err", err)
		return
	}
	if !closed { // otra pulsación se adelantó
		return
	}

	if err := b.session.ChannelPermissionDelete(ticket.ChannelID, ticket.OpenerID); err != nil {
		slog.Error("quitar acceso al usuario", "channel", ticket.ChannelID, "err", err)
	}
	_, err = b.session.ChannelEdit(ticket.ChannelID, &discordgo.ChannelEdit{
		Name: fmt.Sprintf("cerrado-%04d", ticket.Number),
	})
	if err != nil {
		slog.Error("renombrar canal", "channel", ticket.ChannelID, "err", err)
	}

	content := fmt.Sprintf("🔒 Ticket cerrado por <@%s>. El staff puede transcribir la conversación o eliminar el canal.", i.Member.User.ID)
	_, err = b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
		Components: &[]discordgo.MessageComponent{buttonRow(
			discordgo.Button{
				Label:    "Transcribir",
				Style:    discordgo.SecondaryButton,
				CustomID: customIDTranscribeTicket,
				Emoji:    &discordgo.ComponentEmoji{Name: "📝"},
			},
			discordgo.Button{
				Label:    "Eliminar ticket",
				Style:    discordgo.DangerButton,
				CustomID: customIDDeleteTicket,
				Emoji:    &discordgo.ComponentEmoji{Name: "🗑️"},
			},
		)},
	})
	if err != nil {
		slog.Error("editar mensaje del ticket", "channel", ticket.ChannelID, "err", err)
	}
	slog.Info("ticket cerrado", "guild", ticket.GuildID, "number", ticket.Number, "by", i.Member.User.ID)
}

// deleteTicket elimina el canal de un ticket ya cerrado. Solo el staff.
func (b *Bot) deleteTicket(i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ticket, status, err := b.store.TicketByChannel(ctx, i.ChannelID)
	if err != nil {
		slog.Error("buscar ticket", "channel", i.ChannelID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if ticket == nil || status != "closed" {
		b.replyEphemeral(i, "Este ticket no se puede eliminar.")
		return
	}
	settings, err := b.store.GetTicketSettings(ctx, ticket.GuildID)
	if err != nil || settings == nil {
		slog.Error("leer configuracion de tickets", "guild", ticket.GuildID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if !isStaff(i.Member, b.staffRoleFor(ctx, ticket, settings)) {
		b.replyEphemeral(i, "Solo el staff puede eliminar tickets.")
		return
	}

	b.replyEphemeral(i, "Eliminando el ticket…")
	if _, err := b.session.ChannelDelete(ticket.ChannelID); err != nil {
		slog.Error("eliminar canal", "channel", ticket.ChannelID, "err", err)
	}
}

// transcribeTicket guarda la conversación de un ticket cerrado, solo si el staff lo pide.
func (b *Bot) transcribeTicket(i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ticket, status, err := b.store.TicketByChannel(ctx, i.ChannelID)
	if err != nil {
		slog.Error("buscar ticket", "channel", i.ChannelID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if ticket == nil || status != "closed" {
		b.replyEphemeral(i, "Este ticket no se puede transcribir.")
		return
	}
	settings, err := b.store.GetTicketSettings(ctx, ticket.GuildID)
	if err != nil || settings == nil {
		slog.Error("leer configuracion de tickets", "guild", ticket.GuildID, "err", err)
		b.replyEphemeral(i, msgInternal)
		return
	}
	if !isStaff(i.Member, b.staffRoleFor(ctx, ticket, settings)) {
		b.replyEphemeral(i, "Solo el staff puede transcribir tickets.")
		return
	}

	// Leer el historial puede tardar más de los 3 s que da Discord: se difiere la respuesta.
	if err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}); err != nil {
		slog.Error("responder interaccion", "err", err)
		return
	}

	var msg string
	n, hidden, err := b.saveTranscript(ctx, ticket)
	switch {
	case err != nil:
		slog.Error("guardar transcript", "channel", ticket.ChannelID, "err", err)
		msg = "No se pudo guardar el transcript. Inténtalo de nuevo."
	case hidden:
		msg = fmt.Sprintf("Transcript guardado (%d mensajes), pero Discord ocultó el texto de la conversación. "+
			"Activa \"Message Content Intent\" en el Portal de Desarrolladores (Bot) y vuelve a pulsar Transcribir.", n)
	default:
		msg = fmt.Sprintf("Transcript guardado (%d mensajes). Puedes verlo en la app.", n)
	}
	if _, err := b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg}); err != nil {
		slog.Error("editar respuesta", "err", err)
	}
}
