package store

import (
	"context"
	"time"
)

// TranscriptFilter acota qué tickets se listan. From y To son fechas de creación del
// ticket (To excluida); User busca entre quienes escribieron, por nombre o por ID.
type TranscriptFilter struct {
	From *time.Time
	To   *time.Time
	User string
	// 0 no filtra por tipo.
	TypeID int64
}

type TranscriptSummary struct {
	ID         int64      `json:"id"`
	Number     int        `json:"number"`
	OpenerID   string     `json:"openerId"`
	OpenerName string     `json:"openerName"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ClosedAt   *time.Time `json:"closedAt"`
	Messages   int        `json:"messages"`
	// Tipo del ticket: el actual si sigue existiendo y, si no, el que tenía al abrirse.
	TypeID    int64  `json:"typeId"`
	TypeName  string `json:"typeName"`
	TypeEmoji string `json:"typeEmoji"`
	// Respuestas que dio quien abrió el ticket al formulario de su tipo.
	Answers []Answer `json:"answers"`
	// Primer mensaje con texto de quien abrió el ticket (recortado): de qué se habló.
	Preview string `json:"preview"`
}

type TranscriptMessage struct {
	ID           string    `json:"id"`
	AuthorID     string    `json:"authorId"`
	AuthorName   string    `json:"authorName"`
	AuthorAvatar string    `json:"authorAvatar"`
	Content      string    `json:"content"`
	Attachments  []string  `json:"attachments"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Transcript es un ticket con su conversación.
type Transcript struct {
	TranscriptSummary
	Conversation []TranscriptMessage `json:"conversation"`
}

// Solo cuentan los tickets que ya tienen transcripción guardada.
const transcriptWhere = `
	t.guild_id = $1
	AND EXISTS (SELECT 1 FROM ticket_messages x WHERE x.ticket_id = t.id)
	AND ($2::timestamptz IS NULL OR t.created_at >= $2)
	AND ($3::timestamptz IS NULL OR t.created_at < $3)
	AND ($4::text = '' OR EXISTS (
		SELECT 1 FROM ticket_messages u
		WHERE u.ticket_id = t.id
		  AND (u.author_id = $4 OR strpos(lower(u.author_name), lower($4)) > 0)))
	AND ($5::bigint = 0 OR t.id = $5)
	AND ($6::bigint = 0 OR t.type_id = $6)`

const transcriptColumns = `
	t.id, t.number, t.opener_id,
	COALESCE((SELECT m.author_name FROM ticket_messages m
	          WHERE m.ticket_id = t.id AND m.author_id = t.opener_id
	          ORDER BY m.created_at LIMIT 1), t.opener_id),
	t.status, t.created_at, t.closed_at,
	(SELECT count(*) FROM ticket_messages m WHERE m.ticket_id = t.id),
	COALESCE((SELECT left(m.content, 200) FROM ticket_messages m
	          WHERE m.ticket_id = t.id AND m.author_id = t.opener_id AND m.content <> ''
	          ORDER BY m.created_at LIMIT 1), ''),
	COALESCE(t.type_id, 0), COALESCE(ty.name, t.type_name), COALESCE(ty.emoji, t.type_emoji), t.answers`

// ListTranscripts devuelve los tickets con transcripción, del más reciente al más antiguo.
// ticketID a 0 no filtra por ticket.
func (s *Store) ListTranscripts(ctx context.Context, guildID string, f TranscriptFilter, ticketID int64, limit, offset int) ([]TranscriptSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+transcriptColumns+`
		FROM tickets t
		LEFT JOIN ticket_types ty ON ty.id = t.type_id AND ty.guild_id = t.guild_id
		WHERE `+transcriptWhere+`
		ORDER BY t.created_at DESC
		LIMIT $7 OFFSET $8`,
		guildID, f.From, f.To, f.User, ticketID, f.TypeID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TranscriptSummary{}
	for rows.Next() {
		var t TranscriptSummary
		if err := rows.Scan(&t.ID, &t.Number, &t.OpenerID, &t.OpenerName, &t.Status,
			&t.CreatedAt, &t.ClosedAt, &t.Messages, &t.Preview, &t.TypeID, &t.TypeName, &t.TypeEmoji, &t.Answers); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// maxExportTickets limita cuántos tickets entran en una misma exportación.
const maxExportTickets = 500

// LoadTranscripts devuelve los tickets que cumplen el filtro con su conversación completa,
// para ver uno o para exportar. ticketID a 0 no filtra por ticket.
func (s *Store) LoadTranscripts(ctx context.Context, guildID string, f TranscriptFilter, ticketID int64) ([]Transcript, error) {
	summaries, err := s.ListTranscripts(ctx, guildID, f, ticketID, maxExportTickets, 0)
	if err != nil {
		return nil, err
	}
	if len(summaries) == 0 {
		return []Transcript{}, nil
	}

	ids := make([]int64, 0, len(summaries))
	byID := make(map[int64]*Transcript, len(summaries))
	out := make([]Transcript, len(summaries))
	for i, sum := range summaries {
		ids = append(ids, sum.ID)
		out[i] = Transcript{TranscriptSummary: sum, Conversation: []TranscriptMessage{}}
		byID[sum.ID] = &out[i]
	}

	rows, err := s.pool.Query(ctx, `
		SELECT ticket_id, message_id, author_id, author_name, author_avatar, content, attachments, created_at
		FROM ticket_messages
		WHERE ticket_id = ANY($1)
		ORDER BY ticket_id, created_at`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			ticketID int64
			m        TranscriptMessage
		)
		if err := rows.Scan(&ticketID, &m.ID, &m.AuthorID, &m.AuthorName, &m.AuthorAvatar, &m.Content,
			&m.Attachments, &m.CreatedAt); err != nil {
			return nil, err
		}
		if m.Attachments == nil {
			m.Attachments = []string{}
		}
		if t := byID[ticketID]; t != nil {
			t.Conversation = append(t.Conversation, m)
		}
	}
	return out, rows.Err()
}
