package bot

import (
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GuildOverview es la ficha de un servidor para el dashboard.
type GuildOverview struct {
	Name          string    `json:"name"`
	Icon          string    `json:"icon"` // hash del icono; vacío si no tiene
	MemberCount   int       `json:"memberCount"`
	OnlineCount   int       `json:"onlineCount"`
	TextChannels  int       `json:"textChannels"`
	VoiceChannels int       `json:"voiceChannels"`
	Roles         int       `json:"roles"`
	CreatedAt     time.Time `json:"createdAt"`
}

// discordEpochMs es el 1 de enero de 2015 (en milisegundos), el origen de los IDs ("snowflakes") de Discord.
const discordEpochMs = 1420070400000

// Overview reúne los datos del servidor. Miembros y conectados vienen de Discord (conteos
// aproximados); canales y roles, de lo que el bot ya tiene en memoria.
func (b *Bot) Overview(guildID string) (GuildOverview, error) {
	g, err := b.session.GuildWithCounts(guildID)
	if err != nil {
		return GuildOverview{}, err
	}
	o := GuildOverview{
		Name:        g.Name,
		Icon:        g.Icon,
		MemberCount: g.ApproximateMemberCount,
		OnlineCount: g.ApproximatePresenceCount,
	}

	if id, err := strconv.ParseUint(guildID, 10, 64); err == nil {
		o.CreatedAt = time.UnixMilli(int64(id>>22) + discordEpochMs).UTC()
	}

	// Canales y roles salen del estado local, que el bot mantiene al día con los eventos.
	if state, err := b.session.State.Guild(guildID); err == nil {
		o.Roles = len(state.Roles)
		for _, c := range state.Channels {
			switch c.Type {
			case discordgo.ChannelTypeGuildText:
				o.TextChannels++
			case discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildStageVoice:
				o.VoiceChannels++
			}
		}
	}
	return o, nil
}
