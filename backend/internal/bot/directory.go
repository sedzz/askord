package bot

import (
	"slices"

	"github.com/bwmarrin/discordgo"
)

// Canales y roles se leen de la memoria del bot, que Discord mantiene al día con eventos
// (canal creado, rol editado…). Así el dashboard no espera una ida y vuelta a Discord por
// cada consulta; solo si el servidor aún no está en memoria se pregunta a Discord.

func (b *Bot) guildChannels(guildID string) ([]*discordgo.Channel, error) {
	if g, err := b.session.State.Guild(guildID); err == nil && len(g.Channels) > 0 {
		return slices.Clone(g.Channels), nil
	}
	return b.session.GuildChannels(guildID)
}

func (b *Bot) guildRoles(guildID string) ([]*discordgo.Role, error) {
	if g, err := b.session.State.Guild(guildID); err == nil && len(g.Roles) > 0 {
		return slices.Clone(g.Roles), nil
	}
	return b.session.GuildRoles(guildID)
}

// remember anota en memoria un canal o rol recién creado sin esperar al evento de Discord,
// para que la siguiente consulta ya lo vea.
func (b *Bot) rememberChannel(c *discordgo.Channel) {
	_ = b.session.State.ChannelAdd(c)
}

func (b *Bot) rememberRole(guildID string, r *discordgo.Role) {
	_ = b.session.State.RoleAdd(guildID, r)
}
