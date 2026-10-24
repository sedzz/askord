package bot

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/sedzz/askord/internal/store"
	"github.com/sedzz/askord/internal/store/storetest"
)

const (
	testChannel = "chan-1"
	testOpener  = "ana"
	testStaff   = "staff-role"
)

// seedGuild guarda la configuración de tickets del servidor g1.
func seedGuild(t *testing.T, st *store.Store) {
	t.Helper()
	if err := st.SaveTicketSettings(context.Background(), "g1", store.TicketSettings{CategoryID: "cat", StaffRoleID: testStaff}); err != nil {
		t.Fatal(err)
	}
}

// seedTicket crea el ticket 1 de ana en testChannel; closed lo deja cerrado.
func seedTicket(t *testing.T, st *store.Store, closed bool) {
	t.Helper()
	ctx := context.Background()
	seedGuild(t, st)
	if err := st.CreateTicket(ctx, store.Ticket{GuildID: "g1", Number: 1, ChannelID: testChannel, OpenerID: testOpener}); err != nil {
		t.Fatal(err)
	}
	if closed {
		if ok, err := st.CloseTicket(ctx, testChannel, testOpener); err != nil || !ok {
			t.Fatalf("cerrar ticket: %v %v", ok, err)
		}
	}
}

func status(t *testing.T, st *store.Store) string {
	t.Helper()
	_, s, err := st.TicketByChannel(context.Background(), testChannel)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func lastReply(t *testing.T, f *fakeDiscord) string {
	t.Helper()
	r := f.replies()
	if len(r) == 0 {
		t.Fatal("el bot no respondió nada")
	}
	return r[len(r)-1]
}

func TestCloseTicket(t *testing.T) {
	cases := []struct {
		name   string
		who    string
		roles  []string
		closed bool
		allow  bool   // se espera que se cierre
		reply  string // texto que debe contener la respuesta final
	}{
		{"quien lo abrió", testOpener, nil, false, true, "Ticket cerrado"},
		{"el staff", "luis", []string{testStaff}, false, true, "Ticket cerrado"},
		{"un desconocido", "intruso", nil, false, false, "No puedes cerrar"},
		{"ya estaba cerrado", testOpener, nil, true, false, "ya está cerrado"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, f, st := newTestBot(t)
			seedTicket(t, st, c.closed)

			b.onInteraction(nil, click(customIDCloseTicket, testChannel, member(c.who, c.roles...)))

			if got := lastReply(t, f); !strings.Contains(got, c.reply) {
				t.Errorf("respuesta %q, debía contener %q", got, c.reply)
			}
			wantStatus := "open"
			if c.allow || c.closed {
				wantStatus = "closed"
			}
			if got := status(t, st); got != wantStatus {
				t.Errorf("estado %q, quería %q", got, wantStatus)
			}

			revoked := f.find(http.MethodDelete, "/channels/"+testChannel+"/permissions/"+testOpener)
			renamed := f.find(http.MethodPatch, "/channels/"+testChannel)
			if c.allow {
				if len(revoked) != 1 {
					t.Errorf("debía quitar el acceso a quien abrió el ticket (%d peticiones)", len(revoked))
				}
				if len(renamed) != 1 || !strings.Contains(renamed[0].Body, "cerrado-0001") {
					t.Errorf("debía renombrar el canal a cerrado-0001: %v", renamed)
				}
			} else if len(revoked)+len(renamed) != 0 {
				t.Error("un cierre rechazado no debía tocar el canal")
			}
		})
	}
}

const twoMessages = `[
	{"id":"2","type":0,"content":"vale, gracias","author":{"id":"ana","username":"ana"},"timestamp":"2026-01-01T10:05:00Z","attachments":[]},
	{"id":"1","type":0,"content":"hola, necesito ayuda","author":{"id":"ana","username":"ana"},"timestamp":"2026-01-01T10:00:00Z","attachments":[]}
]`

func transcribe(b *Bot, who string, roles ...string) {
	b.onInteraction(nil, click(customIDTranscribeTicket, testChannel, member(who, roles...)))
}

func storedMessages(t *testing.T, st *store.Store) int {
	t.Helper()
	list, err := st.ListTranscripts(context.Background(), "g1", store.TranscriptFilter{}, 0, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		return 0
	}
	return list[0].Messages
}

func TestTranscribeTicket(t *testing.T) {
	b, f, st := newTestBot(t)
	seedTicket(t, st, true)
	f.messages = twoMessages

	transcribe(b, "luis", testStaff)

	if got := lastReply(t, f); !strings.Contains(got, "Transcript guardado (2 mensajes)") {
		t.Errorf("respuesta %q", got)
	}
	if n := storedMessages(t, st); n != 2 {
		t.Errorf("guardó %d mensajes, quería 2", n)
	}

	// Repetirlo no duplica nada.
	transcribe(b, "luis", testStaff)
	if n := storedMessages(t, st); n != 2 {
		t.Errorf("tras repetir hay %d mensajes, quería 2", n)
	}
}

func TestTranscribeRejected(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, st *store.Store)
		who   string
		roles []string
		reply string
	}{
		{"no es staff", func(t *testing.T, st *store.Store) { seedTicket(t, st, true) },
			testOpener, nil, "Solo el staff"},
		{"ticket abierto", func(t *testing.T, st *store.Store) { seedTicket(t, st, false) },
			"luis", []string{testStaff}, "no se puede transcribir"},
		{"contenido eliminado a mano", func(t *testing.T, st *store.Store) {
			seedTicket(t, st, true)
			if err := st.DeleteTranscript(context.Background(), "g1", ticketID(t, st)); err != nil {
				t.Fatal(err)
			}
		}, "luis", []string{testStaff}, "caducó o se eliminó"},
		{"pasó el plazo de retención", func(t *testing.T, st *store.Store) {
			seedTicket(t, st, true)
			storetest.Exec(t, st, `UPDATE tickets SET closed_at = $1`, time.Now().Add(-store.TranscriptRetention-time.Hour))
		}, "luis", []string{testStaff}, "caducó o se eliminó"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, f, st := newTestBot(t)
			c.setup(t, st)
			f.messages = twoMessages

			transcribe(b, c.who, c.roles...)

			if got := lastReply(t, f); !strings.Contains(got, c.reply) {
				t.Errorf("respuesta %q, debía contener %q", got, c.reply)
			}
			if reads := f.find(http.MethodGet, "/messages"); len(reads) != 0 {
				t.Error("no debía leer el historial del canal")
			}
			if n := storedMessages(t, st); n != 0 {
				t.Errorf("se guardaron %d mensajes", n)
			}
		})
	}
}

func ticketID(t *testing.T, st *store.Store) int64 {
	t.Helper()
	tk, _, err := st.TicketByChannel(context.Background(), testChannel)
	if err != nil || tk == nil {
		t.Fatalf("ticket: %v %v", tk, err)
	}
	return tk.ID
}

// seedType crea un tipo de ticket en g1 (con el rol y la categoría del servidor por defecto).
func seedType(t *testing.T, st *store.Store, questions ...store.Question) store.TicketType {
	t.Helper()
	seedGuild(t, st)
	ty, err := st.CreateTicketType(context.Background(), "g1", store.TicketTypeInput{Name: "Soporte", Emoji: "🛠️", Questions: questions})
	if err != nil {
		t.Fatal(err)
	}
	return ty
}

func openButton(typeID int64, who string) *discordgo.InteractionCreate {
	return click(openCustomID(typeID), "panel", member(who))
}

func TestOpenTicketWithoutForm(t *testing.T) {
	b, f, st := newTestBot(t)
	ty := seedType(t, st)

	b.onInteraction(nil, openButton(ty.ID, "ana"))

	if got := lastReply(t, f); !strings.Contains(got, "<#new-channel>") {
		t.Errorf("respuesta %q, debía enlazar al canal nuevo", got)
	}
	created := f.find(http.MethodPost, "/guilds/g1/channels")
	if len(created) != 1 {
		t.Fatalf("canales creados = %d, quería 1", len(created))
	}
	for _, want := range []string{`"name":"ticket-0001"`, `"parent_id":"cat"`} {
		if !strings.Contains(created[0].Body, want) {
			t.Errorf("la petición no contiene %s: %s", want, created[0].Body)
		}
	}
	tk, status, err := st.TicketByChannel(context.Background(), "new-channel")
	if err != nil || tk == nil || status != "open" || tk.OpenerID != "ana" || tk.Number != 1 {
		t.Errorf("ticket guardado = %+v (%s, %v)", tk, status, err)
	}
	if welcome := f.find(http.MethodPost, "/channels/new-channel/messages"); len(welcome) != 1 {
		t.Errorf("mensajes de bienvenida = %d, quería 1", len(welcome))
	}
}

func TestOpenTicketWithForm(t *testing.T) {
	b, f, st := newTestBot(t)
	ty := seedType(t, st,
		store.Question{Label: "¿Qué ocurre?", Required: true},
		store.Question{Label: "Detalles", Long: true})

	// Al pulsar el botón se abre el formulario y todavía no se crea nada.
	b.onInteraction(nil, openButton(ty.ID, "ana"))
	if got := f.callbackType(); got != int(discordgo.InteractionResponseModal) {
		t.Errorf("respuesta de tipo %d, quería el formulario (%d)", got, discordgo.InteractionResponseModal)
	}
	if n := len(f.find(http.MethodPost, "/guilds/g1/channels")); n != 0 {
		t.Fatalf("se creó un canal antes de enviar el formulario (%d)", n)
	}

	// Al enviarlo se crea el ticket con las respuestas; la pregunta opcional vacía se omite.
	f.reset()
	b.onInteraction(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID: "int-2", AppID: "app", Token: "tok2", Type: discordgo.InteractionModalSubmit,
		GuildID: "g1", ChannelID: "panel", Member: member("ana"),
		Data: discordgo.ModalSubmitInteractionData{CustomID: formCustomID(ty.ID), Components: []discordgo.MessageComponent{
			&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q0", Value: "No puedo entrar"}}},
			&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "q1", Value: "  "}}},
		}},
	}})

	if got := lastReply(t, f); !strings.Contains(got, "<#new-channel>") {
		t.Errorf("respuesta %q, debía enlazar al canal nuevo", got)
	}
	tk, _, err := st.TicketByChannel(context.Background(), "new-channel")
	if err != nil || tk == nil {
		t.Fatalf("ticket no guardado: %v", err)
	}
	if tk.TypeID != ty.ID {
		t.Errorf("tipo guardado %d, quería %d", tk.TypeID, ty.ID)
	}
	// El staff ve las respuestas en el canal; la opcional sin contestar no aparece.
	welcome := f.find(http.MethodPost, "/channels/new-channel/messages")
	if len(welcome) != 1 {
		t.Fatalf("mensajes de bienvenida = %d, quería 1", len(welcome))
	}
	for _, want := range []string{"¿Qué ocurre?", "No puedo entrar"} {
		if !strings.Contains(welcome[0].Body, want) {
			t.Errorf("la bienvenida no muestra %q: %s", want, welcome[0].Body)
		}
	}
	if strings.Contains(welcome[0].Body, "Detalles") {
		t.Error("la pregunta sin contestar no debía aparecer")
	}
}

func TestOpenTicketRejected(t *testing.T) {
	t.Run("servidor sin configurar", func(t *testing.T) {
		b, f, _ := newTestBot(t)
		b.onInteraction(nil, openButton(0, "ana"))
		if got := lastReply(t, f); got != msgNotConfigured {
			t.Errorf("respuesta %q", got)
		}
		if n := len(f.find(http.MethodPost, "/guilds/g1/channels")); n != 0 {
			t.Errorf("se creó un canal (%d)", n)
		}
	})

	t.Run("ya tiene un ticket abierto", func(t *testing.T) {
		b, f, st := newTestBot(t)
		ty := seedType(t, st)
		if err := st.CreateTicket(context.Background(), store.Ticket{
			GuildID: "g1", Number: 1, ChannelID: testChannel, OpenerID: "ana", TypeID: ty.ID,
		}); err != nil {
			t.Fatal(err)
		}
		b.onInteraction(nil, openButton(ty.ID, "ana"))
		if got := lastReply(t, f); !strings.Contains(got, "Ya tienes un ticket abierto") || !strings.Contains(got, testChannel) {
			t.Errorf("respuesta %q", got)
		}
		if n := len(f.find(http.MethodPost, "/guilds/g1/channels")); n != 0 {
			t.Errorf("se creó un canal (%d)", n)
		}
	})

	t.Run("su canal anterior ya no existe", func(t *testing.T) {
		b, f, st := newTestBot(t)
		ty := seedType(t, st)
		if err := st.CreateTicket(context.Background(), store.Ticket{
			GuildID: "g1", Number: 99, ChannelID: testChannel, OpenerID: "ana", TypeID: ty.ID,
		}); err != nil {
			t.Fatal(err)
		}
		f.missing["/channels/"+testChannel] = true

		b.onInteraction(nil, openButton(ty.ID, "ana"))

		if got := lastReply(t, f); !strings.Contains(got, "<#new-channel>") {
			t.Errorf("respuesta %q, debía dejar abrir otro", got)
		}
		if got := status(t, st); got != "closed" {
			t.Errorf("el ticket huérfano quedó %q, quería closed", got)
		}
	})

	t.Run("el bot no tiene permisos", func(t *testing.T) {
		b, f, st := newTestBot(t)
		ty := seedType(t, st)
		f.forbidChannelCreate = true
		b.onInteraction(nil, openButton(ty.ID, "ana"))
		if got := lastReply(t, f); got != msgNoPerms {
			t.Errorf("respuesta %q", got)
		}
		if tk, _, _ := st.TicketByChannel(context.Background(), "new-channel"); tk != nil {
			t.Error("se guardó un ticket sin canal")
		}
	})
}
