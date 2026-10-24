package bot

import (
	"errors"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

// LeaveGuild hace que el bot salga del servidor. Si ya no estaba, no es un error.
func (b *Bot) LeaveGuild(guildID string) error {
	err := b.session.GuildLeave(guildID)
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// DeleteMessage borra un mensaje del bot (por ejemplo, el panel de tickets).
func (b *Bot) DeleteMessage(channelID, messageID string) error {
	return b.session.ChannelMessageDelete(channelID, messageID)
}
