package dayzinit

import (
	"fmt"
	"strings"
	"testing"
)

func TestPreprocessorBranchesAndInterleaving(t *testing.T) {
	sourceText := `#include "scripts/common.c"
#include <scripts/other.c>
#define LOCAL_FEATURE
#ifdef FEATURE
void
#ifdef NESTED
main
#else
main
#endif
()
#else
void main()
#endif
{
#ifdef LOCAL_FEATURE
    int value = 1;
#else
    int value = 2;
#endif
}
#ifndef NEVER_DEFINED
void Optional() {}
#endif
class CustomMission: MissionServer {};
Mission CreateCustomMission(string path) { return new CustomMission; }
`
	if err := ValidateSource("init.c", []byte(sourceText)); err != nil {
		t.Fatalf("conditional mission: %v", err)
	}

	data := []byte("/*\n#ifdef IGNORED\n*/\n\"#else\"; // #endif\n#define 名称\n")
	source := newSourceFile("init.c", data)
	variants, found := analyzePreprocessor(source)
	if len(found) != 0 || len(variants) != 1 {
		t.Fatalf("comment/string directives: variants=%d diagnostics=%#v", len(variants), found)
	}
}

func TestPreprocessorDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		text string
		code string
	}{
		{name: "include", text: "#include nope\n", code: "DZI2001"},
		{name: "define", text: "#define 1BAD\n", code: "DZI2002"},
		{name: "ifdef", text: "#ifdef A B\n#endif\n", code: "DZI2003"},
		{name: "else argument", text: "#ifdef A\n#else nope\n#endif\n", code: "DZI2004"},
		{name: "extra else", text: "#else\n", code: "DZI2005"},
		{name: "duplicate else", text: "#ifdef A\n#else\n#else\n#endif\n", code: "DZI2006"},
		{name: "endif argument", text: "#ifdef A\n#endif nope\n", code: "DZI2007"},
		{name: "extra endif", text: "#endif\n", code: "DZI2008"},
		{name: "unsupported", text: "#if A\n", code: "DZI2009"},
		{name: "missing endif", text: "#ifndef A\n", code: "DZI2010"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := newSourceFile("init.c", []byte(test.text))
			_, found := analyzePreprocessor(source)
			if !diagnosticsContain(found, test.code) {
				t.Fatalf("diagnostics %#v do not contain %s", found, test.code)
			}
		})
	}

	var input strings.Builder
	for index := 0; index <= maxConditionalSymbols; index++ {
		fmt.Fprintf(&input, "#ifdef S%d\n#endif\n", index)
	}
	_, found := analyzePreprocessor(newSourceFile("init.c", []byte(input.String())))
	if !diagnosticsContain(found, "DZI2011") {
		t.Fatalf("branch limit diagnostics = %#v", found)
	}
}

func TestPreprocessorHelpers(t *testing.T) {
	if !validIncludeArgument(`"x"`) || !validIncludeArgument(`<x>`) || validIncludeArgument("x") || validIncludeArgument("") {
		t.Fatal("validIncludeArgument")
	}
	if !validIdentifierText("alpha_2") || !validIdentifierText("名称") || validIdentifierText("2bad") || validIdentifierText("bad-name") || validIdentifierText("") {
		t.Fatal("validIdentifierText")
	}
	if firstCodeOffset([]byte(" // comment"), false) != -1 || firstCodeOffset([]byte(" /* x */ #define A"), false) < 0 || firstCodeOffset([]byte("still comment"), true) != -1 {
		t.Fatal("firstCodeOffset")
	}
	if !blockCommentState([]byte("/* open"), false) || blockCommentState([]byte("*/"), true) || blockCommentState([]byte(`"escaped \" /*" //`), false) {
		t.Fatal("blockCommentState")
	}
}
