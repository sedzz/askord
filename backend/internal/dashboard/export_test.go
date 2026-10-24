package dashboard

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sedzz/askord/internal/store"
)

func sample() []store.Transcript {
	at := time.Date(2026, 3, 10, 10, 1, 0, 0, time.UTC)
	return []store.Transcript{{
		TranscriptSummary: store.TranscriptSummary{ID: 1, Number: 7, OpenerName: "Ana", Status: "closed", TypeName: "Pagos", TypeEmoji: "💳", CreatedAt: at},
		Conversation: []store.TranscriptMessage{
			{ID: "m1", AuthorID: "u1", AuthorName: "Ana", Content: "hola, \"qué tal\"", CreatedAt: at},
			{ID: "m2", AuthorID: "u2", AuthorName: "=cmd", Content: "=SUM(A1)", Attachments: []string{"http://x/y.png"}, CreatedAt: at},
		},
	}}
}

func TestExportFormats(t *testing.T) {
	for format, want := range map[string][]string{
		"csv":  {"\xEF\xBB\xBFticket,estado", "0007,closed,💳 Pagos,2026-03-10 10:01:00,Ana,u1,\"hola, \"\"qué tal\"\"\"", "'=cmd", "'=SUM(A1)"},
		"txt":  {"Ticket #0007 — abierto por Ana", "Tipo: 💳 Pagos", "[2026-03-10 10:01:00] Ana: hola", "Adjunto: http://x/y.png"},
		"json": {`"number": 7`, `"conversation"`, `"authorName": "Ana"`, `"typeName": "Pagos"`},
	} {
		rec := httptest.NewRecorder()
		writeExport(rec, format, "ticket-0007", sample())
		body := rec.Body.String()
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: falta %q en:\n%s", format, w, body)
			}
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "ticket-0007."+format) {
			t.Errorf("%s: Content-Disposition inesperado: %q", format, cd)
		}
	}

	rec := httptest.NewRecorder()
	writeExport(rec, "xml", "x", sample())
	if rec.Code != 400 {
		t.Errorf("formato invalido debe dar 400, dio %d", rec.Code)
	}
}
