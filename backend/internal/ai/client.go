package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Config es la conexión con un LLM: la URL base de una API compatible con OpenAI (la que
// termina en /v1, sin /chat/completions), el modelo y la API key.
type Config struct {
	BaseURL string
	Model   string
	APIKey  string
}

// Client habla con APIs compatibles con OpenAI. La URL la elige el admin de cada servidor,
// así que por defecto no puede apuntar a la red interna del servidor (ver New).
type Client struct {
	http         *http.Client
	allowPrivate bool
}

const (
	requestTimeout  = 20 * time.Second
	maxResponseSize = 1 << 20
)

// carrier-grade NAT (100.64.0.0/10): no es privada para net.IP pero tampoco es internet.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// New crea un Client. Con allowPrivate a false, se rechazan las conexiones a direcciones
// locales, privadas o de metadatos de nube (SSRF). Se comprueba la IP ya resuelta al conectar,
// así que un DNS que cambie de respuesta tampoco lo esquiva.
func New(allowPrivate bool) *Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || isInternal(ip) {
				return errors.New("la dirección no está permitida")
			}
			return nil
		}
	}
	return &Client{
		allowPrivate: allowPrivate,
		http: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				Proxy:               nil, // un proxy del entorno saltaría la comprobación de la IP
				DialContext:         dialer.DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
			},
			// Una redirección podría llevar a una dirección interna: no se sigue ninguna.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func isInternal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || cgnat.Contains(ip)
}

// ValidateBaseURL comprueba la URL base y la devuelve normalizada (sin barra final).
func (c *Client) ValidateBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", errors.New("la URL no es válida")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && c.allowPrivate) {
		return "", errors.New("la URL debe empezar por https://")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("la URL no puede llevar usuario, parámetros ni fragmento")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !c.allowPrivate && isInternal(ip) {
		return "", errors.New("la URL no puede apuntar a una dirección local o privada")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// Test comprueba la conexión pidiéndole al modelo una respuesta mínima. El error que devuelve
// está pensado para enseñárselo al admin: nunca incluye la API key.
func (c *Client) Test(ctx context.Context, cfg Config) error {
	base, err := c.ValidateBaseURL(cfg.BaseURL)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"model":      cfg.Model,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "Responde solo: ok"}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return errors.New("la URL no es válida")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return connectionError(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return errors.New("no se pudo leer la respuesta del servidor")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp.StatusCode, data, cfg.APIKey)
	}
	var out struct {
		Choices []json.RawMessage `json:"choices"`
	}
	if json.Unmarshal(data, &out) != nil || len(out.Choices) == 0 {
		return errors.New("el servidor respondió, pero no parece una API compatible con OpenAI")
	}
	return nil
}

func connectionError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("el servidor tardó demasiado en responder")
	case strings.Contains(err.Error(), "la dirección no está permitida"):
		return errors.New("la URL apunta a una dirección local o privada, que no está permitida")
	}
	return errors.New("no se pudo conectar con el servidor")
}

func statusError(code int, data []byte, apiKey string) error {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("la API key no es válida o no tiene permisos")
	case http.StatusNotFound:
		return errors.New("no se encontró el endpoint o el modelo; revisa la URL y el nombre del modelo")
	case http.StatusTooManyRequests:
		return errors.New("el proveedor limitó las peticiones (o la cuenta no tiene saldo)")
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return errors.New("el servidor redirige a otra dirección; usa la URL final")
	}
	// Muchos proveedores explican el motivo en {"error":{"message":"…"}}.
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		msg := e.Error.Message
		if apiKey != "" {
			msg = strings.ReplaceAll(msg, apiKey, "***")
		}
		if r := []rune(msg); len(r) > 200 {
			msg = string(r[:200]) + "…"
		}
		return fmt.Errorf("el proveedor respondió con un error (%d): %s", code, msg)
	}
	return fmt.Errorf("el proveedor respondió con un error (%d)", code)
}
