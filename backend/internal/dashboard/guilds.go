package dashboard

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sedzz/askord/internal/ai"
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
	// sealer cifra las API keys de los LLM; nil si no hay AI_ENCRYPTION_KEY (IA desactivada).
	sealer *ai.Sealer
	llm    *ai.Client
	// discord son las consultas a Discord de los avisos del asistente; en los tests, un falso.
	discord discordAPI
}

// discordAPI es lo que el dashboard le pide a Discord para los avisos del asistente de IA.
type discordAPI interface {
	TextChannels(guildID string) ([]bot.Option, error)
	CreateStaffChannel(guildID, name, staffRoleID string) (bot.Option, error)
}

func New(cfg config.Config, s *store.Store, b *bot.Bot) *Handler {
	h := &Handler{cfg: cfg, store: s, bot: b, llm: ai.New(cfg.AIAllowPrivateURLs)}
	if b != nil {
		h.discord = b
	}
	if len(cfg.AIEncryptionKey) > 0 {
		// Config.Load ya comprobó que la clave mide 32 bytes.
		h.sealer, _ = ai.NewSealer(cfg.AIEncryptionKey)
	}
	return h
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

// RemoveGuild quita el bot del servidor y borra todos sus datos, transcripciones
// incluidas. Es repetible: si el bot no llega a salir, se puede volver a intentar.
func (h *Handler) RemoveGuild(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}

	settings, err := h.store.GetTicketSettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de tickets", "guild", guildID, "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	// El panel publicado quedaría con botones muertos: se borra, si se puede.
	if settings != nil && settings.PanelChannelID != "" && settings.PanelMessageID != "" {
		if err := h.bot.DeleteMessage(settings.PanelChannelID, settings.PanelMessageID); err != nil {
			slog.Warn("borrar panel", "guild", guildID, "err", err)
		}
	}

	if err := h.store.DeleteGuildData(r.Context(), guildID); err != nil {
		slog.Error("borrar datos del servidor", "guild", guildID, "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if err := h.bot.LeaveGuild(guildID); err != nil {
		slog.Error("salir del servidor", "guild", guildID, "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	if err := h.store.DeleteGuild(r.Context(), guildID); err != nil {
		slog.Error("borrar servidor", "guild", guildID, "err", err)
	}

	slog.Info("bot eliminado del servidor desde la app", "guild", guildID, "by", auth.UserFrom(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}
