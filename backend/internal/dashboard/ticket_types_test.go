package dashboard

import (
	"strings"
	"testing"

	"github.com/sedzz/askord/internal/store"
)

func TestCleanQuestions(t *testing.T) {
	got, problem := cleanQuestions([]store.Question{{Label: "  Tu pedido  ", Required: true}, {Label: "Detalle", Long: true}})
	if problem != "" || len(got) != 2 || got[0].Label != "Tu pedido" || !got[0].Required || !got[1].Long {
		t.Errorf("preguntas válidas: %+v %q", got, problem)
	}

	if got, problem := cleanQuestions(nil); problem != "" || got == nil || len(got) != 0 {
		t.Errorf("sin preguntas debe dar una lista vacía (no nil): %+v %q", got, problem)
	}

	tooMany := make([]store.Question, 6)
	for i := range tooMany {
		tooMany[i] = store.Question{Label: "P"}
	}
	if _, problem := cleanQuestions(tooMany); problem == "" {
		t.Error("más de 5 preguntas debe rechazarse")
	}
	if _, problem := cleanQuestions([]store.Question{{Label: "   "}}); problem == "" {
		t.Error("una pregunta vacía debe rechazarse")
	}
	if _, problem := cleanQuestions([]store.Question{{Label: strings.Repeat("x", 46)}}); problem == "" {
		t.Error("una pregunta de más de 45 caracteres debe rechazarse")
	}
}
