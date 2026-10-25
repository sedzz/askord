package store

import (
	"context"
	"errors"
	"testing"
)

func TestAIBehavior(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// Sin conexión no hay comportamiento que guardar.
	if err := st.SaveAIBehavior(ctx, "g1", true, "pautas"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sin conexión: %v, quería ErrNotFound", err)
	}

	conn := AISettings{BaseURL: "https://x/v1", Model: "m", KeyEnc: []byte("cifrada")}
	if err := st.SaveAISettings(ctx, "g1", conn); err != nil {
		t.Fatal(err)
	}
	if a, _ := st.GetAISettings(ctx, "g1"); a.Enabled || a.Guidelines != "" {
		t.Errorf("una conexión nueva debe empezar apagada y sin pautas: %+v", a)
	}

	if err := st.SaveAIBehavior(ctx, "g1", true, "sé amable"); err != nil {
		t.Fatal(err)
	}
	// Editar la conexión conserva interruptor, pautas y, sin clave nueva, la clave.
	conn.Model, conn.KeyEnc = "m2", nil
	if err := st.SaveAISettings(ctx, "g1", conn); err != nil {
		t.Fatal(err)
	}
	a, err := st.GetAISettings(ctx, "g1")
	if err != nil || !a.Enabled || a.Guidelines != "sé amable" || a.Model != "m2" || string(a.KeyEnc) != "cifrada" {
		t.Errorf("tras editar la conexión: %+v (%v)", a, err)
	}

	// Cada servidor tiene lo suyo.
	if err := st.SaveAISettings(ctx, "g2", conn); err != nil {
		t.Fatal(err)
	}
	if other, _ := st.GetAISettings(ctx, "g2"); other.Enabled || other.Guidelines != "" {
		t.Errorf("g2 heredó el comportamiento de g1: %+v", other)
	}
}

func TestTicketTypeAIFields(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	created, err := st.CreateTicketType(ctx, "g1", TicketTypeInput{
		Name: "Compras", AIEnabled: true, AIInstructions: "Ayuda con pedidos.", AIEscalation: "Si piden un reembolso.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.AIEnabled || created.AIInstructions != "Ayuda con pedidos." || created.AIEscalation != "Si piden un reembolso." {
		t.Errorf("al crear: %+v", created)
	}

	// Por defecto un tipo no usa IA.
	plain, err := st.CreateTicketType(ctx, "g1", TicketTypeInput{Name: "General"})
	if err != nil || plain.AIEnabled || plain.AIInstructions != "" || plain.AIEscalation != "" {
		t.Errorf("tipo sin IA: %+v (%v)", plain, err)
	}

	updated, err := st.UpdateTicketType(ctx, "g1", created.ID, TicketTypeInput{Name: "Compras", AIEnabled: false, AIInstructions: "Nuevas."})
	if err != nil || updated.AIEnabled || updated.AIInstructions != "Nuevas." || updated.AIEscalation != "" {
		t.Errorf("al editar: %+v (%v)", updated, err)
	}

	got, err := st.GetTicketType(ctx, "g1", created.ID)
	if err != nil || got.AIInstructions != "Nuevas." {
		t.Errorf("al leer: %+v (%v)", got, err)
	}
	list, err := st.ListTicketTypes(ctx, "g1")
	if err != nil || len(list) != 2 {
		t.Fatalf("lista: %d (%v)", len(list), err)
	}
}
