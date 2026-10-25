package ai

import "testing"

func TestCleanText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"vacío", "", ""},
		{"solo espacios", "  \n\t \n ", ""},
		{"recorta los extremos", "  hola \n", "hola"},
		{"saltos de Windows", "a\r\nb\r\nc", "a\nb\nc"},
		{"saltos de Mac antiguo", "a\rb", "a\nb"},
		{"espacios al final de línea", "a   \nb\t\t\nc", "a\nb\nc"},
		{"una línea en blanco como máximo", "a\n\n\n\n\nb", "a\n\nb"},
		{"respeta una línea en blanco", "a\n\nb", "a\n\nb"},
		{"respeta la sangría de markdown", "- a\n    - b\n\t- c", "- a\n    - b\n\t- c"},
		{"quita NUL", "a\x00b", "ab"},
		{"quita caracteres de control", "a\x07b\x1bc", "abc"},
		{"conserva tabuladores y emojis", "a\tb 🙂 ñ", "a\tb 🙂 ñ"},
		{"conserva el markdown", "# Título\n\n**negrita** y `código`", "# Título\n\n**negrita** y `código`"},
	}
	for _, c := range cases {
		if got := CleanText(c.in); got != c.want {
			t.Errorf("%s: %q, quería %q", c.name, got, c.want)
		}
	}
}

func TestCleanTextIsIdempotent(t *testing.T) {
	in := "  # A\r\n\r\n\r\n- x   \r\n\x00 y  "
	once := CleanText(in)
	if twice := CleanText(once); twice != once {
		t.Errorf("limpiar dos veces cambia el resultado: %q -> %q", once, twice)
	}
}
