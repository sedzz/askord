package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sedzz/askord/internal/config"
	"github.com/sedzz/askord/internal/store"
	"github.com/sedzz/askord/internal/store/storetest"
)

// request pasa una petición por Require y devuelve la respuesta y el usuario que vio el handler.
func request(t *testing.T, h *Handler, cookie string) (*httptest.ResponseRecorder, store.User) {
	t.Helper()
	var seen store.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	w := httptest.NewRecorder()
	h.Require(next).ServeHTTP(w, req)
	return w, seen
}

func TestRequire(t *testing.T) {
	st := storetest.New(t)
	ctx := context.Background()
	h := New(config.Config{}, st)

	if err := st.UpsertUser(ctx, store.User{ID: "ana", Username: "Ana"}); err != nil {
		t.Fatal(err)
	}
	session := func(token string, expires time.Time) {
		t.Helper()
		if err := st.CreateSession(ctx, hash(token), "ana", expires); err != nil {
			t.Fatal(err)
		}
	}
	session("vigente", time.Now().Add(sessionTTL))
	session("caducada", time.Now().Add(-time.Minute))

	t.Run("sin cookie", func(t *testing.T) {
		if w, _ := request(t, h, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("código %d, quería 401", w.Code)
		}
	})
	t.Run("sesión desconocida", func(t *testing.T) {
		if w, _ := request(t, h, "inventada"); w.Code != http.StatusUnauthorized {
			t.Errorf("código %d, quería 401", w.Code)
		}
	})
	t.Run("sesión caducada", func(t *testing.T) {
		if w, _ := request(t, h, "caducada"); w.Code != http.StatusUnauthorized {
			t.Errorf("código %d, quería 401", w.Code)
		}
	})
	t.Run("sesión vigente", func(t *testing.T) {
		w, user := request(t, h, "vigente")
		if w.Code != http.StatusOK || user.ID != "ana" {
			t.Errorf("código %d, usuario %q; quería 200 y ana", w.Code, user.ID)
		}
		// Le queda toda la vida útil: no se renueva ni se escribe otra cookie.
		if c := w.Result().Cookies(); len(c) != 0 {
			t.Errorf("no debía renovarse la cookie: %v", c)
		}
	})
}

func TestRequireRenewsSession(t *testing.T) {
	st := storetest.New(t)
	ctx := context.Background()
	h := New(config.Config{}, st)

	if err := st.UpsertUser(ctx, store.User{ID: "ana", Username: "Ana"}); err != nil {
		t.Fatal(err)
	}
	// Le queda menos de la mitad de su vida: se alarga.
	if err := st.CreateSession(ctx, hash("por-caducar"), "ana", time.Now().Add(sessionTTL/4)); err != nil {
		t.Fatal(err)
	}

	w, _ := request(t, h, "por-caducar")
	if w.Code != http.StatusOK {
		t.Fatalf("código %d, quería 200", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie || cookies[0].Value != "por-caducar" {
		t.Fatalf("cookies = %v, quería la de sesión renovada", cookies)
	}
	_, expires, err := st.UserBySession(ctx, hash("por-caducar"))
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(expires) < sessionTTL-time.Hour {
		t.Errorf("la sesión no se alargó: caduca en %v", time.Until(expires))
	}
}

func TestRequireIgnoresRevokedSession(t *testing.T) {
	st := storetest.New(t)
	ctx := context.Background()
	h := New(config.Config{}, st)

	if err := st.UpsertUser(ctx, store.User{ID: "ana", Username: "Ana"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, hash("cerrada"), "ana", time.Now().Add(sessionTTL)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSession(ctx, hash("cerrada")); err != nil { // lo que hace Logout
		t.Fatal(err)
	}
	if w, _ := request(t, h, "cerrada"); w.Code != http.StatusUnauthorized {
		t.Errorf("código %d, quería 401 tras cerrar sesión", w.Code)
	}
}
