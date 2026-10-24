package dashboard

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sedzz/askord/internal/bot"
	"github.com/sedzz/askord/internal/store"
)

type overviewResponse struct {
	// Nil si Discord no respondió: las estadísticas de tickets se muestran igualmente.
	Guild   *bot.GuildOverview  `json:"guild"`
	Tickets store.TicketStats   `json:"tickets"`
	Series  []store.SeriesPoint `json:"series"`
}

// Rangos de la gráfica que se permiten, en días.
var seriesRanges = map[int]bool{7: true, 30: true, 90: true}

// Overview devuelve la ficha del servidor y las estadísticas de sus tickets.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	guildID, ok := h.guildID(w, r)
	if !ok {
		return
	}

	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || !seriesRanges[n] {
			http.Error(w, "days debe ser 7, 30 o 90", http.StatusBadRequest)
			return
		}
		days = n
	}

	stats, err := h.store.TicketStats(r.Context(), guildID)
	if err != nil {
		slog.Error("estadisticas de tickets", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	series, err := h.store.TicketSeries(r.Context(), guildID, days)
	if err != nil {
		slog.Error("serie de tickets", "err", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}

	resp := overviewResponse{Tickets: stats, Series: series}
	if g, err := h.bot.Overview(guildID); err != nil {
		slog.Warn("ficha del servidor", "guild", guildID, "err", err)
	} else {
		resp.Guild = &g
	}
	writeJSON(w, resp)
}
