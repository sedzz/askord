package dashboard

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sedzz/askord/internal/store"
)

const transcriptsPageSize = 50

// filterFromQuery lee ?from=YYYY-MM-DD&to=YYYY-MM-DD&user=… Ambas fechas son inclusivas
// y se interpretan en UTC.
func filterFromQuery(r *http.Request) (store.TranscriptFilter, error) {
	q := r.URL.Query()
	f := store.TranscriptFilter{User: strings.TrimSpace(q.Get("user"))}

	if v := q.Get("type"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			return f, fmt.Errorf("tipo invalido")
		}
		f.TypeID = id
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return f, fmt.Errorf("fecha inicial invalida")
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return f, fmt.Errorf("fecha final invalida")
		}
		end := t.AddDate(0, 0, 1) // el día final entra entero
		f.To = &end
	}
	return f, nil
}

type transcriptsResponse struct {
	Items   []store.TranscriptSummary `json:"items"`
	HasMore bool                      `json:"hasMore"`
}

// ListTranscripts lista los tickets con transcripción, con filtros y paginación.
func (h *Handler) ListTranscripts(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	f, err := filterFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Se pide uno de más para saber si hay otra página.
	items, err := h.store.ListTranscripts(r.Context(), guildID, f, 0, transcriptsPageSize+1, offset)
	if err != nil {
		slog.Error("listar transcripciones", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	hasMore := len(items) > transcriptsPageSize
	if hasMore {
		items = items[:transcriptsPageSize]
	}
	writeJSON(w, transcriptsResponse{Items: items, HasMore: hasMore})
}

// GetTranscript devuelve un ticket con su conversación.
func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	t, ok := h.oneTranscript(w, r)
	if !ok {
		return
	}
	writeJSON(w, t)
}

// DeleteTranscript borra el contenido de un ticket cerrado (mensajes y respuestas).
func (h *Handler) DeleteTranscript(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "ticketID"), 10, 64)
	if err != nil || ticketID <= 0 {
		http.NotFound(w, r)
		return
	}
	switch err := h.store.DeleteTranscript(r.Context(), guildID, ticketID); {
	case errors.Is(err, store.ErrNotFound):
		http.NotFound(w, r)
	case err != nil:
		slog.Error("eliminar transcripcion", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
	default:
		slog.Info("transcripcion eliminada a mano", "guild", guildID, "ticket", ticketID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ExportTranscript descarga un ticket en txt, json o csv.
func (h *Handler) ExportTranscript(w http.ResponseWriter, r *http.Request) {
	t, ok := h.oneTranscript(w, r)
	if !ok {
		return
	}
	name := fmt.Sprintf("ticket-%04d", t.Number)
	writeExport(w, r.URL.Query().Get("format"), name, []store.Transcript{t})
}

// ExportTranscripts descarga, en csv, json o txt, todos los tickets que cumplen el filtro.
func (h *Handler) ExportTranscripts(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	f, err := filterFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	all, err := h.store.LoadTranscripts(r.Context(), guildID, f, 0)
	if err != nil {
		slog.Error("exportar transcripciones", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	name := "transcripciones-" + time.Now().UTC().Format("20060102")
	writeExport(w, r.URL.Query().Get("format"), name, all)
}

// oneTranscript carga el ticket de la URL, o responde 404 si no existe en este servidor.
func (h *Handler) oneTranscript(w http.ResponseWriter, r *http.Request) (store.Transcript, bool) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return store.Transcript{}, false
	}
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "ticketID"), 10, 64)
	if err != nil || ticketID <= 0 {
		http.NotFound(w, r)
		return store.Transcript{}, false
	}
	all, err := h.store.LoadTranscripts(r.Context(), guildID, store.TranscriptFilter{}, ticketID)
	if err != nil {
		slog.Error("cargar transcripcion", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return store.Transcript{}, false
	}
	if len(all) == 0 {
		http.NotFound(w, r)
		return store.Transcript{}, false
	}
	return all[0], true
}
