package dashboard

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sedzz/askord/internal/auth"
	"github.com/sedzz/askord/internal/bot"
	"github.com/sedzz/askord/internal/config"
	"github.com/sedzz/askord/internal/store"
)

// Permisos que se piden al invitar al bot: gestionar canales y roles, ver
// canales, enviar mensajes, leer historial, crear hilos privados y escribir en hilos.
const botPermissions = 1<<4 | 1<<28 | 1<<10 | 1<<11 | 1<<16 | 1<<36 | 1<<38

type Handler struct {
	cfg   config.Config
	store *store.Store
	bot   *bot.Bot
}

func New(cfg config.Config, s *store.Store, b *bot.Bot) *Handler {
	return &Handler{cfg: cfg, store: s, bot: b}
}

type guildResponse struct {
	store.GuildView
	InviteURL string `json:"inviteUrl"`
}

// Guilds lista los servidores que el usuario administra y si el bot está en cada uno.
func (h *Handler) Guilds(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	guilds, err := h.store.ListUserGuilds(r.Context(), user.ID)
	if err != nil {
		slog.Error("listar servidores", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}

	out := make([]guildResponse, 0, len(guilds))
	for _, g := range guilds {
		out = append(out, guildResponse{GuildView: g, InviteURL: h.inviteURL(g.ID)})
	}
	writeJSON(w, out)
}

func (h *Handler) inviteURL(guildID string) string {
	q := url.Values{}
	q.Set("client_id", h.cfg.DiscordClientID)
	q.Set("scope", "bot applications.commands")
	q.Set("permissions", strconv.FormatUint(botPermissions, 10))
	q.Set("guild_id", guildID)
	q.Set("disable_guild_select", "true")
	return "https://discord.com/oauth2/authorize?" + q.Encode()
}
