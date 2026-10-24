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

// TypeCount es cuántos tickets de un tipo se abrieron en un periodo.
type TypeCount struct {
	TypeID int64  `json:"typeId"` // 0 en la fila "otros"
	Name   string `json:"name"`
	Emoji  string `json:"emoji"`
	Count  int    `json:"count"`
}

// TicketsByType cuenta los tickets abiertos en los últimos days días (el de hoy incluido),
// por cada tipo actual del servidor en su orden y, aparte, los que no tienen tipo o cuyo
// tipo se borró ("otros", con typeId 0, solo si los hay).
func (s *Store) TicketsByType(ctx context.Context, guildID string, days int) ([]TypeCount, error) {
	since := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))

	rows, err := s.pool.Query(ctx, `
		SELECT ty.id, ty.name, ty.emoji, count(t.id)
		FROM ticket_types ty
		LEFT JOIN tickets t
		       ON t.guild_id = ty.guild_id AND t.type_id = ty.id AND t.created_at >= $2
		WHERE ty.guild_id = $1
		GROUP BY ty.id
		ORDER BY ty.position, ty.id`, guildID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TypeCount{}
	for rows.Next() {
		var c TypeCount
		if err := rows.Scan(&c.TypeID, &c.Name, &c.Emoji, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var others int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM tickets t
		WHERE t.guild_id = $1 AND t.created_at >= $2
		  AND NOT EXISTS (SELECT 1 FROM ticket_types ty WHERE ty.id = t.type_id AND ty.guild_id = t.guild_id)`,
		guildID, since).Scan(&others); err != nil {
		return nil, err
	}
	if others > 0 {
		out = append(out, TypeCount{Name: "Otros", Count: others})
	}
	return out, nil
}
