package dayzinit

import (
	"strings"
	"testing"
)

func TestLexerTokenCoverage(t *testing.T) {
	input := "//! doc\r\n/** block */ identifier _two é3 12 0x2A .5 1.25e-2F true null " +
		`"a\n\t\"\\\x41\u0042" ` +
		"// line\n" +
		">>>= <<= >>= ++ -- += -= *= /= %= &= |= ^= == != <= >= && || << >> :: -> " +
		"{}()[];,.:?~+-*/%&|^!<>="
	tokens, found := lexText(input)
	if len(found) != 0 {
		t.Fatalf("lexer diagnostics: %#v", found)
	}
	if tokens[0].kind != tokenIdentifier || tokens[0].text != "identifier" {
		t.Fatalf("first token = %#v", tokens[0])
	}
	joined := tokenTexts(tokens)
	for _, expected := range []string{"é3", "0x2A", ".5", "1.25e-2F", `"a\n\t\"\\\x41\u0042"`, ">>>=", "::", "->", "="} {
		assertContains(t, joined, expected)
	}
	if tokens[len(tokens)-1].kind != tokenEOF {
		t.Fatal("last token is not EOF")
	}
}

func TestLexerDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		text string
		code string
	}{
		{name: "comment", text: "/* open", code: "DZI1101"},
		{name: "hex", text: "0x;", code: "DZI1102"},
		{name: "exponent", text: "1e+;", code: "DZI1103"},
		{name: "string newline", text: "\"open\n", code: "DZI1104"},
		{name: "string eof", text: "\"open", code: "DZI1104"},
		{name: "escape eof", text: "\"open\\", code: "DZI1105"},
		{name: "escape", text: "\"bad\\q\"", code: "DZI1106"},
		{name: "short hex escape", text: "\"bad\\x\"", code: "DZI1106"},
		{name: "short hex escape eof", text: "\"bad\\x", code: "DZI1106"},
		{name: "invalid hex escape digit", text: "\"bad\\xG0\"", code: "DZI1106"},
		{name: "character", text: "@", code: "DZI1107"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, found := lexText(test.text)
			if !diagnosticsContain(found, test.code) {
				t.Fatalf("diagnostics %#v do not contain %s", found, test.code)
			}
		})
	}
}

func TestLexerHelpers(t *testing.T) {
	if !isSpaceByte('\f') || isSpaceByte('x') {
		t.Fatal("isSpaceByte")
	}
	if !isHexByte('A') || !isHexByte('9') || isHexByte('z') {
		t.Fatal("isHexByte")
	}
	if !isIdentifierStartRune('_') || !isIdentifierStartRune('é') || isIdentifierStartRune('1') {
		t.Fatal("isIdentifierStartRune")
	}
	if !isIdentifierContinueRune('1') || isIdentifierContinueRune('-') {
		t.Fatal("isIdentifierContinueRune")
	}
	lexer := lexerState{data: []byte("x"), offset: 1}
	if lexer.peekByte(0) != 0 {
		t.Fatal("peekByte past end")
	}
}

func FuzzLexer(f *testing.F) {
	f.Add([]byte(`void main() { string value = "ok"; }`))
	f.Add([]byte("/*\xff"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if firstInvalidUTF8(data) >= 0 {
			return
		}
		source := newSourceFile("init.c", data)
		_, _ = lex(source, data)
	})
}

func lexText(text string) ([]token, []Diagnostic) {
	data := []byte(text)
	return lex(newSourceFile("init.c", data), data)
}

func tokenTexts(tokens []token) string {
	var values []string
	for _, item := range tokens {
		values = append(values, item.text)
	}
	return strings.Join(values, " ")
}

func diagnosticsContain(items []Diagnostic, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
