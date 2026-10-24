package bot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
	"github.com/sedzz/askord/internal/store/storetest"
)

// call es una petición que el bot hizo a Discord.
type call struct {
	Method string
	Path   string // sin el prefijo /api/vN
	Body   string
}

// fakeDiscord hace de API REST de Discord: responde lo mínimo para que el bot funcione y
// apunta cada petición, para que los tests comprueben qué hizo.
type fakeDiscord struct {
	t        *testing.T
	mu       sync.Mutex
	calls    []call
	messages string // JSON de GET /channels/{id}/messages
	missing  map[string]bool
	// forbidChannelCreate hace que crear canales falle con 403, como sin permisos.
	forbidChannelCreate bool
}

// newTestBot crea un Bot con una base de datos limpia cuya sesión de Discord habla con fakeDiscord.
func newTestBot(t *testing.T) (*Bot, *fakeDiscord, *store.Store) {
	t.Helper()
	st := storetest.New(t)
	f := &fakeDiscord{t: t, messages: "[]", missing: map[string]bool{}}

	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	// Todas las peticiones a discord.com van al servidor falso.
	session.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})}
	session.State.User = &discordgo.User{ID: "bot"}
	return &Bot{session: session, store: st}, f, st
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (f *fakeDiscord) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/api/v"+discordgo.APIVersion)

	f.mu.Lock()
	f.calls = append(f.calls, call{r.Method, path, string(body)})
	missing := f.missing[path]
	messages := f.messages
	forbid := f.forbidChannelCreate
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	isChannelCreate := r.Method == http.MethodPost && strings.HasPrefix(path, "/guilds/") && strings.HasSuffix(path, "/channels")
	switch {
	case forbid && isChannelCreate:
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Permissions","code":50013}`))
	case missing:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Unknown Channel","code":10003}`))
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/messages"):
		_, _ = w.Write([]byte(messages))
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/roles"):
		_, _ = w.Write([]byte(`[]`))
	case isChannelCreate:
		_, _ = w.Write([]byte(`{"id":"new-channel","type":0}`))
	case r.Method == http.MethodDelete || strings.HasSuffix(path, "/callback"):
		w.WriteHeader(http.StatusNoContent)
	default:
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}
}

// find devuelve las peticiones con ese método cuya ruta empieza o termina como se indica.
func (f *fakeDiscord) find(method, pathPart string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method == method && strings.Contains(c.Path, pathPart) {
			out = append(out, c)
		}
	}
	return out
}

// replies devuelve los textos que el bot mostró al usuario: la respuesta directa a la
// interacción y las ediciones posteriores del mensaje.
func (f *fakeDiscord) replies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		switch {
		case c.Method == http.MethodPost && strings.HasSuffix(c.Path, "/callback"):
			var v struct {
				Data struct {
					Content string `json:"content"`
				} `json:"data"`
			}
			_ = json.Unmarshal([]byte(c.Body), &v)
			if v.Data.Content != "" {
				out = append(out, v.Data.Content)
			}
		case c.Method == http.MethodPatch && strings.HasSuffix(c.Path, "/messages/@original"):
			var v struct {
				Content string `json:"content"`
			}
			_ = json.Unmarshal([]byte(c.Body), &v)
			if v.Content != "" {
				out = append(out, v.Content)
			}
		}
	}
	return out
}

// callbackType devuelve el tipo de la primera respuesta a la interacción (4 mensaje,
// 5 diferida, 9 formulario…), o 0 si no hubo.
func (f *fakeDiscord) callbackType() int {
	for _, c := range f.find(http.MethodPost, "/callback") {
		var v struct {
			Type int `json:"type"`
		}
		_ = json.Unmarshal([]byte(c.Body), &v)
		return v.Type
	}
	return 0
}

// click construye la interacción de quien pulsa un botón en un canal.
func click(customID, channelID string, member *discordgo.Member) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID: "int-1", AppID: "app", Token: "tok", Type: discordgo.InteractionMessageComponent,
		GuildID: "g1", ChannelID: channelID, Member: member,
		Data: discordgo.MessageComponentInteractionData{CustomID: customID},
	}}
}

func member(id string, roles ...string) *discordgo.Member {
	return &discordgo.Member{User: &discordgo.User{ID: id}, Roles: roles}
}

// reset olvida las peticiones hechas hasta ahora.
func (f *fakeDiscord) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}
