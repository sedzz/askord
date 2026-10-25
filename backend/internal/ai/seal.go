// Package ai conecta Askord con LLMs compatibles con la API de OpenAI.
package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// ErrCannotOpen indica que un valor cifrado no se pudo descifrar: la clave cambió, el dato
// se corrompió o pertenece a otro servidor.
var ErrCannotOpen = errors.New("no se pudo descifrar")

// Sealer cifra secretos (las API keys) con AES-256-GCM.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer crea un Sealer con una clave de 32 bytes.
func NewSealer(key []byte) (*Sealer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("clave de cifrado: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal cifra plaintext. context (el servidor) queda ligado al resultado: un valor cifrado
// para un servidor no se puede abrir como si fuera de otro. El resultado lleva el nonce delante.
func (s *Sealer) Seal(plaintext, context string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(plaintext), []byte(context)), nil
}

// Open descifra lo que devolvió Seal con el mismo context.
func (s *Sealer) Open(sealed []byte, context string) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n+s.aead.Overhead() {
		return "", ErrCannotOpen
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], []byte(context))
	if err != nil {
		return "", ErrCannotOpen
	}
	return string(plain), nil
}
