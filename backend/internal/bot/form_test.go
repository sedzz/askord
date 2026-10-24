package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

func TestFormModalHasOneFieldPerQuestion(t *testing.T) {
	ty := store.TicketType{ID: 7, Name: strings.Repeat("N", 80), Questions: []store.Question{
		{Label: "Tu pedido", Required: true},
		{Label: "Cuéntanos qué pasó", Required: false, Long: true},
	}}
	data := formModal(ty)

	if data.CustomID != "ticket:form:7" {
		t.Errorf("CustomID = %q", data.CustomID)
	}
	if n := len([]rune(data.Title)); n > maxModalTitle {
		t.Errorf("el título admite %d caracteres y tiene %d", maxModalTitle, n)
	}
	if len(data.Components) != 2 {
		t.Fatalf("debe haber 2 campos, hay %d", len(data.Components))
	}

	first := data.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.TextInput)
	second := data.Components[1].(discordgo.ActionsRow).Components[0].(discordgo.TextInput)
	if first.CustomID != "q0" || first.Style != discordgo.TextInputShort || !first.Required || first.MaxLength != shortAnswerMax {
		t.Errorf("primer campo inesperado: %+v", first)
	}
	if second.CustomID != "q1" || second.Style != discordgo.TextInputParagraph || second.Required || second.MaxLength != longAnswerMax {
		t.Errorf("segundo campo inesperado: %+v", second)
	}
}

func TestParseFormID(t *testing.T) {
	if id, ok := parseFormID("ticket:form:42"); !ok || id != 42 {
		t.Errorf("parseFormID válido: %d %v", id, ok)
	}
	for _, bad := range []string{"ticket:open:42", "ticket:form:", "ticket:form:x", "ticket:form:-1"} {
		if _, ok := parseFormID(bad); ok {
			t.Errorf("%q no debería ser un formulario", bad)
		}
	}
}

func TestFormAnswersPairWithCurrentQuestions(t *testing.T) {
	questions := []store.Question{{Label: "Pedido"}, {Label: "Detalle"}, {Label: "Extra"}}
	i := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type: discordgo.InteractionModalSubmit,
		Data: discordgo.ModalSubmitInteractionData{
			CustomID: "ticket:form:1",
			Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q0", Value: "  A-123  "}}},
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q1", Value: "   "}}},
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q2", Value: "Más info"}}},
			},
		},
	}}

	got := formAnswers(i, questions)
	want := []store.Answer{{Question: "Pedido", Answer: "A-123"}, {Question: "Extra", Answer: "Más info"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAnswersEmbed(t *testing.T) {
	if answersEmbed(nil) != nil {
		t.Error("sin respuestas no hay embed")
	}
	long := strings.Repeat("x", 2000)
	embeds := answersEmbed([]store.Answer{{Question: "P", Answer: long}})
	if len(embeds) != 1 || len([]rune(embeds[0].Fields[0].Value)) > 1024 {
		t.Errorf("el valor de un campo debe caber en 1024 caracteres")
	}
}
