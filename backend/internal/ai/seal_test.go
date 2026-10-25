package ai

import (
	"bytes"
	"errors"
	"testing"
)

func testSealer(t *testing.T) *Sealer {
	t.Helper()
	s, err := NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealRoundTrip(t *testing.T) {
	s := testSealer(t)
	sealed, err := s.Seal("sk-secreta", "g1")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("sk-secreta")) {
		t.Error("el valor cifrado contiene la clave en claro")
	}
	got, err := s.Open(sealed, "g1")
	if err != nil || got != "sk-secreta" {
		t.Errorf("Open = %q, %v", got, err)
	}

	// Cada cifrado usa un nonce nuevo.
	again, _ := s.Seal("sk-secreta", "g1")
	if bytes.Equal(sealed, again) {
		t.Error("dos cifrados iguales: el nonce no cambia")
	}
}

func TestOpenRejects(t *testing.T) {
	s := testSealer(t)
	sealed, _ := s.Seal("sk-secreta", "g1")

	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	otherKey, _ := NewSealer(bytes.Repeat([]byte{9}, 32))

	cases := map[string]func() (string, error){
		"otro servidor":        func() (string, error) { return s.Open(sealed, "g2") },
		"dato alterado":        func() (string, error) { return s.Open(tampered, "g1") },
		"otra clave":           func() (string, error) { return otherKey.Open(sealed, "g1") },
		"dato demasiado corto": func() (string, error) { return s.Open([]byte("x"), "g1") },
		"vacío":                func() (string, error) { return s.Open(nil, "g1") },
	}
	for name, open := range cases {
		if got, err := open(); !errors.Is(err, ErrCannotOpen) || got != "" {
			t.Errorf("%s: %q, %v; quería ErrCannotOpen", name, got, err)
		}
	}
}

func TestNewSealerRejectsBadKey(t *testing.T) {
	if _, err := NewSealer([]byte("corta")); err == nil {
		t.Error("una clave de 5 bytes debía rechazarse")
	}
}
