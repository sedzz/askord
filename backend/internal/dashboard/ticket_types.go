package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/sedzz/askord/internal/bot"
	"github.com/sedzz/askord/internal/store"
)

// Límites de Discord: la etiqueta de un botón admite 80 caracteres y la descripción
// de una opción de desplegable, 100.
const (
	maxTypeName        = 80
	maxTypeDescription = 100
	maxTypeEmoji       = 16 // letras: un emoji con modificadores ocupa varias
)

type typeBody struct {
	Name        string `json:"name"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
	CategoryID  string `json:"categoryId"`
	StaffRoleID string `json:"staffRoleId"`
}

// ListTicketTypes devuelve los tipos del servidor, en su orden.
func (h *Handler) ListTicketTypes(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	types, err := h.store.ListTicketTypes(r.Context(), guildID)
	if err != nil {
		slog.Error("listar tipos de ticket", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, types)
}

func (h *Handler) CreateTicketType(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	in, ok := h.readTypeInput(w, r, guildID)
	if !ok {
		return
	}
	t, err := h.store.CreateTicketType(r.Context(), guildID, in)
	if err != nil {
		typeError(w, err)
		return
	}
	h.refreshPanel(r.Context(), guildID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(t)
}

func (h *Handler) UpdateTicketType(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	id, ok := typeID(w, r)
	if !ok {
		return
	}
	in, ok := h.readTypeInput(w, r, guildID)
	if !ok {
		return
	}
	t, err := h.store.UpdateTicketType(r.Context(), guildID, id, in)
	if err != nil {
		typeError(w, err)
		return
	}
	h.refreshPanel(r.Context(), guildID)
	writeJSON(w, t)
}

func (h *Handler) DeleteTicketType(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	id, ok := typeID(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteTicketType(r.Context(), guildID, id); err != nil {
		typeError(w, err)
		return
	}
	h.refreshPanel(r.Context(), guildID)
	w.WriteHeader(http.StatusNoContent)
}

// ReorderTicketTypes recibe los IDs de todos los tipos en el orden deseado.
func (h *Handler) ReorderTicketTypes(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		http.Error(w, "cuerpo invalido", http.StatusBadRequest)
		return
	}
	if err := h.store.ReorderTicketTypes(r.Context(), guildID, body.IDs); err != nil {
		typeError(w, err)
		return
	}
	h.refreshPanel(r.Context(), guildID)
	w.WriteHeader(http.StatusNoContent)
}

// refreshPanel pone al día el panel ya publicado con los tipos actuales, para que no haya
// que volver a publicarlo. Es de mejor esfuerzo: si falla, el cambio ya está guardado.
func (h *Handler) refreshPanel(ctx context.Context, guildID string) {
	settings, err := h.store.GetTicketSettings(ctx, guildID)
	if err != nil || settings == nil || settings.PanelChannelID == "" || settings.PanelMessageID == "" {
		return
	}
	types, err := h.store.ListTicketTypes(ctx, guildID)
	if err != nil {
		slog.Warn("listar tipos para el panel", "guild", guildID, "err", err)
		return
	}
	if err := h.bot.UpdatePanel(settings.PanelChannelID, settings.PanelMessageID, types); err != nil {
		slog.Warn("actualizar panel", "guild", guildID, "err", err)
	}
}

func typeID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "typeID"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

// readTypeInput lee y valida el cuerpo; si algo no cuadra, responde 400 y devuelve false.
func (h *Handler) readTypeInput(w http.ResponseWriter, r *http.Request, guildID string) (store.TicketTypeInput, bool) {
	var body typeBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		http.Error(w, "cuerpo invalido", http.StatusBadRequest)
		return store.TicketTypeInput{}, false
	}
	in := store.TicketTypeInput{
		Name:        strings.TrimSpace(body.Name),
		Emoji:       strings.TrimSpace(body.Emoji),
		Description: strings.TrimSpace(body.Description),
		CategoryID:  body.CategoryID,
		StaffRoleID: body.StaffRoleID,
	}

	fail := func(msg string) (store.TicketTypeInput, bool) {
		http.Error(w, msg, http.StatusBadRequest)
		return store.TicketTypeInput{}, false
	}
	if n := utf8.RuneCountInString(in.Name); n == 0 || n > maxTypeName {
		return fail("el nombre debe tener entre 1 y 80 caracteres")
	}
	if utf8.RuneCountInString(in.Description) > maxTypeDescription {
		return fail("la descripcion admite hasta 100 caracteres")
	}
	if in.Emoji != "" && (utf8.RuneCountInString(in.Emoji) > maxTypeEmoji || strings.IndexFunc(in.Emoji, unicode.IsSpace) >= 0 || !bot.ValidEmoji(in.Emoji)) {
		return fail("el emoji no es valido")
	}

	// Cada tipo lleva su categoría y su rol de staff, y deben existir en este servidor.
	if in.CategoryID == "" {
		return fail("elige una categoria")
	}
	if in.StaffRoleID == "" {
		return fail("elige un rol de staff")
	}
	categories, err := h.bot.Categories(guildID)
	if err != nil {
		slog.Error("listar categorias", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return store.TicketTypeInput{}, false
	}
	if !slices.ContainsFunc(categories, func(o bot.Option) bool { return o.ID == in.CategoryID }) {
		return fail("categoria invalida")
	}
	roles, err := h.bot.Roles(guildID)
	if err != nil {
		slog.Error("listar roles", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return store.TicketTypeInput{}, false
	}
	if !slices.ContainsFunc(roles, func(o bot.Option) bool { return o.ID == in.StaffRoleID }) {
		return fail("rol invalido")
	}
	return in, true
}

func typeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrTypeNameTaken), errors.Is(err, store.ErrTooManyTypes), errors.Is(err, store.ErrLastType):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, "no encontrado", http.StatusNotFound)
	case errors.Is(err, store.ErrBadOrder):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		slog.Error("tipos de ticket", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
	}
}
