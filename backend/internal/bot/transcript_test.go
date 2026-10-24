package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestReadableContent(t *testing.T) {
	m := &discordgo.Message{
		Content:      "Cerrado por <@1> y <@!2>, avisa a <@&9> (<@&404>)",
		Mentions:     []*discordgo.User{{ID: "1", Username: "sedz"}, {ID: "2", Username: "marcus", GlobalName: "Marcus"}},
		MentionRoles: []string{"9"},
	}
	got := readableContent(m, map[string]string{"9": "Soporte"})
	// <@&404> no está en la lista de roles conocidos: se deja como llegó.
	want := "Cerrado por @sedz y @Marcus, avisa a @Soporte (<@&404>)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if got := readableContent(&discordgo.Message{Content: "sin menciones"}, nil); got != "sin menciones" {
		t.Errorf("sin menciones debe quedar igual, got %q", got)
	}
}
