package store

import (
	"context"
	"time"
)

// TicketStats resume los tickets de un servidor.
type TicketStats struct {
	Total  int `json:"total"`
	Open   int `json:"open"`
	Closed int `json:"closed"`
	// Segundos que tarda de media en cerrarse un ticket; 0 si aún no hay ninguno cerrado.
	AvgResolutionSeconds int `json:"avgResolutionSeconds"`
}

// SeriesPoint es cuántos tickets se abrieron un día (UTC).
type SeriesPoint struct {
	Date   string `json:"date"` // YYYY-MM-DD
	Opened int    `json:"opened"`
}

func (s *Store) TicketStats(ctx context.Context, guildID string) (TicketStats, error) {
	var st TicketStats
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'open'),
		       count(*) FILTER (WHERE status = 'closed'),
		       COALESCE(avg(extract(epoch FROM closed_at - created_at))
		                FILTER (WHERE status = 'closed' AND closed_at IS NOT NULL), 0)::int
		FROM tickets WHERE guild_id = $1`, guildID).
		Scan(&st.Total, &st.Open, &st.Closed, &st.AvgResolutionSeconds)
	return st, err
}

// TicketSeries devuelve los tickets abiertos cada día de los últimos days días (el de hoy
// incluido), con 0 en los días sin tickets para que la gráfica no tenga huecos.
func (s *Store) TicketSeries(ctx context.Context, guildID string, days int) ([]SeriesPoint, error) {
	today := time.Now().UTC().Format(time.DateOnly)
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(d, 'YYYY-MM-DD'), count(t.id)
		FROM generate_series($2::date - ($3::int - 1), $2::date, interval '1 day') AS d
		LEFT JOIN tickets t
		       ON t.guild_id = $1
		      AND t.created_at >= (d AT TIME ZONE 'UTC')
		      AND t.created_at <  ((d + interval '1 day') AT TIME ZONE 'UTC')
		GROUP BY d
		ORDER BY d`, guildID, today, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SeriesPoint, 0, days)
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Date, &p.Opened); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
