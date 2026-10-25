package bot

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

type Bot struct {
	session *discordgo.Session
	store   *store.Store
	// Datos del servidor que vienen de Discord y se guardan un minuto (guildID -> cachedGuild).
	guildCache sync.Map
	// Asistente de IA de los tickets; nil si no está activado (ver EnableAI).
	ai *assistant
}

func New(token string, s *store.Store) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	// Los mensajes (y su texto, un intent privilegiado) solo se usan para el asistente de IA.
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	b := &Bot{session: session, store: s}
	session.AddHandler(b.onReady)
	session.AddHandler(b.onGuildCreate)
	session.AddHandler(b.onGuildUpdate)
	session.AddHandler(b.onGuildDelete)
	session.AddHandler(b.onInteraction)
	session.AddHandler(b.onChannelDelete)
	session.AddHandler(b.onMessageCreate)
	return b, nil
}

func (b *Bot) Start() error { return b.session.Open() }

func (b *Bot) Close() error { return b.session.Close() }

func (b *Bot) onReady(_ *discordgo.Session, r *discordgo.Ready) {
	slog.Info("bot conectado", "user", r.User.Username, "guilds", len(r.Guilds))

	// Limpia servidores de los que nos echaron mientras estábamos apagados.
	ids := make([]string, 0, len(r.Guilds))
	for _, g := range r.Guilds {
		ids = append(ids, g.ID)
	}
	ctx, cancel := timeout()
	defer cancel()
	if err := b.store.DeleteGuildsNotIn(ctx, ids); err != nil {
		slog.Error("limpiar servidores", "err", err)
	}
}

// GuildCreate llega al arrancar (uno por servidor) y cuando el bot entra a uno nuevo.
func (b *Bot) onGuildCreate(_ *discordgo.Session, e *discordgo.GuildCreate) {
	b.saveGuild(e.Guild)
}

func (b *Bot) onGuildUpdate(_ *discordgo.Session, e *discordgo.GuildUpdate) {
	b.saveGuild(e.Guild)
}

func (b *Bot) onGuildDelete(_ *discordgo.Session, e *discordgo.GuildDelete) {
	// Unavailable significa caída temporal de Discord, no que nos hayan echado.
	if e.Unavailable {
		return
	}
	ctx, cancel := timeout()
	defer cancel()
	if err := b.store.DeleteGuild(ctx, e.ID); err != nil {
		slog.Error("borrar servidor", "guild", e.ID, "err", err)
		return
	}
	slog.Info("bot eliminado de un servidor", "guild", e.ID)
}

func (b *Bot) saveGuild(g *discordgo.Guild) {
	var icon *string
	if g.Icon != "" {
		icon = &g.Icon
	}
	ctx, cancel := timeout()
	defer cancel()
	if err := b.store.UpsertGuild(ctx, g.ID, g.Name, icon); err != nil {
		slog.Error("guardar servidor", "guild", g.ID, "err", err)
		return
	}
	slog.Info("servidor registrado", "guild", g.ID, "name", g.Name)
}

func timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// Option es un elemento elegible (categoría o rol) para los selectores del dashboard.
type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Categories devuelve las categorías de canales del servidor, en su orden.
func (b *Bot) Categories(guildID string) ([]Option, error) {
	channels, err := b.guildChannels(guildID)
	if err != nil {
		return nil, err
	}
	cats := []*discordgo.Channel{}
	for _, c := range channels {
		if c.Type == discordgo.ChannelTypeGuildCategory {
			cats = append(cats, c)
		}
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Position < cats[j].Position })

	out := make([]Option, 0, len(cats))
	for _, c := range cats {
		out = append(out, Option{ID: c.ID, Name: c.Name})
	}
	return out, nil
}

// Roles devuelve los roles del servidor de mayor a menor, sin @everyone ni los de bots.
func (b *Bot) Roles(guildID string) ([]Option, error) {
	roles, err := b.guildRoles(guildID)
	if err != nil {
		return nil, err
	}
	usable := []*discordgo.Role{}
	for _, r := range roles {
		if r.ID != guildID && !r.Managed {
			usable = append(usable, r)
		}
	}
	sort.Slice(usable, func(i, j int) bool { return usable[i].Position > usable[j].Position })

	out := make([]Option, 0, len(usable))
	for _, r := range usable {
		out = append(out, Option{ID: r.ID, Name: r.Name})
	}
	return out, nil
}

// ErrMissingPermissions indica que el bot no tiene permiso en el servidor para la acción.
var ErrMissingPermissions = errors.New("el bot no tiene permisos suficientes")

func wrapPermissionError(err error) error {
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusForbidden {
		return ErrMissingPermissions
	}
	return err
}

// CreateCategory crea una categoría de canales vacía.
func (b *Bot) CreateCategory(guildID, name string) (Option, error) {
	c, err := b.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		return Option{}, wrapPermissionError(err)
	}
	b.rememberChannel(c)
	return Option{ID: c.ID, Name: c.Name}, nil
}

// CreateRole crea un rol sin permisos especiales.
func (b *Bot) CreateRole(guildID, name string) (Option, error) {
	r, err := b.session.GuildRoleCreate(guildID, &discordgo.RoleParams{Name: name})
	if err != nil {
		return Option{}, wrapPermissionError(err)
	}
	b.rememberRole(guildID, r)
	return Option{ID: r.ID, Name: r.Name}, nil
}
