package dashboard

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/sedzz/askord/internal/auth"
	"github.com/sedzz/askord/internal/bot"
	"github.com/sedzz/askord/internal/store"
)

type ticketsResponse struct {
	Settings     *store.TicketSettings `json:"settings"`
	Categories   []bot.Option          `json:"categories"`
	Roles        []bot.Option          `json:"roles"`
	TextChannels []bot.Option          `json:"textChannels"`
}

// guildID devuelve el servidor de la URL si el usuario lo administra y el bot
// está dentro. Si no, responde 404 (sin distinguir los casos) y devuelve false.
func (h *Handler) guildID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "guildID")
	ok, err := h.store.CanManageInstalledGuild(r.Context(), auth.UserFrom(r.Context()).ID, id)
	if err != nil {
		slog.Error("comprobar acceso al servidor", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return "", false
	}
	if !ok {
		http.NotFound(w, r)
		return "", false
	}
	return id, true
}

// GetTickets devuelve la configuración de tickets y las opciones para elegir.
func (h *Handler) GetTickets(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}

	settings, err := h.store.GetTicketSettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de tickets", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	categories, err := h.bot.Categories(guildID)
	if err != nil {
		slog.Error("listar categorias", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	roles, err := h.bot.Roles(guildID)
	if err != nil {
		slog.Error("listar roles", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}

	textChannels, err := h.bot.TextChannels(guildID)
	if err != nil {
		slog.Error("listar canales", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}

	writeJSON(w, ticketsResponse{
		Settings: settings, Categories: categories, Roles: roles, TextChannels: textChannels,
	})
}

// PublishPanel envía el panel con el botón "Crear ticket" al canal elegido.
func (h *Handler) PublishPanel(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}

	var in struct {
		ChannelID string `json:"channelId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		http.Error(w, "cuerpo invalido", http.StatusBadRequest)
		return
	}

	settings, err := h.store.GetTicketSettings(r.Context(), guildID)
	if err != nil {
		slog.Error("leer configuracion de tickets", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if settings == nil {
		http.Error(w, "guarda primero la categoria y el rol", http.StatusConflict)
		return
	}

	channels, err := h.bot.TextChannels(guildID)
	if err != nil {
		slog.Error("listar canales", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	if !slices.ContainsFunc(channels, func(o bot.Option) bool { return o.ID == in.ChannelID }) {
		http.Error(w, "canal invalido", http.StatusBadRequest)
		return
	}

	if err := h.store.EnsureDefaultTicketType(r.Context(), guildID); err != nil {
		slog.Error("crear tipo por defecto", "err", err)
	}
	types, err := h.store.ListTicketTypes(r.Context(), guildID)
	if err != nil {
		slog.Error("listar tipos de ticket", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}

	messageID, err := h.bot.PublishPanel(in.ChannelID, settings.PanelChannelID, settings.PanelMessageID, types)
	if errors.Is(err, bot.ErrMissingPermissions) {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err != nil {
		slog.Error("publicar panel", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	if err := h.store.SavePanel(r.Context(), guildID, in.ChannelID, messageID); err != nil {
		slog.Error("guardar panel", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"channelId": in.ChannelID})
}

// PutTickets guarda la categoría y el rol de staff, validando que existan en el servidor.
func (h *Handler) PutTickets(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}

	var in store.TicketSettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		http.Error(w, "cuerpo invalido", http.StatusBadRequest)
		return
	}

	categories, err := h.bot.Categories(guildID)
	if err != nil {
		slog.Error("listar categorias", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	roles, err := h.bot.Roles(guildID)
	if err != nil {
		slog.Error("listar roles", "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	hasID := func(opts []bot.Option, id string) bool {
		return slices.ContainsFunc(opts, func(o bot.Option) bool { return o.ID == id })
	}
	if !hasID(categories, in.CategoryID) || !hasID(roles, in.StaffRoleID) {
		http.Error(w, "categoria o rol invalido", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveTicketSettings(r.Context(), guildID, in); err != nil {
		slog.Error("guardar configuracion de tickets", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if err := h.store.EnsureDefaultTicketType(r.Context(), guildID); err != nil {
		slog.Error("crear tipo por defecto", "err", err)
	}
	writeJSON(w, in)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// CreateCategory crea la categoría "Tickets" y la devuelve para seleccionarla.
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	h.created(w, "categoria", func() (bot.Option, error) { return h.bot.CreateCategory(guildID, "Tickets") })
}

// CreateRole crea el rol "Soporte" y lo devuelve para seleccionarlo.
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	h.created(w, "rol", func() (bot.Option, error) { return h.bot.CreateRole(guildID, "Soporte") })
}

func (h *Handler) created(w http.ResponseWriter, what string, create func() (bot.Option, error)) {
	opt, err := create()
	if errors.Is(err, bot.ErrMissingPermissions) {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err != nil {
		slog.Error("crear "+what, "err", err)
		http.Error(w, "no se pudo consultar Discord", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(opt)
}

// CreateChannel crea el canal "soporte" (solo lectura) para publicar el panel.
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}
	h.created(w, "canal", func() (bot.Option, error) { return h.bot.CreateTextChannel(guildID, "soporte") })
}
