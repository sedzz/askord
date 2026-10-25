package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/ai"
	"github.com/sedzz/askord/internal/store"
	"github.com/sedzz/askord/internal/store/storetest"
)

// fakeLLM hace de API compatible con OpenAI: apunta cada petición y responde lo que diga handle.
type fakeLLM struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests []llmRequest
	handle   func(call int, w http.ResponseWriter)
}

type llmRequest struct {
	Auth     string
	Messages []struct{ Role, Content string }
	Tools    []any
}

func newFakeLLM(t *testing.T) *fakeLLM {
	t.Helper()
	l := &fakeLLM{handle: func(_ int, w http.ResponseWriter) { llmText(w, "Respuesta del modelo") }}
	l.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		req := llmRequest{Auth: r.Header.Get("Authorization"), Tools: body.Tools}
		for _, m := range body.Messages {
			req.Messages = append(req.Messages, struct{ Role, Content string }{m.Role, m.Content})
		}
		l.mu.Lock()
		l.requests = append(l.requests, req)
		call, handle := len(l.requests), l.handle
		l.mu.Unlock()
		handle(call, w)
	}))
	t.Cleanup(l.srv.Close)
	return l
}

func llmJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func llmText(w http.ResponseWriter, text string) {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}}},
		"usage": map[string]int{"prompt_tokens": 50, "completion_tokens": 10}})
	llmJSON(w, 200, string(b))
}

// llmTools responde con texto (puede ir vacío) y una llamada a escalar_a_staff por cada motivo.
func llmTools(w http.ResponseWriter, text string, reasons ...string) {
	var calls []any
	for _, r := range reasons {
		args, _ := json.Marshal(map[string]string{"motivo": r})
		calls = append(calls, map[string]any{"function": map[string]any{"name": ai.EscalateToolName, "arguments": string(args)}})
	}
	var content any
	if text != "" {
		content = text
	}
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content, "tool_calls": calls}}}})
	llmJSON(w, 200, string(b))
}

func (l *fakeLLM) calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.requests)
}

func (l *fakeLLM) request(i int) llmRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.requests[i]
}

// aiEnv es un servidor con el asistente listo: conexión, interruptor, pautas, un tipo con IA y un
// ticket abierto de ana en testChannel.
type aiEnv struct {
	t   *testing.T
	b   *Bot
	f   *fakeDiscord
	llm *fakeLLM
	st  *store.Store
	ty  store.TicketType
}

func newAIEnv(t *testing.T) *aiEnv {
	t.Helper()
	b, f, st := newTestBot(t)
	llm := newFakeLLM(t)
	ctx := context.Background()

	sealer, err := ai.NewSealer(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	b.EnableAI(sealer, ai.New(true))
	b.ai.delay = 20 * time.Millisecond

	seedGuild(t, st)
	key, _ := sealer.Seal("sk-test", "g1")
	if err := st.SaveAISettings(ctx, "g1", store.AISettings{BaseURL: llm.srv.URL, Model: "mi-modelo", KeyEnc: key}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAIBehavior(ctx, "g1", true, "Sé amable."); err != nil {
		t.Fatal(err)
	}
	ty, err := st.CreateTicketType(ctx, "g1", store.TicketTypeInput{
		Name: "Soporte", Description: "Ayuda general", AIEnabled: true,
		AIInstructions: "Pide el número de pedido.", AIEscalation: "Si piden un reembolso.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateTicket(ctx, store.Ticket{
		// El número 99 deja libres los primeros para los tickets que abran los tests.
		GuildID: "g1", Number: 99, ChannelID: testChannel, OpenerID: testOpener, TypeID: ty.ID,
	}); err != nil {
		t.Fatal(err)
	}
	return &aiEnv{t: t, b: b, f: f, llm: llm, st: st, ty: ty}
}

// history fija lo que Discord devuelve al leer el canal (de más antiguo a más reciente).
func (e *aiEnv) history(msgs ...histMsg) {
	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	items := make([]map[string]any, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- { // Discord los devuelve del más nuevo al más viejo
		m := msgs[i]
		items = append(items, map[string]any{
			"id": fmt.Sprint(1000 + i), "type": 0, "content": m.content,
			"timestamp": "2026-01-01T10:00:00Z", "attachments": m.attachments,
			"author": map[string]any{"id": m.author, "username": m.author, "bot": m.bot},
		})
	}
	b, _ := json.Marshal(items)
	e.f.messages = string(b)
}

type histMsg struct {
	author, content string
	bot             bool
	attachments     []map[string]string
}

func user(content string) histMsg    { return histMsg{author: testOpener, content: content} }
func botSays(content string) histMsg { return histMsg{author: "bot", content: content, bot: true} }

// say entrega a Askord un mensaje nuevo del canal, como lo haría Discord.
func (e *aiEnv) say(author, content string, roles ...string) {
	e.sayIn(testChannel, author, content, discordgo.MessageTypeDefault, false, roles...)
}

func (e *aiEnv) sayIn(channel, author, content string, typ discordgo.MessageType, isBot bool, roles ...string) {
	e.b.onMessageCreate(nil, &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-" + content, GuildID: "g1", ChannelID: channel, Content: content, Type: typ,
		Author: &discordgo.User{ID: author, Username: author, Bot: isBot},
		Member: &discordgo.Member{Roles: roles},
	}})
}

// posts devuelve el texto de los mensajes que el bot publicó en el canal.
func (e *aiEnv) posts() []string {
	var out []string
	for _, c := range e.f.find(http.MethodPost, "/channels/"+testChannel+"/messages") {
		var v struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal([]byte(c.Body), &v)
		out = append(out, v.Content)
	}
	return out
}

func (e *aiEnv) ticket() *store.Ticket {
	e.t.Helper()
	tk, _, err := e.st.TicketByChannel(context.Background(), testChannel)
	if err != nil || tk == nil {
		e.t.Fatalf("ticket: %v %v", tk, err)
	}
	return tk
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no se cumplió a tiempo: %s", what)
}

// quiet espera a que el asistente tenga tiempo de actuar (varias esperas) para comprobar que no lo hizo.
func (e *aiEnv) quiet() { time.Sleep(8 * e.b.ai.delay) }

func TestAssistantReplies(t *testing.T) {
	e := newAIEnv(t)
	e.history(user("no me llega el pedido"))

	e.say(testOpener, "no me llega el pedido")
	eventually(t, "una respuesta", func() bool { return len(e.posts()) == 1 })
	e.quiet()

	if got := e.posts(); len(got) != 1 || got[0] != "Respuesta del modelo" {
		t.Errorf("mensajes = %q", got)
	}
	if body := e.f.find(http.MethodPost, "/channels/"+testChannel+"/messages")[0].Body; !strings.Contains(body, `"parse":[]`) {
		t.Errorf("la respuesta debía ir sin menciones: %s", body)
	}
	if len(e.f.find(http.MethodPost, "/typing")) == 0 {
		t.Error("debía mostrar «escribiendo…» mientras el modelo respondía")
	}

	req := e.llm.request(0)
	if req.Auth != "Bearer sk-test" {
		t.Errorf("Authorization = %q: el bot debe descifrar la API key", req.Auth)
	}
	if len(req.Tools) != 1 {
		t.Errorf("herramientas = %d, quería la de escalar", len(req.Tools))
	}
	system := req.Messages[0]
	for _, want := range []string{"Sé amable.", "Pide el número de pedido.", "Si piden un reembolso.", "Soporte"} {
		if system.Role != "system" || !strings.Contains(system.Content, want) {
			t.Errorf("el prompt de sistema no contiene %q", want)
		}
	}
	if last := req.Messages[len(req.Messages)-1]; last.Role != "user" || last.Content != "ana: no me llega el pedido" {
		t.Errorf("último mensaje = %+v", last)
	}
	if tk := e.ticket(); tk.AIReplies != 1 || tk.AIState != store.AIStateActive {
		t.Errorf("ticket = %d respuestas, estado %q", tk.AIReplies, tk.AIState)
	}
}

func TestAssistantAnswersABurstOnce(t *testing.T) {
	e := newAIEnv(t)
	e.history(user("hola"), user("una cosa"), user("mi pedido no llega"))

	e.say(testOpener, "hola")
	e.say(testOpener, "una cosa")
	e.say(testOpener, "mi pedido no llega")
	eventually(t, "una respuesta", func() bool { return len(e.posts()) >= 1 })
	e.quiet()

	if e.llm.calls() != 1 || len(e.posts()) != 1 {
		t.Errorf("%d consultas y %d respuestas; quería 1 y 1", e.llm.calls(), len(e.posts()))
	}
	if last := e.llm.request(0).Messages; !strings.Contains(last[len(last)-1].Content, "ana: hola\nana: una cosa\nana: mi pedido no llega") {
		t.Errorf("el modelo debía recibir los tres mensajes juntos: %+v", last)
	}
}

func TestAssistantStaysQuiet(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *aiEnv)
		say   func(e *aiEnv)
	}{
		{"interruptor general apagado", func(e *aiEnv) {
			_ = e.st.SaveAIBehavior(context.Background(), "g1", false, "")
		}, nil},
		{"tipo sin IA", func(e *aiEnv) {
			_, _ = e.st.UpdateTicketType(context.Background(), "g1", e.ty.ID, store.TicketTypeInput{Name: "Soporte"})
		}, nil},
		{"sin conexión", func(e *aiEnv) { _ = e.st.DeleteAISettings(context.Background(), "g1") }, nil},
		{"ticket cerrado", func(e *aiEnv) { _, _ = e.st.CloseTicket(context.Background(), testChannel, testOpener) }, nil},
		{"ya escalado", func(e *aiEnv) { _, _ = e.st.SetAIState(context.Background(), testChannel, store.AIStateEscalated) }, nil},
		{"ticket sin tipo", func(e *aiEnv) { storetest.Exec(e.t, e.st, `UPDATE tickets SET type_id = NULL`) }, nil},
		{"autor es un bot", nil, func(e *aiEnv) {
			e.sayIn(testChannel, "otro-bot", "hola", discordgo.MessageTypeDefault, true)
		}},
		{"mensaje del sistema", nil, func(e *aiEnv) {
			e.sayIn(testChannel, testOpener, "se unió", discordgo.MessageTypeGuildMemberJoin, false)
		}},
		{"mensaje vacío", nil, func(e *aiEnv) { e.say(testOpener, "") }},
		{"canal que no es de un ticket", nil, func(e *aiEnv) {
			e.sayIn("otro-canal", testOpener, "hola", discordgo.MessageTypeDefault, false)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newAIEnv(t)
			e.history(user("hola"))
			if c.setup != nil {
				c.setup(e)
			}
			if c.say != nil {
				c.say(e)
			} else {
				e.say(testOpener, "hola")
			}
			e.quiet()
			if e.llm.calls() != 0 || len(e.posts()) != 0 {
				t.Errorf("%d consultas y %d mensajes; el asistente debía callarse", e.llm.calls(), len(e.posts()))
			}
		})
	}
}

func TestAssistantDoesNotAnswerItself(t *testing.T) {
	// Un segundo aviso sin nada nuevo (lo último del canal es la respuesta del propio bot).
	e := newAIEnv(t)
	e.history(user("hola"), botSays("Hola, ¿en qué te ayudo?"))
	e.say(testOpener, "hola")
	e.quiet()
	if e.llm.calls() != 0 {
		t.Errorf("consultó al modelo %d veces sin nada nuevo que contestar", e.llm.calls())
	}
	if tk := e.ticket(); tk.AIReplies != 0 {
		t.Errorf("contó %d respuestas sin responder", tk.AIReplies)
	}
}

func TestAssistantPausesWhenStaffSpeaks(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *aiEnv)
		roles []string
	}{
		{"rol de staff", nil, []string{testStaff}},
		{"rol de administrador", func(e *aiEnv) {
			_ = e.b.session.State.GuildAdd(&discordgo.Guild{ID: "g1", OwnerID: "dueño",
				Roles: []*discordgo.Role{{ID: "admin", Permissions: discordgo.PermissionAdministrator}}})
		}, []string{"admin"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newAIEnv(t)
			e.history(user("hola"))
			if c.setup != nil {
				c.setup(e)
			}
			e.say("luis", "Yo me encargo", c.roles...)
			if st := e.ticket().AIState; st != store.AIStatePaused {
				t.Fatalf("estado = %q, quería paused", st)
			}
			// Tras la pausa, el asistente no contesta aunque el usuario siga escribiendo.
			e.say(testOpener, "gracias")
			e.quiet()
			if e.llm.calls() != 0 || len(e.posts()) != 0 {
				t.Errorf("el asistente respondió estando en pausa (%d consultas)", e.llm.calls())
			}
		})
	}

	t.Run("el dueño del servidor", func(t *testing.T) {
		e := newAIEnv(t)
		_ = e.b.session.State.GuildAdd(&discordgo.Guild{ID: "g1", OwnerID: "dueño"})
		e.say("dueño", "Hola")
		if st := e.ticket().AIState; st != store.AIStatePaused {
			t.Errorf("estado = %q, quería paused", st)
		}
	})
}

// Quien abrió el ticket es siempre el usuario, aunque sea del staff: así se puede probar el
// asistente abriéndose uno mismo un ticket.
func TestAssistantAnswersTheOpenerEvenIfStaff(t *testing.T) {
	e := newAIEnv(t)
	_ = e.b.session.State.GuildAdd(&discordgo.Guild{ID: "g1", OwnerID: testOpener})
	e.history(user("probando"))

	e.say(testOpener, "probando", testStaff)
	eventually(t, "una respuesta", func() bool { return len(e.posts()) == 1 })
	if st := e.ticket().AIState; st != store.AIStateActive {
		t.Errorf("estado = %q, quería active", st)
	}
}

func TestAssistantEscalatesWithTheTool(t *testing.T) {
	e := newAIEnv(t)
	e.llm.handle = func(_ int, w http.ResponseWriter) {
		// Dos llamadas a la herramienta a la vez: el staff debe enterarse una sola vez.
		llmTools(w, "Te paso con una persona del equipo.", "Pide un reembolso", "Pide un reembolso")
	}
	e.history(user("quiero mi dinero"))

	e.say(testOpener, "quiero mi dinero")
	eventually(t, "el aviso al staff", func() bool { return len(e.posts()) == 2 })
	e.quiet()

	posts := e.posts()
	if len(posts) != 2 || posts[0] != "Te paso con una persona del equipo." {
		t.Fatalf("mensajes = %q", posts)
	}
	for _, want := range []string{"<@&" + testStaff + ">", "Pide un reembolso", "**Motivo:**"} {
		if !strings.Contains(posts[1], want) {
			t.Errorf("el aviso no contiene %q: %s", want, posts[1])
		}
	}
	notice := e.f.find(http.MethodPost, "/channels/"+testChannel+"/messages")[1].Body
	if !strings.Contains(notice, `"parse":[]`) || !strings.Contains(notice, `"roles":["`+testStaff+`"]`) {
		t.Errorf("el aviso solo debe mencionar al rol de staff: %s", notice)
	}
	if st := e.ticket().AIState; st != store.AIStateEscalated {
		t.Errorf("estado = %q, quería escalated", st)
	}

	// Escalado, ya no contesta.
	e.say(testOpener, "¿hola?")
	e.quiet()
	if e.llm.calls() != 1 || len(e.posts()) != 2 {
		t.Errorf("tras escalar: %d consultas y %d mensajes", e.llm.calls(), len(e.posts()))
	}
}

func TestAssistantEscalatesWithOnlyTheTool(t *testing.T) {
	e := newAIEnv(t)
	e.llm.handle = func(_ int, w http.ResponseWriter) { llmTools(w, "", "No sé resolverlo") }
	e.history(user("algo raro"))
	e.say(testOpener, "algo raro")
	eventually(t, "el aviso al staff", func() bool { return len(e.posts()) == 1 })
	e.quiet()
	if got := e.posts(); len(got) != 1 || !strings.Contains(got[0], "No sé resolverlo") {
		t.Errorf("mensajes = %q", got)
	}
}

func TestAssistantEscalatesAtTheLimit(t *testing.T) {
	e := newAIEnv(t)
	for range ai.MaxReplies {
		if r, err := e.st.ClaimAIReply(context.Background(), testChannel, ai.MaxReplies); err != nil || r != store.ClaimGranted {
			t.Fatalf("preparando el tope: %v %v", r, err)
		}
	}
	e.history(user("otra pregunta"))
	e.say(testOpener, "otra pregunta")
	eventually(t, "el aviso al staff", func() bool { return len(e.posts()) == 1 })
	e.quiet()

	if got := e.posts(); len(got) != 1 || !strings.Contains(got[0], "límite") || !strings.Contains(got[0], "<@&"+testStaff+">") {
		t.Errorf("mensajes = %q", got)
	}
	if e.llm.calls() != 0 {
		t.Error("no debía consultar al modelo con el tope alcanzado")
	}
	if st := e.ticket().AIState; st != store.AIStateEscalated {
		t.Errorf("estado = %q", st)
	}
}

func TestAssistantEscalatesWhenTheProviderFails(t *testing.T) {
	cases := []struct {
		name   string
		handle func(int, http.ResponseWriter)
		want   string
	}{
		{"clave inválida", func(_ int, w http.ResponseWriter) { llmJSON(w, 401, `{"error":{"message":"bad key sk-test"}}`) }, "API key no es válida"},
		{"sin saldo", func(_ int, w http.ResponseWriter) { llmJSON(w, 429, `{}`) }, "limitó las peticiones"},
		{"caído", func(_ int, w http.ResponseWriter) { llmJSON(w, 500, `{"error":{"message":"overloaded"}}`) }, "overloaded"},
		{"respuesta vacía", func(_ int, w http.ResponseWriter) { llmText(w, "") }, "respondió vacío"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newAIEnv(t)
			e.llm.handle = c.handle
			e.history(user("hola"))
			e.say(testOpener, "hola")
			eventually(t, "el aviso al staff", func() bool { return len(e.posts()) == 1 })
			e.quiet()

			got := e.posts()
			if len(got) != 1 || !strings.Contains(got[0], "No puedo responder ahora mismo") ||
				!strings.Contains(got[0], "<@&"+testStaff+">") || !strings.Contains(got[0], c.want) {
				t.Errorf("mensajes = %q, debía avisar al usuario y al staff con %q", got, c.want)
			}
			if strings.Contains(got[0], "sk-test") {
				t.Error("el aviso revela la API key")
			}
			if e.llm.calls() != 1 {
				t.Errorf("%d consultas; no debe reintentar", e.llm.calls())
			}
			if st := e.ticket().AIState; st != store.AIStateEscalated {
				t.Errorf("estado = %q, quería escalated", st)
			}
		})
	}
}

func TestAssistantEscalatesWhenTheKeyCannotBeRead(t *testing.T) {
	e := newAIEnv(t)
	storetest.Exec(t, e.st, `UPDATE ai_settings SET api_key_enc = '\x0102030405060708090a0b0c0d0e0f1011121314'`)
	e.history(user("hola"))
	e.say(testOpener, "hola")
	eventually(t, "el aviso al staff", func() bool { return len(e.posts()) == 1 })
	if got := e.posts()[0]; !strings.Contains(got, "volver a introducirla") {
		t.Errorf("aviso = %q", got)
	}
	if e.llm.calls() != 0 {
		t.Error("no debía consultar al modelo sin poder leer la clave")
	}
}

// Si mientras el modelo piensa el staff toma el relevo, la respuesta ya no pinta nada.
func TestAssistantDropsAReplyIfStaffTakesOver(t *testing.T) {
	e := newAIEnv(t)
	started, release := make(chan struct{}), make(chan struct{})
	e.llm.handle = func(_ int, w http.ResponseWriter) {
		close(started)
		<-release
		llmText(w, "Respuesta tardía")
	}
	e.history(user("hola"))
	e.say(testOpener, "hola")

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("el asistente no consultó al modelo")
	}
	e.say("luis", "Ya me ocupo yo", testStaff) // el staff escribe mientras el modelo piensa
	close(release)
	e.quiet()

	if got := e.posts(); len(got) != 0 {
		t.Errorf("publicó %q aunque el staff tomó el relevo", got)
	}
}

func TestAssistantDropsAReplyIfTheTicketClosed(t *testing.T) {
	e := newAIEnv(t)
	started, release := make(chan struct{}), make(chan struct{})
	e.llm.handle = func(_ int, w http.ResponseWriter) {
		close(started)
		<-release
		llmText(w, "Respuesta tardía")
	}
	e.history(user("hola"))
	e.say(testOpener, "hola")
	<-started
	_, _ = e.st.CloseTicket(context.Background(), testChannel, testOpener)
	close(release)
	e.quiet()
	if got := e.posts(); len(got) != 0 {
		t.Errorf("publicó %q en un ticket cerrado", got)
	}
}

// Un mensaje que llega mientras responde se atiende después, sin pisarse.
func TestAssistantHandlesAMessageDuringAReply(t *testing.T) {
	e := newAIEnv(t)
	started, release := make(chan struct{}), make(chan struct{})
	e.llm.handle = func(call int, w http.ResponseWriter) {
		if call == 1 {
			close(started)
			<-release
		}
		llmText(w, fmt.Sprintf("respuesta %d", call))
	}
	e.history(user("primero"))
	e.say(testOpener, "primero")
	<-started

	e.history(user("primero"), user("segundo"))
	e.say(testOpener, "segundo")
	time.Sleep(4 * e.b.ai.delay) // el temporizador salta mientras la primera consulta sigue en curso
	close(release)

	eventually(t, "las dos respuestas", func() bool { return e.llm.calls() == 2 })
	e.quiet()
	if e.llm.calls() > 2 {
		t.Errorf("%d consultas, quería 2", e.llm.calls())
	}
}

func TestAssistantSplitsLongReplies(t *testing.T) {
	e := newAIEnv(t)
	long := strings.TrimSpace(strings.Repeat("palabra ", 600)) // 4799 caracteres
	e.llm.handle = func(_ int, w http.ResponseWriter) { llmText(w, long) }
	e.history(user("cuéntame todo"))
	e.say(testOpener, "cuéntame todo")
	eventually(t, "la respuesta", func() bool { return len(e.posts()) >= 3 })
	e.quiet()

	got := e.posts()
	if len(got) != 3 {
		t.Fatalf("%d mensajes, quería 3", len(got))
	}
	for i, p := range got {
		if n := len([]rune(p)); n > discordMessageMax {
			t.Errorf("el mensaje %d mide %d caracteres", i, n)
		}
	}
	if strings.Join(got, " ") != long {
		t.Error("al partir la respuesta se perdió o se alteró texto")
	}
}

func TestAssistantConversationForTheModel(t *testing.T) {
	e := newAIEnv(t)
	e.history(
		botSays("Hola, gracias por abrir un ticket."),
		histMsg{author: "otro-bot", content: "ruido de otro bot", bot: true},
		user("mira esto"),
		histMsg{author: testOpener, attachments: []map[string]string{{"id": "1", "filename": "foto.png", "url": "https://x/foto.png"}}},
	)
	e.say(testOpener, "mira esto")
	eventually(t, "la consulta", func() bool { return e.llm.calls() == 1 })

	msgs := e.llm.request(0).Messages
	if len(msgs) != 3 || msgs[1].Role != "assistant" || msgs[1].Content != "Hola, gracias por abrir un ticket." {
		t.Fatalf("mensajes = %+v", msgs)
	}
	if msgs[2].Role != "user" || msgs[2].Content != "ana: mira esto\nana: [adjunto: foto.png]" {
		t.Errorf("mensaje del usuario = %q", msgs[2].Content)
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, "ruido de otro bot") {
			t.Error("los mensajes de otros bots no forman parte de la conversación")
		}
	}
}

func TestAssistantAnswersTheFormRightAfterOpening(t *testing.T) {
	setup := func(t *testing.T, aiOn bool, questions ...store.Question) (*aiEnv, *discordgo.InteractionCreate) {
		e := newAIEnv(t)
		ctx := context.Background()
		// Otro tipo de ticket, el que se abre, con formulario.
		ty, err := e.st.CreateTicketType(ctx, "g1", store.TicketTypeInput{
			Name: "Pagos", Questions: questions, AIEnabled: aiOn, AIInstructions: "Ayuda con cobros.",
		})
		if err != nil {
			t.Fatal(err)
		}
		// Lo último del canal nuevo es la bienvenida del bot.
		e.history(botSays("Hola, gracias por abrir un ticket."))
		return e, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
			ID: "int-9", AppID: "app", Token: "tok9", Type: discordgo.InteractionModalSubmit,
			GuildID: "g1", ChannelID: "panel", Member: member("luis"),
			Data: discordgo.ModalSubmitInteractionData{CustomID: formCustomID(ty.ID), Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q0", Value: "No me llegó el cobro"}}},
			}},
		}}
	}
	newChannelPosts := func(e *aiEnv) int {
		return len(e.f.find(http.MethodPost, "/channels/new-channel/messages"))
	}

	t.Run("con IA y formulario responde sin esperar al usuario", func(t *testing.T) {
		e, i := setup(t, true, store.Question{Label: "¿Qué ocurre?", Required: true})
		e.b.onInteraction(nil, i)
		eventually(t, "la respuesta inicial", func() bool { return e.llm.calls() == 1 && newChannelPosts(e) >= 2 })

		msgs := e.llm.request(0).Messages
		if !strings.Contains(msgs[0].Content, "Ayuda con cobros.") {
			t.Error("el prompt no lleva las instrucciones del tipo que se abrió")
		}
		answers := msgs[1]
		if answers.Role != "user" || !strings.Contains(answers.Content, "¿Qué ocurre?: No me llegó el cobro") {
			t.Errorf("las respuestas del formulario deben ir como mensaje de usuario: %+v", answers)
		}
		if last := msgs[len(msgs)-1]; last.Role != "user" || !strings.Contains(last.Content, "Aviso del sistema") {
			t.Errorf("la conversación debe acabar en un turno de usuario: %+v", last)
		}
		if tk, _, _ := e.st.TicketByChannel(context.Background(), "new-channel"); tk == nil || tk.AIReplies != 1 {
			t.Errorf("ticket nuevo = %+v", tk)
		}
	})

	t.Run("sin IA en el tipo no responde", func(t *testing.T) {
		e, i := setup(t, false, store.Question{Label: "¿Qué ocurre?", Required: true})
		e.b.onInteraction(nil, i)
		eventually(t, "el ticket", func() bool { return newChannelPosts(e) >= 1 })
		e.quiet()
		if e.llm.calls() != 0 {
			t.Errorf("%d consultas; el tipo no tiene IA", e.llm.calls())
		}
	})
}

func TestAssistantDoesNotSpeakFirstWithoutAForm(t *testing.T) {
	e := newAIEnv(t)
	ty, err := e.st.CreateTicketType(context.Background(), "g1", store.TicketTypeInput{Name: "Sin formulario", AIEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.b.onInteraction(nil, openButton(ty.ID, "luis"))
	eventually(t, "el ticket", func() bool { return len(e.f.find(http.MethodPost, "/channels/new-channel/messages")) >= 1 })
	e.quiet()
	if e.llm.calls() != 0 {
		t.Errorf("%d consultas: sin formulario espera a que el usuario escriba", e.llm.calls())
	}
}

func TestSplitMessage(t *testing.T) {
	if got := splitMessage("   ", 10); len(got) != 0 {
		t.Errorf("texto vacío = %q", got)
	}
	if got := splitMessage("corto", 10); len(got) != 1 || got[0] != "corto" {
		t.Errorf("texto corto = %q", got)
	}
	// Prefiere cortar por un salto de línea.
	got := splitMessage("aaaa bbbb\ncccc dddd", 12)
	if len(got) != 2 || got[0] != "aaaa bbbb" || got[1] != "cccc dddd" {
		t.Errorf("por salto de línea = %q", got)
	}
	// Si no hay espacios, corta duro.
	got = splitMessage(strings.Repeat("x", 25), 10)
	if len(got) != 3 || len([]rune(got[0])) != 10 || len([]rune(got[2])) != 5 {
		t.Errorf("sin espacios = %q", got)
	}
	// Los caracteres de varios bytes no se parten.
	for _, p := range splitMessage(strings.Repeat("ñ", 25), 10) {
		if !strings.HasPrefix(p, "ñ") || strings.ContainsRune(p, '�') {
			t.Errorf("parte rota: %q", p)
		}
	}
}

func TestNotTicketCache(t *testing.T) {
	e := newAIEnv(t)
	a := e.b.ai
	if a.knownNotTicket("c1") {
		t.Fatal("un canal desconocido no es «no-ticket»")
	}
	a.rememberNotTicket("c1")
	if !a.knownNotTicket("c1") {
		t.Error("debía recordarlo")
	}
	a.forgetNotTicket("c1")
	if a.knownNotTicket("c1") {
		t.Error("al crear un ticket en ese canal debía olvidarlo")
	}

	a.mu.Lock()
	a.notTicket["viejo"] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if a.knownNotTicket("viejo") {
		t.Error("una marca caducada no debe valer")
	}

	for i := range notTicketMaxSize + 5 {
		a.rememberNotTicket(fmt.Sprint("canal-", i))
	}
	a.mu.Lock()
	n := len(a.notTicket)
	a.mu.Unlock()
	if n > notTicketMaxSize {
		t.Errorf("la caché creció a %d entradas", n)
	}
}

// Un canal recién creado para un ticket no se queda marcado como «no-ticket».
func TestNewTicketChannelIsNotCachedAsNotTicket(t *testing.T) {
	e := newAIEnv(t)
	e.b.ai.rememberNotTicket("new-channel")
	ty, _ := e.st.CreateTicketType(context.Background(), "g1", store.TicketTypeInput{Name: "Otro"})
	e.b.onInteraction(nil, openButton(ty.ID, "luis"))
	eventually(t, "el ticket", func() bool { return len(e.f.find(http.MethodPost, "/channels/new-channel/messages")) >= 1 })
	if e.b.ai.knownNotTicket("new-channel") {
		t.Error("el canal del ticket nuevo seguía marcado como «no-ticket»")
	}
}

func TestAssistantIsOffWithoutEnableAI(t *testing.T) {
	b, _, st := newTestBot(t)
	seedTicket(t, st, false)
	// Sin EnableAI no hay asistente: ni entra en pánico ni hace nada.
	b.onMessageCreate(nil, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "g1", ChannelID: testChannel, Content: "hola", Author: &discordgo.User{ID: testOpener},
	}})
}
