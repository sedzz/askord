package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/sedzz/askord/internal/config"
	"github.com/sedzz/askord/internal/store"
)

const (
	sessionCookie = "askord_session"
	stateCookie   = "askord_oauth_state"
	sessionTTL    = 7 * 24 * time.Hour
)

var discordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

type Handler struct {
	cfg   config.Config
	store *store.Store
	oauth *oauth2.Config
}

func New(cfg config.Config, s *store.Store) *Handler {
	return &Handler{
		cfg:   cfg,
		store: s,
		oauth: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURL,
			Scopes:       []string{"identify", "guilds"},
			Endpoint:     discordEndpoint,
		},
	}
}

// Login redirige a Discord con un state anti-CSRF.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, stateCookie, state, 10*time.Minute)
	http.Redirect(w, r, h.oauth.AuthCodeURL(state), http.StatusFound)
}

// Callback valida el state, obtiene el usuario de Discord y abre sesion.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(stateCookie)
	got := r.URL.Query().Get("state")
	if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(got)) != 1 {
		http.Error(w, "state invalido", http.StatusBadRequest)
		return
	}
	h.clearCookie(w, stateCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.cfg.FrontendURL+"/login?error=denied", http.StatusFound)
		return
	}

	tok, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("intercambio de codigo", "err", err)
		http.Error(w, "no se pudo iniciar sesion", http.StatusBadGateway)
		return
	}

	client := h.oauth.Client(r.Context(), tok)
	user, err := fetchDiscordUser(r.Context(), client)
	if err != nil {
		slog.Error("obtener usuario de discord", "err", err)
		http.Error(w, "no se pudo iniciar sesion", http.StatusBadGateway)
		return
	}
	if err := h.store.UpsertUser(r.Context(), user); err != nil {
		slog.Error("guardar usuario", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}

	// La lista de servidores se guarda en cada login; si falla, el login sigue.
	if guilds, err := fetchManageableGuilds(r.Context(), client); err != nil {
		slog.Error("obtener servidores del usuario", "err", err)
	} else if err := h.store.ReplaceUserGuilds(r.Context(), user.ID, guilds); err != nil {
		slog.Error("guardar servidores del usuario", "err", err)
	}

	token, err := randomToken()
	if err != nil {
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if err := h.store.CreateSession(r.Context(), hash(token), user.ID, time.Now().Add(sessionTTL)); err != nil {
		slog.Error("crear sesion", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, sessionCookie, token, sessionTTL)
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = h.store.DeleteSession(r.Context(), hash(c.Value))
	}
	h.clearCookie(w, sessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

type ctxKey struct{}

// Require carga el usuario de la sesion o responde 401.
func (h *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Error(w, "no autenticado", http.StatusUnauthorized)
			return
		}
		u, expires, err := h.store.UserBySession(r.Context(), hash(c.Value))
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "no autenticado", http.StatusUnauthorized)
			return
		}
		if err != nil {
			slog.Error("cargar sesion", "err", err)
			http.Error(w, "error interno", http.StatusInternalServerError)
			return
		}

		// Sesión deslizante: se renueva solo cuando queda menos de la mitad,
		// para no escribir en la base de datos en cada petición.
		if time.Until(expires) < sessionTTL/2 {
			if err := h.store.ExtendSession(r.Context(), hash(c.Value), time.Now().Add(sessionTTL)); err != nil {
				slog.Error("extender sesion", "err", err)
			} else {
				h.setCookie(w, sessionCookie, c.Value, sessionTTL)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func UserFrom(ctx context.Context) store.User {
	u, _ := ctx.Value(ctxKey{}).(store.User)
	return u
}

// Me devuelve el usuario autenticado.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(UserFrom(r.Context()))
}

func fetchDiscordUser(ctx context.Context, client *http.Client) (store.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me", nil)
	if err != nil {
		return store.User{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return store.User{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return store.User{}, fmt.Errorf("discord respondio %d", resp.StatusCode)
	}

	var body struct {
		ID       string  `json:"id"`
		Username string  `json:"username"`
		Avatar   *string `json:"avatar"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return store.User{}, err
	}
	return store.User{ID: body.ID, Username: body.Username, Avatar: body.Avatar}, nil
}

const (
	permAdministrator = 1 << 3
	permManageGuild   = 1 << 5
)

// fetchManageableGuilds devuelve los servidores donde el usuario es dueño o
// tiene Administrador / Gestionar servidor.
func fetchManageableGuilds(ctx context.Context, client *http.Client) ([]store.UserGuild, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord respondio %d", resp.StatusCode)
	}

	var body []struct {
		ID          string      `json:"id"`
		Name        string      `json:"name"`
		Icon        *string     `json:"icon"`
		Owner       bool        `json:"owner"`
		Permissions json.Number `json:"permissions"` // string o número según la versión de la API
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	guilds := []store.UserGuild{}
	for _, g := range body {
		perms, _ := strconv.ParseUint(g.Permissions.String(), 10, 64)
		if g.Owner || perms&(permAdministrator|permManageGuild) != 0 {
			guilds = append(guilds, store.UserGuild{ID: g.ID, Name: g.Name, Icon: g.Icon, Owner: g.Owner})
		}
	}
	return guilds, nil
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
