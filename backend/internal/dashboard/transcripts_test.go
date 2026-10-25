package dashboard

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sedzz/askord/internal/auth"
	"github.com/sedzz/askord/internal/config"
	"github.com/sedzz/askord/internal/store"
	"github.com/sedzz/askord/internal/store/storetest"
)

// Cookie de sesión del backend (ver auth.sessionCookie).
const sessionCookie = "askord_session"

type env struct {
	t      *testing.T
	st     *store.Store
	router http.Handler
}

// newEnv monta la API real (auth.Require + handlers) sobre una base de datos limpia.
// Usuario "ana" administra el servidor g1; "luis", el g2. El bot está instalado en ambos.
func newEnv(t *testing.T) *env { return newEnvWith(t, config.Config{}) }

// newEnvWith es newEnv con una configuración concreta (por ejemplo, con clave de cifrado).
func newEnvWith(t *testing.T, cfg config.Config) *env {
	t.Helper()
	st := storetest.New(t)
	ctx := context.Background()

	for _, u := range []struct{ user, guild string }{{"ana", "g1"}, {"luis", "g2"}} {
		must(t, st.UpsertUser(ctx, store.User{ID: u.user, Username: u.user}))
		must(t, st.UpsertGuild(ctx, u.guild, u.guild, nil))
		must(t, st.ReplaceUserGuilds(ctx, u.user, []store.UserGuild{{ID: u.guild, Name: u.guild}}))
		sum := sha256.Sum256([]byte("token-" + u.user))
		must(t, st.CreateSession(ctx, sum[:], u.user, time.Now().Add(24*time.Hour)))
	}

	authH := auth.New(cfg, st)
	dashH := New(cfg, st, nil) // estos endpoints no usan el bot
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(authH.Require)
		r.Delete("/guilds/{guildID}/transcripts/{ticketID}", dashH.DeleteTranscript)
		r.Get("/guilds/{guildID}/ai", dashH.GetAI)
		r.Put("/guilds/{guildID}/ai", dashH.PutAI)
		r.Delete("/guilds/{guildID}/ai", dashH.DeleteAI)
		r.Post("/guilds/{guildID}/ai/test", dashH.TestAI)
	})
	return &env{t: t, st: st, router: r}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ticket crea un ticket del servidor con un mensaje; closed lo cierra.
func (e *env) ticket(guild string, number int, closed bool) int64 {
	e.t.Helper()
	ctx := context.Background()
	channel := guild + "-chan-" + string(rune('a'+number))
	must(e.t, e.st.CreateTicket(ctx, store.Ticket{
		GuildID: guild, Number: number, ChannelID: channel, OpenerID: "u1",
		Answers: []store.Answer{{Question: "¿Qué pasa?", Answer: "nada"}},
	}))
	t, _, err := e.st.TicketByChannel(ctx, channel)
	must(e.t, err)
	must(e.t, e.st.SaveTicketMessages(ctx, t.ID, []store.TicketMessage{
		{ID: channel + "-m", AuthorID: "u1", AuthorName: "u1", Content: "hola", CreatedAt: time.Now()},
	}))
	if closed {
		if ok, err := e.st.CloseTicket(ctx, channel, "ana"); err != nil || !ok {
			e.t.Fatalf("cerrar ticket: ok=%v err=%v", ok, err)
		}
	}
	return t.ID
}

// delete lanza DELETE /api/guilds/{guild}/transcripts/{ticket} como ese usuario ("" = sin sesión).
func (e *env) delete(user, guild, ticket string) int {
	req := httptest.NewRequest(http.MethodDelete, "/api/guilds/"+guild+"/transcripts/"+ticket, nil)
	if user != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token-" + user})
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w.Code
}

// intact indica si el ticket conserva sus mensajes y respuestas.
func (e *env) intact(guild string, id int64) bool {
	e.t.Helper()
	list, err := e.st.LoadTranscripts(context.Background(), guild, store.TranscriptFilter{}, id)
	must(e.t, err)
	return len(list) == 1 && len(list[0].Conversation) == 1 && len(list[0].Answers) == 1
}

func TestDeleteTranscript(t *testing.T) {
	e := newEnv(t)
	closed := e.ticket("g1", 1, true)

	if got := e.delete("ana", "g1", strconv.FormatInt(closed, 10)); got != http.StatusNoContent {
		t.Fatalf("borrado correcto = %d, quería 204", got)
	}
	list, err := e.st.LoadTranscripts(context.Background(), "g1", store.TranscriptFilter{}, closed)
	must(t, err)
	if len(list) != 0 {
		t.Error("la transcripción sigue disponible tras borrarla")
	}

	// Repetirlo no falla.
	if got := e.delete("ana", "g1", strconv.FormatInt(closed, 10)); got != http.StatusNoContent {
		t.Errorf("repetir el borrado = %d, quería 204", got)
	}
}

func TestDeleteTranscriptRejected(t *testing.T) {
	e := newEnv(t)
	closed := e.ticket("g1", 1, true)
	open := e.ticket("g1", 2, false)

	cases := []struct {
		name                string
		user, guild, ticket string
		want                int
	}{
		{"sin sesión", "", "g1", strconv.FormatInt(closed, 10), http.StatusUnauthorized},
		{"sesión desconocida", "nadie", "g1", strconv.FormatInt(closed, 10), http.StatusUnauthorized},
		{"servidor de otro usuario", "luis", "g1", strconv.FormatInt(closed, 10), http.StatusNotFound},
		{"ticket de otro servidor", "luis", "g2", strconv.FormatInt(closed, 10), http.StatusNotFound},
		{"ticket abierto", "ana", "g1", strconv.FormatInt(open, 10), http.StatusNotFound},
		{"ticket inexistente", "ana", "g1", "99999", http.StatusNotFound},
		{"id no numérico", "ana", "g1", "abc", http.StatusNotFound},
		{"id cero", "ana", "g1", "0", http.StatusNotFound},
	}
	for _, c := range cases {
		if got := e.delete(c.user, c.guild, c.ticket); got != c.want {
			t.Errorf("%s: %d, quería %d", c.name, got, c.want)
		}
	}

	// Ninguna petición rechazada tocó el contenido.
	if !e.intact("g1", closed) || !e.intact("g1", open) {
		t.Error("una petición rechazada borró contenido")
	}
}
