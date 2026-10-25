package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// newTestStore se conecta a la base de datos de TEST_DATABASE_URL (con las migraciones ya
// aplicadas o por aplicar) y la deja sin tickets, tipos ni conexiones de IA. Sin la variable, el test se omite.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	st, err := New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tickets", "ticket_types", "ai_settings"} {
		if _, err := st.pool.Exec(context.Background(), `DELETE FROM `+table); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// insertTicket crea un ticket con un mensaje y respuestas. closedAgo nil lo deja abierto.
func insertTicket(t *testing.T, st *Store, guild string, number int, closedAgo *time.Duration) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := st.pool.QueryRow(ctx, `
		INSERT INTO tickets (guild_id, number, channel_id, opener_id, status, closed_at, answers)
		VALUES ($1, $2, $3, 'u1',
		        CASE WHEN $4::timestamptz IS NULL THEN 'open' ELSE 'closed' END, $4,
		        '[{"question":"¿Qué pasa?","answer":"nada"}]')
		RETURNING id`,
		guild, number, fmt.Sprintf("%s-chan-%d", guild, number), closedTime(closedAgo)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	msg := TicketMessage{ID: fmt.Sprintf("%s-msg-%d", guild, number), AuthorID: "u1", AuthorName: "Ana", Content: "hola", CreatedAt: time.Now()}
	if err := st.SaveTicketMessages(ctx, id, []TicketMessage{msg}); err != nil {
		t.Fatal(err)
	}
	return id
}

func closedTime(ago *time.Duration) *time.Time {
	if ago == nil {
		return nil
	}
	c := time.Now().Add(-*ago)
	return &c
}

func hasContent(t *testing.T, st *Store, id int64) (messages int, answers string, purged bool) {
	t.Helper()
	err := st.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM ticket_messages WHERE ticket_id = t.id),
		       t.answers::text, t.content_purged_at IS NOT NULL
		FROM tickets t WHERE t.id = $1`, id).Scan(&messages, &answers, &purged)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestPurgeExpiredTranscripts(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	old, recent := 100*24*time.Hour, 10*24*time.Hour

	expired := insertTicket(t, st, "g1", 1, &old)
	fresh := insertTicket(t, st, "g1", 2, &recent)
	open := insertTicket(t, st, "g1", 3, nil)

	n, err := st.PurgeExpiredTranscripts(ctx, time.Now().Add(-TranscriptRetention), 500)
	if err != nil || n != 1 {
		t.Fatalf("purga: n=%d err=%v, quería 1", n, err)
	}

	if m, a, p := hasContent(t, st, expired); m != 0 || a != "[]" || !p {
		t.Errorf("caducado: mensajes=%d answers=%s purgado=%v", m, a, p)
	}
	for name, id := range map[string]int64{"reciente": fresh, "abierto": open} {
		if m, a, p := hasContent(t, st, id); m != 1 || a == "[]" || p {
			t.Errorf("%s no debía tocarse: mensajes=%d answers=%s purgado=%v", name, m, a, p)
		}
	}

	// El registro del ticket se conserva para las estadísticas.
	var rows int
	if err := st.pool.QueryRow(ctx, `SELECT count(*) FROM tickets`).Scan(&rows); err != nil || rows != 3 {
		t.Errorf("tickets = %d (err %v), quería 3", rows, err)
	}

	// Repetirla no encuentra nada más.
	if n, err := st.PurgeExpiredTranscripts(ctx, time.Now().Add(-TranscriptRetention), 500); err != nil || n != 0 {
		t.Errorf("segunda purga: n=%d err=%v, quería 0", n, err)
	}
}

func TestPurgeRespectsBatchLimit(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	old := 100 * 24 * time.Hour
	for i := 1; i <= 3; i++ {
		insertTicket(t, st, "g1", i, &old)
	}
	cutoff := time.Now().Add(-TranscriptRetention)
	if n, _ := st.PurgeExpiredTranscripts(ctx, cutoff, 2); n != 2 {
		t.Errorf("primer lote = %d, quería 2", n)
	}
	if n, _ := st.PurgeExpiredTranscripts(ctx, cutoff, 2); n != 1 {
		t.Errorf("segundo lote = %d, quería 1", n)
	}
}

func TestDeleteTranscript(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	recent := 5 * 24 * time.Hour
	closed := insertTicket(t, st, "g1", 1, &recent)
	open := insertTicket(t, st, "g1", 2, nil)

	if err := st.DeleteTranscript(ctx, "g2", closed); !errors.Is(err, ErrNotFound) {
		t.Errorf("otro servidor: err=%v, quería ErrNotFound", err)
	}
	if err := st.DeleteTranscript(ctx, "g1", open); !errors.Is(err, ErrNotFound) {
		t.Errorf("ticket abierto: err=%v, quería ErrNotFound", err)
	}
	if err := st.DeleteTranscript(ctx, "g1", closed); err != nil {
		t.Fatal(err)
	}
	if m, a, p := hasContent(t, st, closed); m != 0 || a != "[]" || !p {
		t.Errorf("tras borrar: mensajes=%d answers=%s purgado=%v", m, a, p)
	}
	// Ya no aparece en la lista de transcripciones.
	list, err := st.ListTranscripts(ctx, "g1", TranscriptFilter{}, 0, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		if it.ID == closed {
			t.Error("el ticket borrado sigue en la lista")
		}
	}
}

func TestSaveTicketMessagesAfterPurge(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	recent := 5 * 24 * time.Hour
	id := insertTicket(t, st, "g1", 1, &recent)
	if err := st.DeleteTranscript(ctx, "g1", id); err != nil {
		t.Fatal(err)
	}

	msg := TicketMessage{ID: "otro", AuthorID: "u1", AuthorName: "Ana", Content: "de nuevo", CreatedAt: time.Now()}
	if err := st.SaveTicketMessages(ctx, id, []TicketMessage{msg}); !errors.Is(err, ErrTranscriptPurged) {
		t.Errorf("err=%v, quería ErrTranscriptPurged", err)
	}
	if m, _, _ := hasContent(t, st, id); m != 0 {
		t.Errorf("se regeneraron %d mensajes", m)
	}
}

func TestTranscriptExpired(t *testing.T) {
	now := time.Now()
	ago := func(d time.Duration) *time.Time { c := now.Add(-d); return &c }
	cases := map[string]Ticket{
		"purgado":       {ContentPurgedAt: ago(time.Hour)},
		"pasó el plazo": {ClosedAt: ago(TranscriptRetention + time.Hour)},
	}
	for name, tk := range cases {
		if !tk.TranscriptExpired(now) {
			t.Errorf("%s: debía estar caducado", name)
		}
	}
	if (Ticket{ClosedAt: ago(time.Hour)}).TranscriptExpired(now) {
		t.Error("cerrado hace una hora no debía estar caducado")
	}
	if (Ticket{}).TranscriptExpired(now) {
		t.Error("abierto no debía estar caducado")
	}
}

func TestDeleteGuildDataRemovesAISettings(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	for _, g := range []string{"g1", "g2"} {
		if err := st.SaveAISettings(ctx, g, AISettings{BaseURL: "https://x", Model: "m", KeyEnc: []byte("cifrada")}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.DeleteGuildData(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if a, _ := st.GetAISettings(ctx, "g1"); a != nil {
		t.Error("al quitar el bot debía borrarse la API key del servidor")
	}
	if a, _ := st.GetAISettings(ctx, "g2"); a == nil {
		t.Error("se borró la conexión de otro servidor")
	}
}
