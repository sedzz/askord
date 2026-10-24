package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sedzz/askord/internal/store"
)

const exportTimeLayout = "2006-01-02 15:04:05"

// writeExport envía las transcripciones como descarga, en el formato pedido.
func writeExport(w http.ResponseWriter, format, name string, transcripts []store.Transcript) {
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
		writeCSV(w, transcripts)
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, name))
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(transcripts)
	case "txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, name))
		writeText(w, transcripts)
	default:
		http.Error(w, "formato invalido: usa csv, json o txt", http.StatusBadRequest)
	}
}

// Una fila por mensaje. Lleva BOM para que Excel reconozca el UTF-8.
func writeCSV(w http.ResponseWriter, transcripts []store.Transcript) {
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"ticket", "estado", "tipo", "fecha", "autor", "autor_id", "mensaje", "adjuntos"})
	for _, t := range transcripts {
		for _, m := range t.Conversation {
			_ = cw.Write([]string{
				fmt.Sprintf("%04d", t.Number),
				t.Status,
				safeCell(typeLabel(t.TypeEmoji, t.TypeName)),
				m.CreatedAt.UTC().Format(exportTimeLayout),
				safeCell(m.AuthorName),
				m.AuthorID,
				safeCell(m.Content),
				strings.Join(m.Attachments, " "),
			})
		}
	}
	cw.Flush()
}

// safeCell evita que una hoja de cálculo ejecute como fórmula un texto que empieza por
// = + - @ (inyección de fórmulas en CSV).
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func writeText(w http.ResponseWriter, transcripts []store.Transcript) {
	for i, t := range transcripts {
		if i > 0 {
			fmt.Fprint(w, "\n"+strings.Repeat("=", 60)+"\n\n")
		}
		fmt.Fprintf(w, "Ticket #%04d — abierto por %s\n", t.Number, t.OpenerName)
		if label := typeLabel(t.TypeEmoji, t.TypeName); label != "" {
			fmt.Fprintf(w, "Tipo: %s\n", label)
		}
		fmt.Fprintf(w, "Estado: %s · Creado: %s UTC", t.Status, t.CreatedAt.UTC().Format(exportTimeLayout))
		if t.ClosedAt != nil {
			fmt.Fprintf(w, " · Cerrado: %s UTC", t.ClosedAt.UTC().Format(exportTimeLayout))
		}
		fmt.Fprint(w, "\n\n")
		if len(t.Answers) > 0 {
			fmt.Fprint(w, "Formulario:\n")
			for _, a := range t.Answers {
				fmt.Fprintf(w, "  %s: %s\n", a.Question, a.Answer)
			}
			fmt.Fprint(w, "\n")
		}
		for _, m := range t.Conversation {
			fmt.Fprintf(w, "[%s] %s: %s\n", m.CreatedAt.UTC().Format(exportTimeLayout), m.AuthorName, m.Content)
			for _, a := range m.Attachments {
				fmt.Fprintf(w, "    Adjunto: %s\n", a)
			}
		}
	}
}

// typeLabel junta emoji y nombre del tipo; vacío si el ticket no tiene tipo.
func typeLabel(emoji, name string) string {
	return strings.TrimSpace(emoji + " " + name)
}
