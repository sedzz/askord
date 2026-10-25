package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func reply(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func chatCfg(l *llm) Config {
	return Config{BaseURL: l.srv.URL + "/v1", Model: "mi-modelo", APIKey: "sk-1"}
}

func TestChatText(t *testing.T) {
	l := newLLM(t, reply(`{"choices":[{"message":{"role":"assistant","content":"  Hola, ¿en qué te ayudo?  "}}],
		"usage":{"prompt_tokens":120,"completion_tokens":9}}`))

	resp, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{
		Messages:  []Message{{Role: RoleSystem, Content: "reglas"}, {Role: RoleUser, Content: "hola"}},
		MaxTokens: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hola, ¿en qué te ayudo?" || len(resp.ToolCalls) != 0 {
		t.Errorf("respuesta = %+v", resp)
	}
	if resp.PromptTokens != 120 || resp.CompletionTokens != 9 {
		t.Errorf("uso = %d/%d", resp.PromptTokens, resp.CompletionTokens)
	}

	if l.path != "/v1/chat/completions" || l.auth != "Bearer sk-1" || l.body["model"] != "mi-modelo" {
		t.Errorf("petición: %s %s %v", l.path, l.auth, l.body["model"])
	}
	if l.body["max_tokens"] != float64(300) {
		t.Errorf("max_tokens = %v", l.body["max_tokens"])
	}
	msgs, _ := l.body["messages"].([]any)
	if len(msgs) != 2 || msgs[1].(map[string]any)["role"] != "user" {
		t.Errorf("mensajes enviados = %v", l.body["messages"])
	}
	// Sin herramientas no se piden.
	if _, ok := l.body["tools"]; ok {
		t.Error("se enviaron herramientas sin pedirlas")
	}
	if _, ok := l.body["tool_choice"]; ok {
		t.Error("se envió tool_choice sin herramientas")
	}
}

func TestChatToolCall(t *testing.T) {
	l := newLLM(t, reply(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
		{"id":"c1","type":"function","function":{"name":"escalar_a_staff","arguments":"{\"motivo\":\" Pide un reembolso \"}"}}]}}]}`))

	resp, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "quiero mi dinero"}},
		Tools:    []Tool{EscalateTool},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != EscalateToolName {
		t.Fatalf("herramientas = %+v", resp.ToolCalls)
	}
	if got := resp.ToolCalls[0].Argument("motivo"); got != "Pide un reembolso" {
		t.Errorf("motivo = %q", got)
	}
	if resp.Content != "" {
		t.Errorf("contenido = %q, quería vacío", resp.Content)
	}

	tools, _ := l.body["tools"].([]any)
	if len(tools) != 1 || l.body["tool_choice"] != "auto" {
		t.Fatalf("tools = %v, tool_choice = %v", l.body["tools"], l.body["tool_choice"])
	}
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if tools[0].(map[string]any)["type"] != "function" || fn["name"] != "escalar_a_staff" || fn["parameters"] == nil {
		t.Errorf("herramienta enviada = %v", tools[0])
	}
}

func TestChatTextAndToolCall(t *testing.T) {
	l := newLLM(t, reply(`{"choices":[{"message":{"content":"Te paso con una persona.","tool_calls":[
		{"function":{"name":"escalar_a_staff","arguments":"{\"motivo\":\"x\"}"}}]}}]}`))
	resp, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
	if err != nil || resp.Content != "Te paso con una persona." || len(resp.ToolCalls) != 1 {
		t.Errorf("respuesta = %+v (%v)", resp, err)
	}
}

func TestToolCallArgument(t *testing.T) {
	cases := []struct{ name, args, want string }{
		{"normal", `{"motivo":"x"}`, "x"},
		{"recorta", `{"motivo":"  x  "}`, "x"},
		{"falta", `{"otro":"x"}`, ""},
		{"no es texto", `{"motivo":3}`, ""},
		{"json roto", `{"motivo":`, ""},
		{"vacío", ``, ""},
	}
	for _, c := range cases {
		if got := (ToolCall{Arguments: c.args}).Argument("motivo"); got != c.want {
			t.Errorf("%s: %q, quería %q", c.name, got, c.want)
		}
	}
}

func TestChatBadResponses(t *testing.T) {
	cases := map[string]string{
		"sin opciones":   `{"choices":[]}`,
		"no es openai":   `{"hola":"mundo"}`,
		"no es json":     `<html>`,
		"vacía":          `{"choices":[{"message":{"content":""}}]}`,
		"solo espacios":  `{"choices":[{"message":{"content":"   "}}]}`,
		"contenido nulo": `{"choices":[{"message":{"content":null}}]}`,
	}
	for name, body := range cases {
		l := newLLM(t, reply(body))
		_, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
		var e *Error
		if !errors.As(err, &e) || e.Kind != KindBadResponse {
			t.Errorf("%s: error %v, quería KindBadResponse", name, err)
		}
	}
}

func TestChatErrorKinds(t *testing.T) {
	status := func(code int) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
	}
	cases := []struct {
		name   string
		handle func(http.ResponseWriter, *http.Request)
		want   ErrorKind
	}{
		{"key inválida", status(401), KindAuth},
		{"sin permisos", status(403), KindAuth},
		{"modelo inexistente", status(404), KindNotFound},
		{"límite o sin saldo", status(429), KindRateLimit},
		{"error del proveedor", status(500), KindOther},
	}
	for _, c := range cases {
		l := newLLM(t, c.handle)
		_, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
		var e *Error
		if !errors.As(err, &e) || e.Kind != c.want {
			t.Errorf("%s: error %v, quería tipo %d", c.name, err, c.want)
		}
	}
}

func TestChatConnectionAndTimeout(t *testing.T) {
	// Servidor que ya no existe: no se puede conectar.
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	_, err := New(true).Chat(context.Background(), Config{BaseURL: url, Model: "m", APIKey: "k"},
		ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindConnection {
		t.Errorf("sin servidor: %v, quería KindConnection", err)
	}

	// Servidor que no contesta.
	l := newLLM(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = New(true).Chat(ctx, chatCfg(l), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
	if !errors.As(err, &e) || e.Kind != KindTimeout {
		t.Errorf("sin respuesta: %v, quería KindTimeout", err)
	}
}

func TestChatNeverLeaksTheKey(t *testing.T) {
	l := newLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"clave sk-1 rechazada"}}`))
	})
	_, err := New(true).Chat(context.Background(), chatCfg(l), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
	if err == nil || strings.Contains(err.Error(), "sk-1") {
		t.Errorf("error = %v, la key debía ocultarse", err)
	}
}

func TestChatBlocksInternalAddresses(t *testing.T) {
	hit := false
	l := newLLM(t, func(http.ResponseWriter, *http.Request) { hit = true })
	port := l.srv.URL[strings.LastIndex(l.srv.URL, ":"):]
	_, err := New(false).Chat(context.Background(), Config{BaseURL: "https://localhost" + port, Model: "m", APIKey: "k"},
		ChatRequest{Messages: []Message{{Role: RoleUser, Content: "a"}}})
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindConnection || hit {
		t.Errorf("error = %v, hit = %v; debía bloquear la conexión", err, hit)
	}
}
