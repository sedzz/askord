package ai

import (
	"regexp"
	"strings"
	"unicode"
)

// Límites, en caracteres, de los textos que escribe el admin. Van a cada petición al modelo,
// así que acotan lo que cuesta cada mensaje.
const (
	MaxGuidelines   = 4000 // pautas generales del servidor
	MaxInstructions = 2000 // instrucciones de un tipo de ticket
	MaxEscalation   = 1000 // cuándo pasar un ticket de un tipo a una persona
)

var (
	trailingSpace = regexp.MustCompile(`(?m)[ \t]+$`)
	blankLines    = regexp.MustCompile(`\n{3,}`)
)

// CleanText normaliza un texto escrito por el admin antes de guardarlo: saltos de línea
// uniformes, sin espacios sobrantes al final de las líneas ni más de una línea en blanco
// seguida, y sin caracteres de control (un NUL haría fallar a Postgres). La sangría y el
// resto de espacios se respetan, porque en markdown significan algo.
func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	s = trailingSpace.ReplaceAllString(s, "")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
