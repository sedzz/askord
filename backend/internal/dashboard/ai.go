package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sedzz/askord/internal/ai"
	"github.com/sedzz/askord/internal/store"
)

const (
	maxAIModel  = 100
	maxAIAPIKey = 500
)

// aiView es lo que el dashboard ve de la conexión. La API key nunca sale: solo si existe.
type aiView struct {
	// Available es false si este despliegue de Askord no tiene configurada la clave de cifrado.
	Available bool   `json:"available"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	HasKey    bool   `json:"hasKey"`
	// Interruptor general del asistente y pautas del servidor.
	Enabled    bool   `json:"enabled"`
	Guidelines string `json:"guidelines"`
}

type aiBody struct {
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	// Vacía: se conserva la que ya había guardada.
	APIKey string `json:"apiKey"`
}

type aiBehaviorBody struct {
	Enabled    bool   `json:"enabled"`
	Guidelines string `json:"guidelines"`
}

// GetAI devuelve la conexión con el LLM del servidor.
func (h *Handler) GetAI(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	a, err := h.store.GetAISettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, h.aiView(a))
}

func (h *Handler) aiView(a *store.AISettings) aiView {
	v := aiView{Available: h.sealer != nil}
	if a != nil {
		v.BaseURL, v.Model, v.HasKey = a.BaseURL, a.Model, len(a.KeyEnc) > 0
		v.Enabled, v.Guidelines = a.Enabled, a.Guidelines
	}
	return v
}

// PutAI guarda la conexión con el LLM. No la prueba: para eso está TestAI.
func (h *Handler) PutAI(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	body, existing, ok := h.aiInput(w, r, guildID)
	if !ok {
		return
	}

	in := store.AISettings{BaseURL: body.BaseURL, Model: body.Model}
	if body.APIKey != "" {
		enc, err := h.sealer.Seal(body.APIKey, guildID)
		if err != nil {
			slog.Error("cifrar api key", "err", err)
			http.Error(w, "error interno", http.StatusInternalServerError)
			return
		}
		in.KeyEnc = enc
	} else if existing == nil || len(existing.KeyEnc) == 0 {
		http.Error(w, "falta la API key", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveAISettings(r.Context(), guildID, in); err != nil {
		slog.Error("guardar configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	saved, err := h.store.GetAISettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, h.aiView(saved))
}

// PutAIBehavior guarda el interruptor general del asistente y las pautas del servidor. Hace
// falta una conexión guardada: sin proveedor no hay asistente que activar.
func (h *Handler) PutAIBehavior(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	if h.sealer == nil {
		http.Error(w, "la IA no está disponible en este despliegue de Askord", http.StatusServiceUnavailable)
		return
	}
	var body aiBehaviorBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		http.Error(w, "cuerpo invalido", http.StatusBadRequest)
		return
	}
	guidelines := ai.CleanText(body.Guidelines)
	if utf8.RuneCountInString(guidelines) > ai.MaxGuidelines {
		http.Error(w, "las pautas admiten hasta 4000 caracteres", http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetAISettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if existing == nil || len(existing.KeyEnc) == 0 {
		http.Error(w, "guarda antes la conexión con tu proveedor de IA", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveAIBehavior(r.Context(), guildID, body.Enabled, guidelines); err != nil {
		slog.Error("guardar comportamiento de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	saved, err := h.store.GetAISettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, h.aiView(saved))
}

// cleanTypeAI normaliza y valida los textos de IA de un tipo de ticket. Devuelve el motivo si
// algo no es válido.
func cleanTypeAI(instructions, escalation string) (string, string, string) {
	instructions, escalation = ai.CleanText(instructions), ai.CleanText(escalation)
	if utf8.RuneCountInString(instructions) > ai.MaxInstructions {
		return "", "", "las instrucciones de la IA admiten hasta 2000 caracteres"
	}
	if utf8.RuneCountInString(escalation) > ai.MaxEscalation {
		return "", "", "las reglas para pasar a una persona admiten hasta 1000 caracteres"
	}
	return instructions, escalation, ""
}

// DeleteAI borra la conexión, API key incluida.
func (h *Handler) DeleteAI(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteAISettings(r.Context(), guildID); err != nil {
		slog.Error("borrar configuracion de ia", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type aiTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// TestAI prueba una conexión sin guardarla. Con la API key vacía usa la ya guardada, así el
// admin puede probar cambios de URL o de modelo sin volver a escribirla.
func (h *Handler) TestAI(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	body, existing, ok := h.aiInput(w, r, guildID)
	if !ok {
		return
	}

	key := body.APIKey
	if key == "" {
		if existing == nil || len(existing.KeyEnc) == 0 {
			http.Error(w, "falta la API key", http.StatusBadRequest)
			return
		}
		var err error
		if key, err = h.sealer.Open(existing.KeyEnc, guildID); err != nil {
			writeJSON(w, aiTestResult{Message: "no se pudo leer la API key guardada; vuelve a introducirla"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	if err := h.llm.Test(ctx, ai.Config{BaseURL: body.BaseURL, Model: body.Model, APIKey: key}); err != nil {
		writeJSON(w, aiTestResult{Message: err.Error()})
		return
	}
	writeJSON(w, aiTestResult{OK: true})
}

// aiInput lee y valida el cuerpo y devuelve también lo que ya había guardado. Responde por sí
// misma si algo falla.
func (h *Handler) aiInput(w http.ResponseWriter, r *http.Request, guildID string) (aiBody, *store.AISettings, bool) {
	fail := func(status int, msg string) (aiBody, *store.AISettings, bool) {
		http.Error(w, msg, status)
		return aiBody{}, nil, false
	}
	if h.sealer == nil {
		return fail(http.StatusServiceUnavailable, "la IA no está disponible en este despliegue de Askord")
	}

	var body aiBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<13)).Decode(&body); err != nil {
		return fail(http.StatusBadRequest, "cuerpo invalido")
	}
	baseURL, err := h.llm.ValidateBaseURL(body.BaseURL)
	if err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}
	body.BaseURL = baseURL
	body.Model = strings.TrimSpace(body.Model)
	if n := utf8.RuneCountInString(body.Model); n == 0 || n > maxAIModel {
		return fail(http.StatusBadRequest, "el modelo debe tener entre 1 y 100 caracteres")
	}
	body.APIKey = strings.TrimSpace(body.APIKey)
	if utf8.RuneCountInString(body.APIKey) > maxAIAPIKey {
		return fail(http.StatusBadRequest, "la API key es demasiado larga")
	}

	existing, err := h.store.GetAISettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de ia", "err", err)
		return fail(http.StatusInternalServerError, "error interno")
	}
	return body, existing, true
}
