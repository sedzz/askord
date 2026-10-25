package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/ai"
	"github.com/sedzz/askord/internal/store"
)

const (
	// Espera tras el último mensaje antes de responder: si el usuario escribe varios seguidos,
	// el asistente contesta una sola vez a todos.
	assistantDelay = 3 * time.Second
	// Cuánto se recuerda que un canal no es de un ticket, para no consultar la base de datos
	// por cada mensaje de cualquier canal del servidor.
	notTicketTTL     = 30 * time.Second
	notTicketMaxSize = 10000
	// Un mensaje de Discord admite 2000 caracteres.
	discordMessageMax = 2000
	typingEvery       = 8 * time.Second
	assistantTimeout  = 90 * time.Second
	maxReasonChars    = 300
)

// assistant es el asistente de IA de los tickets: lo que necesita el bot para responder.
type assistant struct {
	sealer *ai.Sealer
	llm    *ai.Client
	delay  time.Duration

	mu        sync.Mutex
	runs      map[string]*channelRun // por canal de ticket
	notTicket map[string]time.Time   // canal -> hasta cuándo se da por no-ticket
}

// channelRun es el estado de espera y ejecución del asistente en un canal.
type channelRun struct {
	timer   *time.Timer
	running bool
	again   bool // llegó algo mientras respondía: hay que volver a mirar
	initial bool // la primera respuesta, tras abrir el ticket con formulario
}

// EnableAI activa el asistente de IA. Sin llamarlo, el bot no responde a los mensajes.
func (b *Bot) EnableAI(sealer *ai.Sealer, llm *ai.Client) {
	b.ai = &assistant{
		sealer:    sealer,
		llm:       llm,
		delay:     assistantDelay,
		runs:      map[string]*channelRun{},
		notTicket: map[string]time.Time{},
	}
}

// onMessageCreate atiende cada mensaje del servidor. Solo le importan los de canales de ticket.
func (b *Bot) onMessageCreate(_ *discordgo.Session, m *discordgo.MessageCreate) {
	a := b.ai
	if a == nil || m.GuildID == "" || m.Author == nil || m.Author.Bot {
		return
	}
	if m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply {
		return
	}
	if m.Content == "" && len(m.Attachments) == 0 {
		return
	}
	if a.knownNotTicket(m.ChannelID) {
		return
	}

	ctx, cancel := timeout()
	defer cancel()
	ticket, status, err := b.store.TicketByChannel(ctx, m.ChannelID)
	if err != nil {
		slog.Error("buscar ticket", "channel", m.ChannelID, "err", err)
		return
	}
	if ticket == nil {
		a.rememberNotTicket(m.ChannelID)
		return
	}
	if status != "open" || ticket.AIState != store.AIStateActive {
		return
	}

	// Quien abrió el ticket es siempre el usuario, aunque sea administrador: así el dueño del
	// servidor puede probar el asistente abriéndose un ticket. Cualquier otro del staff, al
	// escribir, toma el relevo y el asistente se calla.
	if m.Author.ID != ticket.OpenerID && b.isStaffMember(ctx, m, ticket) {
		if paused, err := b.store.SetAIState(ctx, ticket.ChannelID, store.AIStatePaused); err != nil {
			slog.Error("pausar el asistente", "channel", ticket.ChannelID, "err", err)
		} else if paused {
			slog.Info("asistente en pausa: escribió el staff", "channel", ticket.ChannelID, "by", m.Author.ID)
		}
		return
	}
	a.schedule(b, m.ChannelID, false)
}

// isStaffMember: el autor tiene el rol de staff del ticket o es administrador del servidor.
func (b *Bot) isStaffMember(ctx context.Context, m *discordgo.MessageCreate, ticket *store.Ticket) bool {
	var roles []string
	if m.Member != nil {
		roles = m.Member.Roles
	}
	if settings, err := b.store.GetTicketSettings(ctx, ticket.GuildID); err == nil && settings != nil {
		if staffRole := b.staffRoleFor(ctx, ticket, settings); staffRole != "" && slices.Contains(roles, staffRole) {
			return true
		}
	}
	return b.isGuildAdmin(ticket.GuildID, m.Author.ID, roles)
}

// isGuildAdmin lo deduce de lo que el bot ya tiene en memoria (dueño y roles del servidor), sin
// preguntar a Discord: los mensajes no traen los permisos del autor.
func (b *Bot) isGuildAdmin(guildID, userID string, roleIDs []string) bool {
	g, err := b.session.State.Guild(guildID)
	if err != nil {
		return false
	}
	if g.OwnerID == userID {
		return true
	}
	for _, r := range g.Roles {
		if r.Permissions&discordgo.PermissionAdministrator != 0 && slices.Contains(roleIDs, r.ID) {
			return true
		}
	}
	return false
}

func (a *assistant) knownNotTicket(channelID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	until, ok := a.notTicket[channelID]
	if ok && time.Now().Before(until) {
		return true
	}
	delete(a.notTicket, channelID)
	return false
}

func (a *assistant) rememberNotTicket(channelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.notTicket) >= notTicketMaxSize {
		clear(a.notTicket)
	}
	a.notTicket[channelID] = time.Now().Add(notTicketTTL)
}

// forgetNotTicket se llama al crear un ticket: su canal deja de darse por no-ticket.
func (a *assistant) forgetNotTicket(channelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.notTicket, channelID)
}

// schedule programa una respuesta tras la espera; cada llamada nueva reinicia la cuenta.
func (a *assistant) schedule(b *Bot, channelID string, initial bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	run := a.runs[channelID]
	if run == nil {
		run = &channelRun{}
		a.runs[channelID] = run
	}
	if run.timer != nil {
		run.timer.Stop()
	}
	run.initial = run.initial || initial
	run.timer = time.AfterFunc(a.delay, func() { b.runAssistant(channelID) })
}

// runAssistant responde en un canal. Nunca hay dos a la vez en el mismo canal: si llega algo
// mientras responde, vuelve a mirar al terminar.
func (b *Bot) runAssistant(channelID string) {
	a := b.ai
	a.mu.Lock()
	run := a.runs[channelID]
	if run == nil {
		a.mu.Unlock()
		return
	}
	if run.running {
		run.again = true
		a.mu.Unlock()
		return
	}
	run.running, run.timer = true, nil
	initial := run.initial
	run.initial = false
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		run.running = false
		again := run.again
		run.again = false
		if !again && run.timer == nil {
			delete(a.runs, channelID)
		}
		a.mu.Unlock()
		if again {
			a.schedule(b, channelID, false)
		}
	}()

	b.respond(channelID, initial)
}

// respond hace una pasada del asistente: comprueba que le toca, lee la conversación, consulta
// al modelo y publica la respuesta, o pasa el ticket a una persona.
func (b *Bot) respond(channelID string, initial bool) {
	ctx, cancel := context.WithTimeout(context.Background(), assistantTimeout)
	defer cancel()

	ticket, status, err := b.store.TicketByChannel(ctx, channelID)
	if err != nil {
		slog.Error("buscar ticket", "channel", channelID, "err", err)
		return
	}
	if ticket == nil || status != "open" || ticket.AIState != store.AIStateActive || ticket.TypeID == 0 {
		return
	}
	settings, err := b.store.GetAISettings(ctx, ticket.GuildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "guild", ticket.GuildID, "err", err)
		return
	}
	if settings == nil || !settings.Enabled || len(settings.KeyEnc) == 0 {
		return
	}
	ticketType, err := b.store.GetTicketType(ctx, ticket.GuildID, ticket.TypeID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("leer tipo de ticket", "guild", ticket.GuildID, "err", err)
		}
		return
	}
	if !ticketType.AIEnabled {
		return
	}

	history, err := b.conversation(ticket)
	if err != nil {
		slog.Error("leer la conversacion del ticket", "channel", channelID, "err", err)
		return
	}
	// Si lo último es del propio asistente no hay nada nuevo que contestar (salvo la primera
	// respuesta, cuando lo último es la bienvenida).
	if !initial && (len(history) == 0 || history[len(history)-1].Assistant) {
		return
	}

	switch claim, err := b.store.ClaimAIReply(ctx, channelID, ai.MaxReplies); {
	case err != nil:
		slog.Error("reservar respuesta del asistente", "channel", channelID, "err", err)
		return
	case claim == store.ClaimLimitReached:
		b.escalate(ctx, ticket,
			"He llegado al límite de respuestas automáticas, así que te atenderá una persona del equipo.",
			fmt.Sprintf("Se alcanzó el límite de %d respuestas del asistente.", ai.MaxReplies))
		return
	case claim == store.ClaimNotActive:
		return
	}

	key, err := b.ai.sealer.Open(settings.KeyEnc, ticket.GuildID)
	if err != nil {
		slog.Error("descifrar la API key", "guild", ticket.GuildID, "err", err)
		b.failAndEscalate(ctx, ticket, "no se pudo leer la API key guardada; hay que volver a introducirla")
		return
	}

	stopTyping := b.keepTyping(ctx, channelID)
	resp, err := b.ai.llm.Chat(ctx, ai.Config{BaseURL: settings.BaseURL, Model: settings.Model, APIKey: key}, ai.ChatRequest{
		Messages:  ai.BuildMessages(ticketContext(settings, ticketType, ticket), history),
		Tools:     []ai.Tool{ai.EscalateTool},
		MaxTokens: ai.MaxReplyTokens,
	})
	stopTyping()
	if err != nil {
		slog.Error("consultar al modelo", "guild", ticket.GuildID, "channel", channelID, "err", err)
		b.failAndEscalate(ctx, ticket, err.Error())
		return
	}

	// Mientras el modelo pensaba pudo cerrarse el ticket o escribir el staff: entonces la
	// respuesta ya no pinta nada.
	if current, st, err := b.store.TicketByChannel(ctx, channelID); err != nil || current == nil ||
		st != "open" || current.AIState != store.AIStateActive {
		slog.Info("respuesta del asistente descartada: el ticket cambió", "channel", channelID)
		return
	}

	if resp.Content != "" {
		b.say(channelID, resp.Content)
	}
	for _, call := range resp.ToolCalls {
		if call.Name != ai.EscalateToolName {
			continue
		}
		reason := call.Argument("motivo")
		if reason == "" {
			reason = "El asistente pidió pasar el ticket a una persona."
		}
		b.escalate(ctx, ticket, "🙋 Este ticket necesita a una persona del equipo.", reason)
		return
	}
	slog.Info("el asistente respondió", "guild", ticket.GuildID, "channel", channelID,
		"prompt_tokens", resp.PromptTokens, "completion_tokens", resp.CompletionTokens)
}

func ticketContext(settings *store.AISettings, t store.TicketType, ticket *store.Ticket) ai.TicketContext {
	answers := make([]ai.FormAnswer, 0, len(ticket.Answers))
	for _, an := range ticket.Answers {
		answers = append(answers, ai.FormAnswer{Question: an.Question, Answer: an.Answer})
	}
	return ai.TicketContext{
		TypeName:        t.Name,
		TypeDescription: t.Description,
		Guidelines:      settings.Guidelines,
		Instructions:    t.AIInstructions,
		Escalation:      t.AIEscalation,
		Answers:         answers,
	}
}

// conversation lee los últimos mensajes del canal, del más antiguo al más reciente.
func (b *Bot) conversation(ticket *store.Ticket) ([]ai.Turn, error) {
	messages, err := b.session.ChannelMessages(ticket.ChannelID, ai.MaxHistoryTurns, "", "", "")
	if err != nil {
		return nil, err
	}
	roles := b.roleNames(ticket.GuildID)
	me := ""
	if b.session.State.User != nil {
		me = b.session.State.User.ID
	}

	turns := make([]ai.Turn, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- { // Discord los devuelve del más nuevo al más viejo
		m := messages[i]
		if m.Author == nil || (m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply) {
			continue
		}
		mine := m.Author.ID == me
		if m.Author.Bot && !mine {
			continue // otros bots no son parte de la conversación
		}
		content := readableContent(m, roles)
		for _, att := range m.Attachments {
			content += "\n[adjunto: " + att.Filename + "]"
		}
		turns = append(turns, ai.Turn{
			Author:    cmpName(m.Author),
			Content:   strings.TrimSpace(content),
			Assistant: mine,
		})
	}
	return turns, nil
}

func cmpName(u *discordgo.User) string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// keepTyping muestra "escribiendo…" mientras el modelo responde. Devuelve la función que lo para.
func (b *Bot) keepTyping(ctx context.Context, channelID string) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(typingEvery)
		defer t.Stop()
		for {
			if err := b.session.ChannelTyping(channelID); err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return cancel
}

// noMentions evita que un mensaje del asistente avise a nadie, diga lo que diga el modelo.
func noMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
}

// say publica un texto en el canal, partido en varios mensajes si no cabe en uno.
func (b *Bot) say(channelID, text string) {
	for _, part := range splitMessage(text, discordMessageMax) {
		if _, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content:         part,
			AllowedMentions: noMentions(),
		}); err != nil {
			slog.Error("enviar mensaje del asistente", "channel", channelID, "err", err)
			return
		}
	}
}

// splitMessage parte un texto en trozos de como mucho max caracteres, cortando por un salto de
// línea o un espacio cuando se puede.
func splitMessage(text string, max int) []string {
	runes := []rune(strings.TrimSpace(text))
	var parts []string
	for len(runes) > max {
		cut := max
		for i := max; i > max/2; i-- {
			if runes[i-1] == '\n' {
				cut = i
				break
			}
			if runes[i-1] == ' ' && cut == max {
				cut = i
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

// escalate pasa el ticket a una persona: el asistente se calla y el staff recibe el aviso con el
// motivo. Solo avisa una vez aunque se pida varias a la vez.
func (b *Bot) escalate(ctx context.Context, ticket *store.Ticket, headline, reason string) {
	changed, err := b.store.SetAIState(ctx, ticket.ChannelID, store.AIStateEscalated)
	if err != nil {
		slog.Error("escalar el ticket", "channel", ticket.ChannelID, "err", err)
		return
	}
	if !changed {
		return
	}

	var mention string
	allowed := noMentions()
	if settings, err := b.store.GetTicketSettings(ctx, ticket.GuildID); err == nil && settings != nil {
		if role := b.staffRoleFor(ctx, ticket, settings); role != "" {
			mention = "<@&" + role + "> "
			allowed.Roles = []string{role}
		}
	}
	content := fmt.Sprintf("%s%s\n**Motivo:** %s", mention, headline, truncate(strings.TrimSpace(reason), maxReasonChars))
	if _, err := b.session.ChannelMessageSendComplex(ticket.ChannelID, &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: allowed,
	}); err != nil {
		slog.Error("avisar al staff", "channel", ticket.ChannelID, "err", err)
	}
	slog.Info("ticket pasado a una persona", "guild", ticket.GuildID, "channel", ticket.ChannelID, "reason", reason)
}

// failAndEscalate: el proveedor de IA falló. El usuario no se queda esperando una respuesta que no
// va a llegar: se avisa y se pasa el ticket al staff con el motivo técnico.
func (b *Bot) failAndEscalate(ctx context.Context, ticket *store.Ticket, reason string) {
	b.escalate(ctx, ticket, "No puedo responder ahora mismo, así que te atenderá una persona del equipo.", reason)
}
