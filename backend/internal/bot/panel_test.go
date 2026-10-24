package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
)

func types(n int) []store.TicketType {
	out := make([]store.TicketType, n)
	for i := range out {
		out[i] = store.TicketType{ID: int64(i + 1), Name: "Tipo " + string(rune('A'+i)), Emoji: "💳"}
	}
	return out
}

func TestPanelSingleTypeKeepsTheClassicButton(t *testing.T) {
	_, components := panelMessage(types(1))
	row := components[0].(discordgo.ActionsRow)
	btn := row.Components[0].(discordgo.Button)
	if btn.Label != "Crear ticket" || btn.CustomID != "ticket:open:1" {
		t.Errorf("botón inesperado: %+v", btn)
	}
}

func TestPanelButtonsUpToFiveThenSelect(t *testing.T) {
	_, components := panelMessage(types(5))
	row := components[0].(discordgo.ActionsRow)
	if len(row.Components) != 5 {
		t.Fatalf("con 5 tipos debe haber 5 botones, hay %d", len(row.Components))
	}
	if label := row.Components[2].(discordgo.Button).Label; label != "Tipo C" {
		t.Errorf("con varios tipos el botón lleva el nombre del tipo, got %q", label)
	}

	_, components = panelMessage(types(6))
	menu, ok := components[0].(discordgo.ActionsRow).Components[0].(discordgo.SelectMenu)
	if !ok || len(menu.Options) != 6 || menu.CustomID != customIDSelectType {
		t.Fatalf("con 6 tipos debe haber un desplegable de 6 opciones: %+v", components[0])
	}
	if menu.Options[0].Value != "1" {
		t.Errorf("el valor de la opción es el ID del tipo, got %q", menu.Options[0].Value)
	}
}

func TestPanelWithoutTypesStillHasAButton(t *testing.T) {
	_, components := panelMessage(nil)
	row := components[0].(discordgo.ActionsRow)
	if len(row.Components) != 1 || row.Components[0].(discordgo.Button).CustomID != customIDOpenTicket {
		t.Errorf("sin tipos debe quedar el botón antiguo: %+v", row)
	}
}

func TestParseTypeID(t *testing.T) {
	for id, want := range map[string]int64{
		"ticket:open":     0, // panel antiguo
		"ticket:open:42":  42,
		"ticket:open:abc": 0,
		"ticket:open:-3":  0,
		"ticket:close":    0,
	} {
		if got := parseTypeID(id); got != want {
			t.Errorf("parseTypeID(%q) = %d, want %d", id, got, want)
		}
	}
	if openCustomID(7) != "ticket:open:7" || openCustomID(0) != "ticket:open" {
		t.Error("openCustomID no coincide con parseTypeID")
	}
}

func TestValidEmoji(t *testing.T) {
	for s, want := range map[string]bool{
		"💳":     true,
		"🐛":     true,
		"1️⃣":   true, // dígito con modificadores: un keycap
		"👨‍👩‍👧": true, // secuencia con unión de ancho cero
		"ab":    false,
		"a":     false,
		"1":     false,
		"!":     false,
		"":      false,
		"💳x":    false,
	} {
		if got := ValidEmoji(s); got != want {
			t.Errorf("ValidEmoji(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestPickTypeAndOfType(t *testing.T) {
	list := types(3)
	if got, ok := pickType(list, 0); !ok || got.ID != 1 {
		t.Errorf("con 0 se elige el primero: %+v %v", got, ok)
	}
	if got, ok := pickType(list, 3); !ok || got.ID != 3 {
		t.Errorf("tipo existente: %+v %v", got, ok)
	}
	if _, ok := pickType(list, 99); ok {
		t.Error("un tipo borrado no debe encontrarse")
	}
	if got, ok := pickType(nil, 0); !ok || got.ID != 0 {
		t.Errorf("sin tipos, 0 usa los valores del servidor: %+v %v", got, ok)
	}

	if ofType(list[0], 1) != "" {
		t.Error("con un solo tipo no se nombra")
	}
	if got := ofType(list[0], 3); got != " de tipo **💳 Tipo A**" {
		t.Errorf("ofType = %q", got)
	}
}
