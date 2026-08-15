package batchvalidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSourcePositionsAndLineEndings(t *testing.T) {
	source := newSource("mixed.cmd", []byte("a\r\nb\nc\rd"))
	want := []Position{
		{Offset: 0, Line: 1, Column: 1},
		{Offset: 3, Line: 2, Column: 1},
		{Offset: 5, Line: 3, Column: 1},
		{Offset: 7, Line: 4, Column: 1},
	}
	for _, position := range want {
		if got := source.position(position.Offset); got != position {
			t.Fatalf("position(%d) = %#v, want %#v", position.Offset, got, position)
		}
	}
	if got := source.position(-10); got.Offset != 0 {
		t.Fatalf("negative offset was not clamped: %#v", got)
	}
	if got := source.position(100); got.Offset != len(source.Bytes) {
		t.Fatalf("large offset was not clamped: %#v", got)
	}
	if got := source.span(4, 2); got.End.Offset != 4 {
		t.Fatalf("reversed span was not clamped: %#v", got)
	}
}

func TestPhysicalLines(t *testing.T) {
	if got := physicalLines(nil); got != nil {
		t.Fatalf("empty source lines = %#v", got)
	}
	got := physicalLines([]byte("a\r\nb\nc\rd"))
	want := []physicalLine{{0, 1, 3}, {3, 4, 5}, {5, 6, 7}, {7, 8, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("physical lines = %#v, want %#v", got, want)
	}
	if got := physicalLines([]byte("a\n")); len(got) != 1 {
		t.Fatalf("trailing newline created an extra physical line: %#v", got)
	}
}

func TestScannerRecognizesSyntaxAndMakesProgress(t *testing.T) {
	src := []byte(" @:() & && | || < > >> <& >& ^& ^ \"(&)\" % !\r\ntext")
	tokens := scan(src)
	seen := map[tokenKind]bool{}
	previous := 0
	for _, current := range tokens {
		if current.start < previous || current.end <= current.start || current.end > len(src) {
			t.Fatalf("invalid token progress: %#v after %d", current, previous)
		}
		previous = current.end
		seen[current.kind] = true
	}
	for _, kind := range []tokenKind{
		tokenText, tokenWhitespace, tokenNewline, tokenEscaped, tokenQuote,
		tokenAt, tokenColon, tokenLParen, tokenRParen, tokenAmp, tokenAnd,
		tokenPipe, tokenOr, tokenLess, tokenGreater, tokenAppend,
		tokenLessAmp, tokenGreaterAmp, tokenPercent, tokenBang,
	} {
		if !seen[kind] {
			t.Errorf("scanner did not emit token kind %d", kind)
		}
	}
	if got := scan([]byte("^")); len(got) != 1 || got[0].kind != tokenEscaped || got[0].end != 1 {
		t.Fatalf("truncated caret token = %#v", got)
	}
}

func TestScannerHelpers(t *testing.T) {
	for _, value := range []byte{'\r', '\n', '^', '"', '%', '!', ' ', '\t', '\v', '\f', '@', ':', '(', ')', '&', '|', '<', '>'} {
		if !scannerBoundary(value) {
			t.Errorf("%q should be a scanner boundary", value)
		}
	}
	if scannerBoundary('x') {
		t.Fatal("ordinary text reported as scanner boundary")
	}
	start, end := trimSpace([]byte("  x \t"), 0, 5)
	if start != 2 || end != 3 {
		t.Fatalf("trimSpace = %d,%d", start, end)
	}
	words := splitWords([]byte("one \"two words\" three^ four"), 10)
	if len(words) != 3 || words[1].text != "\"two words\"" || words[2].start != 26 {
		t.Fatalf("splitWords = %#v", words)
	}
	if got := splitWords([]byte("one   "), 0); len(got) != 1 {
		t.Fatalf("trailing-space words = %#v", got)
	}
	if !equalFoldASCII("MiXeD", "mixed") || equalFoldASCII("a", "bb") || equalFoldASCII("a", "b") {
		t.Fatal("ASCII case comparison failed")
	}
	if got := lowerASCII("A-Z_9"); got != "a-z_9" {
		t.Fatalf("lowerASCII = %q", got)
	}
}

func TestValidateFileAndResultHelpers(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ok.cmd")
	if err := os.WriteFile(path, []byte("echo ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ValidateFile(path, Options{})
	if err != nil || !result.Valid() || result.HasErrors() {
		t.Fatalf("ValidateFile = %#v, %v", result, err)
	}
	if _, err := ValidateFile(filepath.Join(directory, "missing.cmd"), Options{}); err == nil {
		t.Fatal("ValidateFile accepted a missing file")
	}
	bad := ValidateSource("bad.cmd", []byte("goto missing"), Options{})
	if bad.Valid() || !bad.HasErrors() {
		t.Fatalf("bad result helpers = %#v", bad)
	}
}

func TestParseBuildsSourcePreservingAST(t *testing.T) {
	src := []byte(":start\necho one >out && vendor.exe x\n(\necho grouped\n)\n")
	script, result := Parse("ast.cmd", src, Options{ReportUnsupported: true})
	if result.HasErrors() || result.FullyValidated {
		t.Fatalf("parse result = %#v", result)
	}
	if len(script.Statements) != 5 {
		t.Fatalf("statement count = %d: %#v", len(script.Statements), script.Statements)
	}
	label := script.Statements[0]
	if label.Kind != StatementLabel || label.Label != "start" {
		t.Fatalf("label = %#v", label)
	}
	command := script.Statements[1]
	if command.Kind != StatementCommand || command.Chain.First.Name != "echo" || len(command.Chain.First.Redirections) != 1 {
		t.Fatalf("command = %#v", command)
	}
	if len(command.Chain.Rest) != 1 || command.Chain.Rest[0].Op != ChainAnd || !command.Chain.Rest[0].Command.Opaque {
		t.Fatalf("chain = %#v", command.Chain)
	}
	if script.Statements[2].Kind != StatementGroup || script.Statements[4].Kind != StatementGroup {
		t.Fatalf("group statements were not preserved: %#v", script.Statements)
	}
}
