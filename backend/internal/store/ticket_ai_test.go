package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// newAITicket crea un ticket abierto del servidor g1 con respuestas de formulario.
func newAITicket(t *testing.T, st *Store, channel string) {
	t.Helper()
	err := st.CreateTicket(context.Background(), Ticket{
		GuildID: "g1", Number: 1, ChannelID: channel, OpenerID: "ana",
		Answers: []Answer{{Question: "¿Qué ocurre?", Answer: "No me llegó"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func claim(t *testing.T, st *Store, channel string, max int) ClaimResult {
	t.Helper()
	r, err := st.ClaimAIReply(context.Background(), channel, max)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTicketStartsWithActiveAssistant(t *testing.T) {
	st := newTestStore(t)
	newAITicket(t, st, "c1")

	tk, status, err := st.TicketByChannel(context.Background(), "c1")
	if err != nil || tk == nil || status != "open" {
		t.Fatalf("ticket = %+v (%s, %v)", tk, status, err)
	}
	if tk.AIState != AIStateActive || tk.AIReplies != 0 {
		t.Errorf("estado inicial = %q con %d respuestas", tk.AIState, tk.AIReplies)
	}
	// El asistente necesita las respuestas del formulario.
	if len(tk.Answers) != 1 || tk.Answers[0].Answer != "No me llegó" {
		t.Errorf("respuestas = %+v", tk.Answers)
	}
}

func TestClaimAIReplyStopsAtTheLimit(t *testing.T) {
	st := newTestStore(t)
	newAITicket(t, st, "c1")

	for i := 1; i <= 3; i++ {
		if got := claim(t, st, "c1", 3); got != ClaimGranted {
			t.Fatalf("respuesta %d: %v, quería ClaimGranted", i, got)
		}
	}
	for range 2 { // repetir no concede ni suma
		if got := claim(t, st, "c1", 3); got != ClaimLimitReached {
			t.Errorf("tras el tope: %v, quería ClaimLimitReached", got)
		}
	}
	if tk, _, _ := st.TicketByChannel(context.Background(), "c1"); tk.AIReplies != 3 {
		t.Errorf("respuestas contadas = %d, quería 3", tk.AIReplies)
	}
}

// Con muchos mensajes a la vez, solo se conceden tantas respuestas como el tope.
func TestClaimAIReplyIsAtomic(t *testing.T) {
	st := newTestStore(t)
	newAITicket(t, st, "c1")

	const limit, attempts = 20, 80
	var granted, limited atomic.Int32
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch r, err := st.ClaimAIReply(context.Background(), "c1", limit); {
			case err != nil:
				t.Error(err)
			case r == ClaimGranted:
				granted.Add(1)
			case r == ClaimLimitReached:
				limited.Add(1)
			}
		}()
	}
	wg.Wait()

	if granted.Load() != limit || limited.Load() != attempts-limit {
		t.Errorf("concedidas %d y rechazadas %d; quería %d y %d", granted.Load(), limited.Load(), limit, attempts-limit)
	}
	if tk, _, _ := st.TicketByChannel(context.Background(), "c1"); tk.AIReplies != limit {
		t.Errorf("respuestas contadas = %d, quería %d", tk.AIReplies, limit)
	}
}

func TestClaimAIReplyNeedsAnActiveOpenTicket(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if got := claim(t, st, "no-existe", 20); got != ClaimNotActive {
		t.Errorf("ticket inexistente: %v", got)
	}

	newAITicket(t, st, "escalado")
	if ok, _ := st.SetAIState(ctx, "escalado", AIStateEscalated); !ok {
		t.Fatal("no se pudo escalar")
	}
	if got := claim(t, st, "escalado", 20); got != ClaimNotActive {
		t.Errorf("escalado: %v, quería ClaimNotActive", got)
	}

	// Un ticket cerrado no se atiende, aunque el asistente siga "activo".
	if err := st.CreateTicket(ctx, Ticket{GuildID: "g1", Number: 2, ChannelID: "cerrado", OpenerID: "luis"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.CloseTicket(ctx, "cerrado", "luis"); !ok || err != nil {
		t.Fatalf("cerrar: %v %v", ok, err)
	}
	if got := claim(t, st, "cerrado", 20); got != ClaimNotActive {
		t.Errorf("cerrado: %v, quería ClaimNotActive", got)
	}
	// Y un ticket escalado que además llegó al tope sigue siendo "no activo", no "tope".
	if tk, _, _ := st.TicketByChannel(ctx, "cerrado"); tk.AIReplies != 0 {
		t.Errorf("un ticket cerrado no debe contar respuestas: %d", tk.AIReplies)
	}
}

func TestSetAIState(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	newAITicket(t, st, "c1")

	if ok, err := st.SetAIState(ctx, "c1", AIStateEscalated); !ok || err != nil {
		t.Fatalf("primera vez: %v %v", ok, err)
	}
	// Una segunda petición (doble clic, dos llamadas a la herramienta…) no cambia nada.
	if ok, err := st.SetAIState(ctx, "c1", AIStateEscalated); ok || err != nil {
		t.Errorf("segunda vez: %v %v, quería false", ok, err)
	}
	// Ya escalado, una pausa posterior no lo cambia.
	if ok, _ := st.SetAIState(ctx, "c1", AIStatePaused); ok {
		t.Error("un ticket escalado no debe pasar a pausado")
	}
	if tk, _, _ := st.TicketByChannel(ctx, "c1"); tk.AIState != AIStateEscalated {
		t.Errorf("estado = %q, quería escalated", tk.AIState)
	}

	// El staff pausa un ticket activo.
	newAITicketN(t, st, "c2", 2)
	if ok, _ := st.SetAIState(ctx, "c2", AIStatePaused); !ok {
		t.Error("no se pudo pausar")
	}
	if tk, _, _ := st.TicketByChannel(ctx, "c2"); tk.AIState != AIStatePaused {
		t.Errorf("estado = %q, quería paused", tk.AIState)
	}

	// Solo se puede salir del estado activo hacia escalado o pausado.
	newAITicketN(t, st, "c3", 3)
	for _, bad := range []string{AIStateActive, "", "otro"} {
		if ok, err := st.SetAIState(ctx, "c3", bad); ok || err == nil {
			t.Errorf("estado %q: %v %v, debía rechazarse", bad, ok, err)
		}
	}
	if ok, err := st.SetAIState(ctx, "no-existe", AIStateEscalated); ok || err != nil {
		t.Errorf("ticket inexistente: %v %v", ok, err)
	}
}

func newAITicketN(t *testing.T, st *Store, channel string, number int) {
	t.Helper()
	if err := st.CreateTicket(context.Background(), Ticket{GuildID: "g1", Number: number, ChannelID: channel, OpenerID: channel}); err != nil {
		t.Fatal(err)
	}
}

// Solo uno de los que llegan a la vez consigue escalar, y por tanto solo uno avisa al staff.
func TestSetAIStateOnlyOneWins(t *testing.T) {
	st := newTestStore(t)
	newAITicket(t, st, "c1")

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := st.SetAIState(context.Background(), "c1", AIStateEscalated); err != nil {
				t.Error(err)
			} else if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Errorf("%d peticiones consiguieron escalar, quería 1", wins.Load())
	}
}
