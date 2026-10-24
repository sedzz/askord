package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

type Bot struct {
	session *discordgo.Session
	store   *store.Store
}

func New(token string, s *store.Store) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	b := &Bot{session: session, store: s}
	session.AddHandler(b.onReady)
	session.AddHandler(b.onGuildCreate)
	session.AddHandler(b.onGuildUpdate)
	session.AddHandler(b.onGuildDelete)
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
