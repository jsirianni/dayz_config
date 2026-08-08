package dayzinit

import (
	"fmt"
	"strings"
	"testing"
)

func TestParserComprehensiveValidSyntax(t *testing.T) {
	text := `typedef int Identifier;
[Metadata("mode")]
enum Mode: BaseMode { FIRST = 1, SECOND; };
modded class Derived<Class T>: MissionServer
{
    const int Field = 1, Values[2];
    map<string, array<int>> Nested;
    Derived() { ; }
    void ~Derived() { }
    proto native int NativeCall(int value);
    int Compute(int input, int fallback = 2)
    {
        ref array<string> names = { "one", "two", };
        int total = 0;
        for (int index = 0; index < 2; index++)
            total += index;
        foreach (int index, string value: names)
        {
            if (value == "") continue;
            while (input > 0) { input--; break; }
        }
        switch (input)
        {
            case 0: return (int)fallback;
            case 1:
            default: return total > 0 ? total : fallback;
        }
    }
};
void Assign(out int target) { target = 1; }
void main()
{
    int value = 0;
    Assign(value);
    Derived item = new Derived;
    item.Compute(value, fallback: 3);
    delete item;
}
Mission CreateCustomMission(string path) { return new Derived(); }
`
	program, parseDiagnostics := parseText(t, text)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parse diagnostics: %#v", parseDiagnostics)
	}
	semanticDiagnostics := validateSemantics(newSourceFile("init.c", []byte(text)), program)
	if len(semanticDiagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %#v", semanticDiagnostics)
	}
	contractDiagnostics := validateMissionContract(newSourceFile("init.c", []byte(text)), program)
	if len(contractDiagnostics) != 0 {
		t.Fatalf("contract diagnostics: %#v", contractDiagnostics)
	}
	if program.Declarations[0].Type.String() != "int" {
		t.Fatalf("typedef type = %s", program.Declarations[0].Type.String())
	}
}

func TestParserDiagnosticsAndRecovery(t *testing.T) {
	tests := []struct {
		name string
		text string
		code string
	}{
		{name: "top", text: ";", code: "DZI2101"},
		{name: "expect", text: "class Thing x", code: "DZI2102"},
		{name: "identifier", text: "class 1 {}", code: "DZI2103"},
		{name: "attribute", text: "[attr void main() {}", code: "DZI2104"},
		{name: "type", text: "class Thing { ; }", code: "DZI2105"},
		{name: "class member", text: "class Thing { @ }", code: "DZI1107"},
		{name: "enum separator", text: "enum E { A B }", code: "DZI2107"},
		{name: "function body", text: "void f() x;", code: "DZI2108"},
		{name: "statement", text: "void f() { case }", code: "DZI2102"},
		{name: "switch label", text: "void f() { switch (1) { value; } }", code: "DZI2111"},
		{name: "expression", text: "void f() { int x = ; }", code: "DZI2201"},
		{name: "type parameters", text: "class Thing<T { }", code: "DZI2113"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := []byte(test.text)
			source := newSourceFile("init.c", data)
			tokens, lexical := lex(source, data)
			if len(lexical) != 0 {
				if diagnosticsContain(lexical, test.code) {
					return
				}
				t.Fatalf("lexical diagnostics: %#v", lexical)
			}
			_, found := parse(source, tokens)
			if !diagnosticsContain(found, test.code) {
				t.Fatalf("diagnostics %#v do not contain %s", found, test.code)
			}
		})
	}

	data := []byte("void first( { x; } class Good {}; void second() { int = ; value( ; }")
	source := newSourceFile("init.c", data)
	tokens, _ := lex(source, data)
	_, found := parse(source, tokens)
	if len(found) < 2 || len(found) > maxDiagnostics {
		t.Fatalf("recovery diagnostics = %d", len(found))
	}
}

func TestParserDepthLimit(t *testing.T) {
	text := "void f() {" + strings.Repeat("{", maxParseDepth+2) + strings.Repeat("}", maxParseDepth+2) + "}"
	_, found := parseText(t, text)
	if !diagnosticsContain(found, "DZI2112") {
		t.Fatalf("diagnostics %#v do not contain nesting limit", found)
	}
}

func TestParserHelpers(t *testing.T) {
	if modifierCount([]string{"static", "STATIC", "static"}, "static") != 3 {
		t.Fatal("modifierCount")
	}
	if !startsExpression(token{kind: tokenNumber}) || !startsExpression(token{text: "("}) || startsExpression(token{text: ";"}) {
		t.Fatal("startsExpression")
	}
	if expressionEnd(nil, nil) != 0 || statementEnd(nil, nil) != 0 {
		t.Fatal("nil end helpers")
	}
	reference := typeRef{Name: "map", Arguments: []typeRef{{Name: "string"}, {Name: "array", Arguments: []typeRef{{Name: "int"}}}}, ArrayDepth: 1}
	if reference.String() != "map<string,array<int>>[]" {
		t.Fatalf("type string = %q", reference.String())
	}
}

func TestParserAlternateAndDefensivePaths(t *testing.T) {
	text := `[[Meta("nested")]] int[] GlobalValues;
class Destructor { ~Destructor() {} };
class Generic<A<B<C>>> {};
class MissingBase: {};
enum MissingEnumBase: { VALUE };
typedef ;
array<; brokenGeneric;
void Parameters(int values[2],) {}
void BadParameter(int valid, ;) {}
int first, ;
void Alternatives()
{
    map<string, array<int>> localMap;
    int[] localArray;
    for (;;) { }
    int index = 0;
    for (index = 0; index < 1;) { index++; }
    foreach (: GlobalValues) { }
    Parameters(1,);
    new ;
    (map<string, array<int>>) index;
    (int[]) index;
}`
	_, found := parseText(t, text)
	if len(found) == 0 {
		t.Fatal("malformed alternate syntax returned no diagnostics")
	}

	source := newSourceFile("init.c", []byte(""))
	eof := token{kind: tokenEOF}
	state := parserState{source: source, tokens: []token{eof}, index: 2}
	if state.current().kind != tokenEOF || state.peek(10).kind != tokenEOF {
		t.Fatal("token fallback helpers")
	}
	state.index = 0
	if state.previous().kind != tokenEOF {
		t.Fatal("previous at start")
	}
	state.depth = maxParseDepth
	if expression := state.parseExpression(1); expression != nil || !diagnosticsContain(state.found.items, "DZI2112") {
		t.Fatalf("expression depth diagnostics = %#v", state.found.items)
	}

	classState := parserStateFor("Name { member; }")
	classState.depth = maxParseDepth
	classState.parseClass(0, nil)
	if !diagnosticsContain(classState.found.items, "DZI2112") {
		t.Fatalf("class depth diagnostics = %#v", classState.found.items)
	}
	blockState := parserStateFor("{ value; }")
	blockState.depth = maxParseDepth
	blockState.parseBlock()
	if !diagnosticsContain(blockState.found.items, "DZI2112") {
		t.Fatalf("block depth diagnostics = %#v", blockState.found.items)
	}

	castNoToken := parserState{source: source, tokens: []token{{kind: tokenIdentifier, text: "int"}, {kind: tokenSymbol, text: ")"}}}
	if castNoToken.looksLikeCast() {
		t.Fatal("cast without following expression accepted")
	}
	castWrongClose := parserStateFor("int + value")
	if castWrongClose.looksLikeCast() {
		t.Fatal("cast without close accepted")
	}

	topSync := parserStateFor("value; class Next {};")
	topSync.synchronizeTopLevel()
	topSync.synchronizeTopLevel()
	statementSync := parserStateFor("value; next")
	statementSync.synchronizeStatement()
	enumSync := parserStateFor("value, next")
	enumSync.synchronizeEnum()
}

func FuzzParser(f *testing.F) {
	f.Add([]byte(validMission))
	f.Add([]byte("void f() { (((((; }"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 || firstInvalidUTF8(data) >= 0 {
			return
		}
		source := newSourceFile("init.c", data)
		tokens, lexical := lex(source, data)
		if len(lexical) == 0 {
			_, _ = parse(source, tokens)
		}
	})
}

func parseText(t *testing.T, text string) (*program, []Diagnostic) {
	t.Helper()
	data := []byte(text)
	source := newSourceFile("init.c", data)
	tokens, lexical := lex(source, data)
	if len(lexical) != 0 {
		t.Fatalf("lexical diagnostics: %#v", lexical)
	}
	return parse(source, tokens)
}

func parserStateFor(text string) parserState {
	data := []byte(text)
	source := newSourceFile("init.c", data)
	tokens, _ := lex(source, data)
	return parserState{source: source, tokens: tokens}
}

func diagnosticCodes(items []Diagnostic) string {
	values := make([]string, len(items))
	for index, item := range items {
		values[index] = item.Code
	}
	return fmt.Sprint(values)
}
