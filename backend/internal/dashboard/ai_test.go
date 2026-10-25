package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sedzz/askord/internal/config"
)

// aiEnv monta el dashboard con clave de cifrado y permiso para URLs locales (el LLM falso
// escucha en 127.0.0.1).
func aiEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, config.Config{
		AIEncryptionKey:    bytes.Repeat([]byte{3}, 32),
		AIAllowPrivateURLs: true,
	})
}

// fakeLLM acepta solo la API key indicada y apunta con qué key se le llamó.
func fakeLLM(t *testing.T, validKey string) (url string, keysSeen *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		seen = append(seen, key)
		if key != validKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &seen
}

// call lanza una petición JSON a la API como ese usuario y devuelve código y cuerpo.
func (e *env) call(method, user, path string, body any) (int, string) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api"+path, rd)
	if user != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token-" + user})
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

type aiJSON struct {
	Available  bool   `json:"available"`
	BaseURL    string `json:"baseUrl"`
	Model      string `json:"model"`
	Enabled    bool   `json:"enabled"`
	HasKey     bool   `json:"hasKey"`
	Guidelines string `json:"guidelines"`
}

func decodeAI(t *testing.T, body string) aiJSON {
	t.Helper()
	var v aiJSON
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("respuesta no es JSON: %q", body)
	}
	return v
}

func TestAIUnavailableWithoutEncryptionKey(t *testing.T) {
	e := newEnv(t) // sin AIEncryptionKey

	code, body := e.call(http.MethodGet, "ana", "/guilds/g1/ai", nil)
	if code != http.StatusOK || decodeAI(t, body).Available {
		t.Errorf("GET: %d %s, quería 200 con available=false", code, body)
	}
	put := map[string]any{"baseUrl": "https://api.openai.com/v1", "model": "gpt", "apiKey": "sk-1"}
	if code, _ := e.call(http.MethodPut, "ana", "/guilds/g1/ai", put); code != http.StatusServiceUnavailable {
		t.Errorf("PUT sin clave de cifrado = %d, quería 503", code)
	}
	if code, _ := e.call(http.MethodPost, "ana", "/guilds/g1/ai/test", put); code != http.StatusServiceUnavailable {
		t.Errorf("test sin clave de cifrado = %d, quería 503", code)
	}
}

func TestAISaveAndRead(t *testing.T) {
	e := aiEnv(t)
	put := map[string]any{"baseUrl": "http://127.0.0.1:1/v1/", "model": " mi-modelo ", "apiKey": "sk-muy-secreta"}

	code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai", put)
	if code != http.StatusOK {
		t.Fatalf("PUT = %d %s", code, body)
	}
	got := decodeAI(t, body)
	if !got.Available || !got.HasKey || got.Enabled || got.Model != "mi-modelo" || got.BaseURL != "http://127.0.0.1:1/v1" {
		t.Errorf("respuesta = %+v", got)
	}

	// La API key no sale nunca, ni al guardar ni al leer.
	_, read := e.call(http.MethodGet, "ana", "/guilds/g1/ai", nil)
	for _, b := range []string{body, read} {
		if strings.Contains(b, "sk-muy-secreta") {
			t.Errorf("la respuesta contiene la API key: %s", b)
		}
	}

	// En la base de datos va cifrada, y solo se abre con la clave y el servidor correctos.
	stored, err := e.st.GetAISettings(context.Background(), "g1")
	if err != nil || stored == nil {
		t.Fatalf("no se guardó: %v", err)
	}
	if bytes.Contains(stored.KeyEnc, []byte("sk-muy-secreta")) {
		t.Error("la API key está en claro en la base de datos")
	}
}

func TestAIKeepsKeyWhenOmitted(t *testing.T) {
	e := aiEnv(t)
	url, seen := fakeLLM(t, "sk-guardada")
	first := map[string]any{"baseUrl": url, "model": "m1", "apiKey": "sk-guardada"}
	if code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai", first); code != http.StatusOK {
		t.Fatalf("PUT = %d %s", code, body)
	}

	// Cambiar el modelo sin repetir la key la conserva.
	second := map[string]any{"baseUrl": url, "model": "m2"}
	code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai", second)
	if got := decodeAI(t, body); code != http.StatusOK || got.Model != "m2" || !got.HasKey {
		t.Fatalf("PUT sin key = %d %s", code, body)
	}

	// Y la prueba sin key usa la guardada: el LLM falso la recibe en claro y la acepta.
	code, body = e.call(http.MethodPost, "ana", "/guilds/g1/ai/test", map[string]any{"baseUrl": url, "model": "m2"})
	if code != http.StatusOK || !strings.Contains(body, `"ok":true`) {
		t.Errorf("test = %d %s", code, body)
	}
	if len(*seen) != 1 || (*seen)[0] != "sk-guardada" {
		t.Errorf("el LLM recibió las keys %v, quería [sk-guardada]", *seen)
	}
}

func TestAIFirstSaveNeedsKey(t *testing.T) {
	e := aiEnv(t)
	put := map[string]any{"baseUrl": "https://api.openai.com/v1", "model": "gpt"}
	if code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai", put); code != http.StatusBadRequest || !strings.Contains(body, "API key") {
		t.Errorf("PUT sin key = %d %s", code, body)
	}
	if code, _ := e.call(http.MethodPost, "ana", "/guilds/g1/ai/test", put); code != http.StatusBadRequest {
		t.Errorf("test sin key = %d, quería 400", code)
	}
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s != nil {
		t.Error("se guardó algo sin API key")
	}
}

func TestAITestReportsProblems(t *testing.T) {
	e := aiEnv(t)
	url, _ := fakeLLM(t, "sk-buena")

	code, body := e.call(http.MethodPost, "ana", "/guilds/g1/ai/test", map[string]any{"baseUrl": url, "model": "m", "apiKey": "sk-buena"})
	if code != http.StatusOK || !strings.Contains(body, `"ok":true`) {
		t.Errorf("key buena: %d %s", code, body)
	}
	code, body = e.call(http.MethodPost, "ana", "/guilds/g1/ai/test", map[string]any{"baseUrl": url, "model": "m", "apiKey": "sk-mala"})
	if code != http.StatusOK || !strings.Contains(body, `"ok":false`) || !strings.Contains(body, "API key no es válida") {
		t.Errorf("key mala: %d %s", code, body)
	}
	if strings.Contains(body, "sk-mala") {
		t.Error("el resultado revela la API key")
	}
	// La prueba no guarda nada.
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s != nil {
		t.Error("probar la conexión no debía guardarla")
	}
}

func TestAIValidation(t *testing.T) {
	e := aiEnv(t)
	cases := map[string]map[string]any{
		"url vacía":           {"baseUrl": "", "model": "m", "apiKey": "k"},
		"url inválida":        {"baseUrl": "no es una url", "model": "m", "apiKey": "k"},
		"esquema ftp":         {"baseUrl": "ftp://host/v1", "model": "m", "apiKey": "k"},
		"sin modelo":          {"baseUrl": "https://api.openai.com/v1", "model": "  ", "apiKey": "k"},
		"modelo largo":        {"baseUrl": "https://api.openai.com/v1", "model": strings.Repeat("m", 101), "apiKey": "k"},
		"key demasiado larga": {"baseUrl": "https://api.openai.com/v1", "model": "m", "apiKey": strings.Repeat("k", 501)},
	}
	for name, body := range cases {
		if code, _ := e.call(http.MethodPut, "ana", "/guilds/g1/ai", body); code != http.StatusBadRequest {
			t.Errorf("%s: PUT = %d, quería 400", name, code)
		}
	}

	// Sin permiso para URLs locales (producción), una privada se rechaza.
	prod := newEnvWith(t, config.Config{AIEncryptionKey: bytes.Repeat([]byte{3}, 32)})
	for _, u := range []string{"https://127.0.0.1/v1", "https://10.0.0.1/v1", "https://169.254.169.254/v1", "http://api.example.com/v1"} {
		body := map[string]any{"baseUrl": u, "model": "m", "apiKey": "k"}
		if code, _ := prod.call(http.MethodPut, "ana", "/guilds/g1/ai", body); code != http.StatusBadRequest {
			t.Errorf("%s: PUT = %d, quería 400", u, code)
		}
	}
}

func TestAIAccessControl(t *testing.T) {
	e := aiEnv(t)
	put := map[string]any{"baseUrl": "https://api.openai.com/v1", "model": "gpt", "apiKey": "sk-1"}
	if code, _ := e.call(http.MethodPut, "ana", "/guilds/g1/ai", put); code != http.StatusOK {
		t.Fatalf("PUT = %d", code)
	}

	for _, c := range []struct {
		name, user, method, path string
		want                     int
	}{
		{"leer sin sesión", "", http.MethodGet, "/guilds/g1/ai", http.StatusUnauthorized},
		{"leer de otro servidor", "luis", http.MethodGet, "/guilds/g1/ai", http.StatusNotFound},
		{"escribir en otro servidor", "luis", http.MethodPut, "/guilds/g1/ai", http.StatusNotFound},
		{"probar en otro servidor", "luis", http.MethodPost, "/guilds/g1/ai/test", http.StatusNotFound},
		{"borrar en otro servidor", "luis", http.MethodDelete, "/guilds/g1/ai", http.StatusNotFound},
	} {
		if code, _ := e.call(c.method, c.user, c.path, put); code != c.want {
			t.Errorf("%s: %d, quería %d", c.name, code, c.want)
		}
	}
	// Nada de lo anterior tocó la conexión de ana.
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s == nil || len(s.KeyEnc) == 0 {
		t.Error("la conexión se perdió o se alteró")
	}
	// Luis tiene la suya aparte.
	if _, body := e.call(http.MethodGet, "luis", "/guilds/g2/ai", nil); decodeAI(t, body).HasKey {
		t.Error("luis ve la key de otro servidor")
	}
}

func TestAIDelete(t *testing.T) {
	e := aiEnv(t)
	put := map[string]any{"baseUrl": "https://api.openai.com/v1", "model": "gpt", "apiKey": "sk-1"}
	e.call(http.MethodPut, "ana", "/guilds/g1/ai", put)

	if code, _ := e.call(http.MethodDelete, "ana", "/guilds/g1/ai", nil); code != http.StatusNoContent {
		t.Fatalf("DELETE = %d", code)
	}
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s != nil {
		t.Error("la conexión sigue guardada")
	}
	// Repetirlo no falla, y leer devuelve una conexión vacía.
	if code, _ := e.call(http.MethodDelete, "ana", "/guilds/g1/ai", nil); code != http.StatusNoContent {
		t.Errorf("repetir DELETE = %d", code)
	}
	if _, body := e.call(http.MethodGet, "ana", "/guilds/g1/ai", nil); decodeAI(t, body).HasKey {
		t.Error("tras borrar aún hay key")
	}
}

// connect guarda una conexión para ana en g1.
func (e *env) connect() {
	e.t.Helper()
	put := map[string]any{"baseUrl": "https://api.openai.com/v1", "model": "gpt", "apiKey": "sk-1"}
	if code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai", put); code != http.StatusOK {
		e.t.Fatalf("PUT = %d %s", code, body)
	}
}

func TestAIBehavior(t *testing.T) {
	e := aiEnv(t)
	e.connect()

	// Los saltos de línea de Windows, los espacios al final y los caracteres de control se limpian.
	raw := "  # Pautas\r\n\r\n\r\n\r\nSé amable.   \r\n- No prometas reembolsos\x00  \n\n"
	code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai/behavior", map[string]any{"enabled": true, "guidelines": raw})
	if code != http.StatusOK {
		t.Fatalf("PUT = %d %s", code, body)
	}
	got := decodeAI(t, body)
	want := "# Pautas\n\nSé amable.\n- No prometas reembolsos"
	if !got.Enabled || got.Guidelines != want || !got.HasKey {
		t.Errorf("respuesta = %+v\nquería pautas %q", got, want)
	}
	if _, read := e.call(http.MethodGet, "ana", "/guilds/g1/ai", nil); decodeAI(t, read).Guidelines != want {
		t.Errorf("GET no devuelve las pautas guardadas: %s", read)
	}

	// Cambiar la conexión después no toca el interruptor ni las pautas.
	put := map[string]any{"baseUrl": "https://openrouter.ai/api/v1", "model": "otro"}
	code, body = e.call(http.MethodPut, "ana", "/guilds/g1/ai", put)
	if got := decodeAI(t, body); code != http.StatusOK || !got.Enabled || got.Guidelines != want || got.Model != "otro" {
		t.Errorf("PUT conexión = %d %+v; debía conservar interruptor y pautas", code, got)
	}

	// Y se pueden apagar y vaciar.
	_, body = e.call(http.MethodPut, "ana", "/guilds/g1/ai/behavior", map[string]any{"enabled": false, "guidelines": ""})
	if got := decodeAI(t, body); got.Enabled || got.Guidelines != "" {
		t.Errorf("tras apagar = %+v", got)
	}
}

func TestAIBehaviorNeedsConnection(t *testing.T) {
	e := aiEnv(t)
	code, body := e.call(http.MethodPut, "ana", "/guilds/g1/ai/behavior", map[string]any{"enabled": true, "guidelines": "hola"})
	if code != http.StatusBadRequest || !strings.Contains(body, "conexión") {
		t.Errorf("sin conexión: %d %s", code, body)
	}
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s != nil {
		t.Error("se creó una fila sin conexión")
	}

	// Sin cifrado configurado, ni siquiera se intenta.
	if code, _ := newEnv(t).call(http.MethodPut, "ana", "/guilds/g1/ai/behavior", map[string]any{}); code != http.StatusServiceUnavailable {
		t.Errorf("sin cifrado = %d, quería 503", code)
	}
}

func TestAIBehaviorLimits(t *testing.T) {
	e := aiEnv(t)
	e.connect()
	put := func(guidelines string) int {
		code, _ := e.call(http.MethodPut, "ana", "/guilds/g1/ai/behavior", map[string]any{"guidelines": guidelines})
		return code
	}
	if code := put(strings.Repeat("a", 4000)); code != http.StatusOK {
		t.Errorf("4000 caracteres = %d, quería 200", code)
	}
	if code := put(strings.Repeat("ñ", 4000)); code != http.StatusOK {
		t.Errorf("4000 ñ (8000 bytes) = %d, el límite es en caracteres y no en bytes", code)
	}
	if code := put(strings.Repeat("a", 4001)); code != http.StatusBadRequest {
		t.Errorf("4001 caracteres = %d, quería 400", code)
	}
	// Los espacios que se van a limpiar no cuentan para el límite.
	if code := put(strings.Repeat("a", 4000) + strings.Repeat(" \n", 50)); code != http.StatusOK {
		t.Errorf("4000 caracteres más espacios finales = %d, quería 200", code)
	}
}

func TestAIBehaviorAccessControl(t *testing.T) {
	e := aiEnv(t)
	e.connect()
	body := map[string]any{"enabled": true, "guidelines": "intruso"}

	if code, _ := e.call(http.MethodPut, "", "/guilds/g1/ai/behavior", body); code != http.StatusUnauthorized {
		t.Errorf("sin sesión = %d, quería 401", code)
	}
	if code, _ := e.call(http.MethodPut, "luis", "/guilds/g1/ai/behavior", body); code != http.StatusNotFound {
		t.Errorf("otro servidor = %d, quería 404", code)
	}
	if s, _ := e.st.GetAISettings(context.Background(), "g1"); s == nil || s.Enabled || s.Guidelines != "" {
		t.Errorf("una petición rechazada cambió el comportamiento: %+v", s)
	}
}

func TestCleanTypeAI(t *testing.T) {
	instructions, escalation, problem := cleanTypeAI("  Ayuda con pedidos.  \r\n", "\nSi hay enfado\r\n")
	if problem != "" || instructions != "Ayuda con pedidos." || escalation != "Si hay enfado" {
		t.Errorf("válido: %q %q %q", instructions, escalation, problem)
	}
	if _, _, problem := cleanTypeAI("", ""); problem != "" {
		t.Errorf("vacíos deben ser válidos: %q", problem)
	}
	if _, _, problem := cleanTypeAI(strings.Repeat("a", 2001), ""); !strings.Contains(problem, "2000") {
		t.Errorf("instrucciones largas: %q", problem)
	}
	if _, _, problem := cleanTypeAI("", strings.Repeat("a", 1001)); !strings.Contains(problem, "1000") {
		t.Errorf("reglas largas: %q", problem)
	}
	if _, _, problem := cleanTypeAI(strings.Repeat("a", 2000), strings.Repeat("ñ", 1000)); problem != "" {
		t.Errorf("justo en el límite: %q", problem)
	}
}
