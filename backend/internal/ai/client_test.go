package ai

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateBaseURL(t *testing.T) {
	strict, local := New(false), New(true)
	cases := []struct {
		name    string
		client  *Client
		raw     string
		want    string // vacío: debe fallar
		wantErr string
	}{
		{"https normal", strict, "https://api.openai.com/v1", "https://api.openai.com/v1", ""},
		{"quita la barra final", strict, "  https://api.openai.com/v1/ ", "https://api.openai.com/v1", ""},
		{"con puerto", strict, "https://llm.example.com:8443/v1", "https://llm.example.com:8443/v1", ""},
		{"http no se admite", strict, "http://api.example.com/v1", "", "https://"},
		{"sin esquema", strict, "api.openai.com/v1", "", ""},
		{"vacía", strict, "", "", ""},
		{"otro esquema con host", strict, "ftp://host/v1", "", "https://"},
		{"otro esquema", strict, "file:///etc/passwd", "", ""},
		{"con usuario", strict, "https://user:pass@api.example.com/v1", "", "usuario"},
		{"con parámetros", strict, "https://api.example.com/v1?key=1", "", "parámetros"},
		{"loopback", strict, "https://127.0.0.1/v1", "", "local o privada"},
		{"red privada", strict, "https://10.0.0.5/v1", "", "local o privada"},
		{"metadatos de nube", strict, "https://169.254.169.254/latest", "", "local o privada"},
		{"ipv6 loopback", strict, "https://[::1]/v1", "", "local o privada"},
		{"en desarrollo http local", local, "http://localhost:11434/v1", "http://localhost:11434/v1", ""},
		{"en desarrollo ip privada", local, "https://10.0.0.5/v1", "https://10.0.0.5/v1", ""},
	}
	for _, c := range cases {
		got, err := c.client.ValidateBaseURL(c.raw)
		if c.want == "" {
			if err == nil {
				t.Errorf("%s: %q debía rechazarse, dio %q", c.name, c.raw, got)
			} else if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: error %q, debía contener %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; quería %q", c.name, got, err, c.want)
		}
	}
}

func TestIsInternal(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "::1", "fe80::1", "fd00::1", "224.0.0.1"} {
		if !isInternal(parseIP(t, ip)) {
			t.Errorf("%s debía considerarse interna", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "104.18.0.1", "2606:4700::1111"} {
		if isInternal(parseIP(t, ip)) {
			t.Errorf("%s debía considerarse pública", ip)
		}
	}
}

// llm monta un servidor compatible con OpenAI que ejecuta handle y apunta lo que recibe.
type llm struct {
	srv  *httptest.Server
	auth string
	body map[string]any
	path string
}

func newLLM(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *llm {
	t.Helper()
	l := &llm{}
	l.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.auth, l.path = r.Header.Get("Authorization"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&l.body)
		handle(w, r)
	}))
	t.Cleanup(l.srv.Close)
	return l
}

func okResponse(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
}

func TestTestSuccess(t *testing.T) {
	l := newLLM(t, okResponse)
	err := New(true).Test(context.Background(), Config{BaseURL: l.srv.URL + "/v1", Model: "mi-modelo", APIKey: "sk-123"})
	if err != nil {
		t.Fatal(err)
	}
	if l.path != "/v1/chat/completions" {
		t.Errorf("ruta %q", l.path)
	}
	if l.auth != "Bearer sk-123" {
		t.Errorf("Authorization = %q", l.auth)
	}
	if l.body["model"] != "mi-modelo" {
		t.Errorf("modelo enviado = %v", l.body["model"])
	}
}

func TestTestErrors(t *testing.T) {
	reply := func(status int, body string) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
	}
	cases := []struct {
		name   string
		handle func(http.ResponseWriter, *http.Request)
		want   string
	}{
		{"key inválida", reply(401, `{"error":{"message":"Incorrect API key provided: sk-secreta"}}`), "API key no es válida"},
		{"sin permisos", reply(403, `{}`), "API key no es válida"},
		{"modelo inexistente", reply(404, `{}`), "endpoint o el modelo"},
		{"límite de peticiones", reply(429, `{}`), "limitó"},
		{"error con mensaje", reply(400, `{"error":{"message":"max_tokens demasiado bajo"}}`), "max_tokens demasiado bajo"},
		{"error sin mensaje", reply(500, `boom`), "(500)"},
		{"no es openai", reply(200, `{"hola":"mundo"}`), "compatible con OpenAI"},
		{"no es json", reply(200, `<html>`), "compatible con OpenAI"},
		{"sin opciones", reply(200, `{"choices":[]}`), "compatible con OpenAI"},
		{"redirección", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
		}, "redirige"},
	}
	for _, c := range cases {
		l := newLLM(t, c.handle)
		err := New(true).Test(context.Background(), Config{BaseURL: l.srv.URL, Model: "m", APIKey: "sk-secreta"})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, debía contener %q", c.name, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "sk-secreta") {
			t.Errorf("%s: el error revela la API key: %v", c.name, err)
		}
	}
}

func TestTestHidesKeyEchoedByProvider(t *testing.T) {
	l := newLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request for key sk-secreta"}}`))
	})
	err := New(true).Test(context.Background(), Config{BaseURL: l.srv.URL, Model: "m", APIKey: "sk-secreta"})
	if err == nil || strings.Contains(err.Error(), "sk-secreta") || !strings.Contains(err.Error(), "***") {
		t.Errorf("error = %v, la key debía ocultarse", err)
	}
}

func TestTestTimeout(t *testing.T) {
	l := newLLM(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := New(true).Test(ctx, Config{BaseURL: l.srv.URL, Model: "m", APIKey: "k"})
	if err == nil || !strings.Contains(err.Error(), "tardó demasiado") {
		t.Errorf("error = %v", err)
	}
}

// El cliente estricto no debe conectar a una dirección interna aunque la URL parezca normal
// (un nombre que resuelve a 127.0.0.1). Se prueba con localhost por https.
func TestTestBlocksInternalAddresses(t *testing.T) {
	hit := false
	l := newLLM(t, func(w http.ResponseWriter, r *http.Request) { hit = true })
	port := l.srv.URL[strings.LastIndex(l.srv.URL, ":"):]

	err := New(false).Test(context.Background(), Config{BaseURL: "https://localhost" + port, Model: "m", APIKey: "k"})
	if err == nil || !strings.Contains(err.Error(), "local o privada") {
		t.Errorf("error = %v, debía bloquear la conexión", err)
	}
	if hit {
		t.Error("el servidor interno recibió la petición")
	}
}

func parseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("IP inválida %q", s)
	}
	return ip
}
