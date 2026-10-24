package bot

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

const (
	customIDOpenTicket = "ticket:open" // paneles antiguos, y base de "ticket:open:<tipo>"
	customIDSelectType = "ticket:select"
)

const (
	defaultTypeEmoji = "📩"
	// Una fila de Discord admite cinco botones; con más tipos se usa un desplegable.
	maxPanelButtons = 5
)

// openCustomID es el identificador del botón que abre un ticket de ese tipo.
func openCustomID(typeID int64) string {
	if typeID == 0 {
		return customIDOpenTicket
	}
	return customIDOpenTicket + ":" + strconv.FormatInt(typeID, 10)
}

// parseTypeID lee el tipo de un identificador "ticket:open:<tipo>". Devuelve 0 si no lo trae.
func parseTypeID(customID string) int64 {
	rest, ok := strings.CutPrefix(customID, customIDOpenTicket+":")
	if !ok {
		return 0
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

// ValidEmoji dice si un texto puede usarse como emoji de un botón: Discord rechaza el
// mensaje entero si no lo es. Debe llevar algún carácter no ASCII y ninguna letra.
func ValidEmoji(s string) bool {
	nonASCII := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			return false
		}
		if r > unicode.MaxASCII {
			nonASCII = true
		}
	}
	return nonASCII
}

func emojiOf(t store.TicketType) *discordgo.ComponentEmoji {
	if t.Emoji != "" && ValidEmoji(t.Emoji) {
		return &discordgo.ComponentEmoji{Name: t.Emoji}
	}
	return &discordgo.ComponentEmoji{Name: defaultTypeEmoji}
}

// panelMessage construye el contenido del panel: un botón por tipo (hasta cinco) o, si hay
// más, un desplegable. Con un solo tipo se queda el botón de siempre, "Crear ticket".
func panelMessage(types []store.TicketType) ([]*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	if len(types) == 0 {
		types = []store.TicketType{{Emoji: defaultTypeEmoji}} // sin ID: abre el primer tipo que haya
	}

	description := "¿Necesitas ayuda? Pulsa el botón para abrir un ticket privado con el staff."
	if len(types) > 1 {
		description = "¿Necesitas ayuda? Elige el tipo de ticket que necesitas y se abrirá un canal privado con el staff."
	}
	embeds := []*discordgo.MessageEmbed{{Title: "Soporte", Description: description, Color: 0xFFC5D3}}

	if len(types) <= maxPanelButtons {
		row := discordgo.ActionsRow{}
		for _, t := range types {
			label := t.Name
			if len(types) == 1 {
				label = "Crear ticket"
			}
			row.Components = append(row.Components, discordgo.Button{
				Label:    label,
				Style:    discordgo.SecondaryButton,
				CustomID: openCustomID(t.ID),
				Emoji:    emojiOf(t),
			})
		}
		return embeds, []discordgo.MessageComponent{row}
	}

	options := make([]discordgo.SelectMenuOption, 0, len(types))
	for _, t := range types {
		options = append(options, discordgo.SelectMenuOption{
			Label:       t.Name,
			Value:       strconv.FormatInt(t.ID, 10),
			Description: t.Description,
			Emoji:       emojiOf(t),
		})
	}
	return embeds, []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{
			MenuType:    discordgo.StringSelectMenu,
			CustomID:    customIDSelectType,
			Placeholder: "Elige el tipo de ticket",
			Options:     options,
		},
	}}}
}

// PublishPanel envía el panel al canal y borra el anterior, si había.
func (b *Bot) PublishPanel(channelID, oldChannelID, oldMessageID string, types []store.TicketType) (string, error) {
	embeds, components := panelMessage(types)
	msg, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     embeds,
		Components: components,
	})
	if err != nil {
		return "", wrapPermissionError(err)
	}
	if oldChannelID != "" && oldMessageID != "" {
		_ = b.session.ChannelMessageDelete(oldChannelID, oldMessageID) // mejor esfuerzo
	}
	return msg.ID, nil
}

// UpdatePanel actualiza en su sitio un panel ya publicado (por ejemplo, tras añadir un tipo),
// sin enviar un mensaje nuevo.
func (b *Bot) UpdatePanel(channelID, messageID string, types []store.TicketType) error {
	embeds, components := panelMessage(types)
	_, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    channelID,
		ID:         messageID,
		Embeds:     &embeds,
		Components: &components,
	})
	return wrapPermissionError(err)
}
