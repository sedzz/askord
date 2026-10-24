package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

const customIDOpenTicket = "ticket:open"

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
	channels, err := b.session.GuildChannels(guildID)
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

// PublishPanel envía el mensaje con el botón "Crear ticket" y borra el panel anterior, si había.
func (b *Bot) PublishPanel(channelID, oldChannelID, oldMessageID string) (string, error) {
	msg, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       "Soporte",
			Description: "¿Necesitas ayuda? Pulsa el botón para abrir un ticket privado con el staff.",
			Color:       0x5865F2,
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Crear ticket",
					Style:    discordgo.PrimaryButton,
					CustomID: customIDOpenTicket,
					Emoji:    &discordgo.ComponentEmoji{Name: "🎫"},
				},
			}},
		},
	})
	if err != nil {
		return "", wrapPermissionError(err)
	}
	if oldChannelID != "" && oldMessageID != "" {
		_ = b.session.ChannelMessageDelete(oldChannelID, oldMessageID) // mejor esfuerzo
	}
	return msg.ID, nil
}

func (b *Bot) onInteraction(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent || i.GuildID == "" || i.Member == nil {
		return
	}
	if i.MessageComponentData().CustomID == customIDOpenTicket {
		b.openTicket(i)
	}
}

// openTicket responde en privado al usuario que pulsó el botón.
func (b *Bot) openTicket(i *discordgo.InteractionCreate) {
	// Crear el canal puede tardar más que los 3 s que da Discord: se difiere la respuesta.
	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		slog.Error("responder interaccion", "err", err)
		return
	}

	msg := b.createTicket(i.GuildID, i.Member.User)
	if _, err := b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg}); err != nil {
		slog.Error("editar respuesta", "err", err)
	}
}

// createTicket crea el canal privado y devuelve el mensaje para el usuario.
func (b *Bot) createTicket(guildID string, user *discordgo.User) string {
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

	existing, err := b.store.OpenTicketByUser(ctx, guildID, user.ID)
	if err != nil {
		slog.Error("buscar ticket abierto", "guild", guildID, "err", err)
		return msgInternal
	}
	if existing != nil {
		if b.channelExists(existing.ChannelID) {
			return fmt.Sprintf("Ya tienes un ticket abierto: <#%s>", existing.ChannelID)
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
		ParentID: settings.CategoryID,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			// @everyone usa como ID el del propio servidor.
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
			{ID: user.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketMemberPerms},
			{ID: settings.StaffRoleID, Type: discordgo.PermissionOverwriteTypeRole, Allow: ticketMemberPerms},
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
		GuildID: guildID, Number: number, ChannelID: channel.ID, OpenerID: user.ID,
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
			"Hola <@%s>, gracias por abrir un ticket. Cuéntanos tu problema y <@&%s> te atenderá en cuanto pueda.",
			user.ID, settings.StaffRoleID),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: []string{user.ID},
			Roles: []string{settings.StaffRoleID},
		},
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
	return Option{ID: c.ID, Name: c.Name}, nil
}
