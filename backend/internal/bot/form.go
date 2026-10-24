package bot

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

const customIDFormPrefix = "ticket:form:"

// Límites de Discord para un formulario (modal).
const (
	maxModalTitle   = 45
	maxEmbedField   = 1000 // un campo de embed admite 1024
	shortAnswerMax  = 200
	longAnswerMax   = 1000
	fieldIDTemplate = "q" // los campos se llaman q0, q1…
)

func formCustomID(typeID int64) string {
	return customIDFormPrefix + strconv.FormatInt(typeID, 10)
}

// parseFormID lee el tipo de un identificador "ticket:form:<tipo>".
func parseFormID(customID string) (int64, bool) {
	rest, ok := strings.CutPrefix(customID, customIDFormPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	return id, err == nil && id >= 0
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}

// formModal construye el formulario de un tipo: un campo por pregunta.
func formModal(t store.TicketType) *discordgo.InteractionResponseData {
	rows := make([]discordgo.MessageComponent, 0, len(t.Questions))
	for i, q := range t.Questions {
		style, max := discordgo.TextInputShort, shortAnswerMax
		if q.Long {
			style, max = discordgo.TextInputParagraph, longAnswerMax
		}
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.TextInput{
				CustomID:  fieldIDTemplate + strconv.Itoa(i),
				Label:     truncate(q.Label, maxQuestionLabel),
				Style:     style,
				Required:  q.Required,
				MaxLength: max,
			},
		}})
	}

	title := t.Name
	if title == "" {
		title = "Abrir ticket"
	}
	return &discordgo.InteractionResponseData{
		CustomID:   formCustomID(t.ID),
		Title:      truncate(title, maxModalTitle),
		Components: rows,
	}
}

// maxQuestionLabel es el límite de Discord para la etiqueta de un campo.
const maxQuestionLabel = 45

// formAnswers lee las respuestas del formulario enviado, emparejadas con las preguntas
// actuales del tipo. Las vacías (preguntas opcionales sin contestar) se omiten.
func formAnswers(i *discordgo.InteractionCreate, questions []store.Question) []store.Answer {
	values := map[string]string{}
	for _, row := range i.ModalSubmitData().Components {
		var components []discordgo.MessageComponent
		switch r := row.(type) {
		case *discordgo.ActionsRow:
			components = r.Components
		case discordgo.ActionsRow:
			components = r.Components
		}
		for _, c := range components {
			switch in := c.(type) {
			case *discordgo.TextInput:
				values[in.CustomID] = in.Value
			case discordgo.TextInput:
				values[in.CustomID] = in.Value
			}
		}
	}

	var out []store.Answer
	for idx, q := range questions {
		if v := strings.TrimSpace(values[fieldIDTemplate+strconv.Itoa(idx)]); v != "" {
			out = append(out, store.Answer{Question: q.Label, Answer: v})
		}
	}
	return out
}

// answersEmbed muestra las respuestas en el canal del ticket, para que el staff las vea de entrada.
func answersEmbed(answers []store.Answer) []*discordgo.MessageEmbed {
	if len(answers) == 0 {
		return nil
	}
	fields := make([]*discordgo.MessageEmbedField, 0, len(answers))
	for _, a := range answers {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  truncate(a.Question, 256),
			Value: truncate(a.Answer, maxEmbedField),
		})
	}
	return []*discordgo.MessageEmbed{{Title: "Formulario", Color: 0xFFC5D3, Fields: fields}}
}
