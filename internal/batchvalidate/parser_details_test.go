package batchvalidate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestChainOperatorsAndRedirectionMetadata(t *testing.T) {
	src := []byte("@echo one >out & echo two >>\"two out\" && sort 0<input || echo err 2>&1 | echo piped 0<&3")
	script, result := Parse("shell.cmd", src, Options{})
	if result.HasErrors() {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	chain := script.Statements[0].Chain
	if len(chain.Rest) != 4 {
		t.Fatalf("chain links = %#v", chain.Rest)
	}
	wantOps := []ChainOp{ChainSequence, ChainAnd, ChainOr, ChainPipe}
	for index, link := range chain.Rest {
		if link.Op != wantOps[index] {
			t.Errorf("link %d operator = %d, want %d", index, link.Op, wantOps[index])
		}
	}
	commands := append([]Command{chain.First}, commandLinks(chain.Rest)...)
	wantRedirections := []struct{ handle, operator, target string }{
		{"", ">", "out"}, {"", ">>", "\"two out\""}, {"0", "<", "input"}, {"2", ">&", "1"}, {"0", "<&", "3"},
	}
	redirectionIndex := 0
	for _, command := range commands {
		for _, redirection := range command.Redirections {
			want := wantRedirections[redirectionIndex]
			if redirection.Handle != want.handle || redirection.Operator != want.operator || redirection.Target != want.target {
				t.Errorf("redirection %d = %#v, want %#v", redirectionIndex, redirection, want)
			}
			redirectionIndex++
		}
	}
	if redirectionIndex != len(wantRedirections) {
		t.Fatalf("redirection count = %d", redirectionIndex)
	}
}

func commandLinks(links []CommandLink) []Command {
	commands := make([]Command, len(links))
	for index, link := range links {
		commands[index] = link.Command
	}
	return commands
}

func TestCommandNamesAndGroupLines(t *testing.T) {
	cases := []struct{ input, name, attached string }{
		{"GOTO:EOF", "goto", ":EOF"},
		{"Echo:hello", "echo", ":hello"},
		{"Vendor.EXE", "vendor.exe", ""},
	}
	for _, test := range cases {
		name, attached := commandName(test.input)
		if name != test.name || attached != test.attached {
			t.Errorf("commandName(%q) = %q,%q", test.input, name, attached)
		}
	}
	if !startsElseOnNewLine(" ELSE echo x") || !startsElseOnNewLine(") ELSE(echo x)") || !startsElseOnNewLine(") ELSE") || startsElseOnNewLine(") echo x") {
		t.Fatal("ELSE line recognition failed")
	}
	if delta := parenthesisDelta([]byte("( ^( \"(\" )")); delta != 0 {
		t.Fatalf("parenthesis delta = %d", delta)
	}
	if minInt(1, 2) != 1 || minInt(3, 2) != 2 {
		t.Fatal("minInt failed")
	}
}

func TestMalformedChainAndCommandRecoveryBranches(t *testing.T) {
	script, result := Parse("gaps.cmd", []byte("echo one && && echo two\n(   echo three)\n@  echo four\n@("), Options{})
	if !hasCode(result, "BAT1001") {
		t.Fatalf("gap diagnostics = %#v", result.Diagnostics)
	}
	if len(script.Statements) != 4 || len(script.Statements[0].Chain.Rest) != 1 || script.Statements[0].Chain.Rest[0].Command.Name != "echo" {
		t.Fatalf("recovered script = %#v", script)
	}
}

func TestRedirectionEdgeHelpers(t *testing.T) {
	parser := newBatchParser(newSource("redirect.cmd", []byte("echo ^\n>   out\n>\"a^\"b\"\n>plain^ name")), Options{ReportUnsupported: true})
	clean := []byte("echo ^")
	parser.parseRedirections(clean, 0)
	clean = []byte(">   out")
	if got := parser.parseRedirections(clean, 7); len(got) != 1 || got[0].Target != "out" {
		t.Fatalf("spaced target = %#v", got)
	}
	if got := redirectionTargetEnd([]byte("\"a^\"b\" tail"), 0); got != 6 {
		t.Fatalf("quoted caret target end = %d", got)
	}
	if got := redirectionTargetEnd([]byte("plain^ name tail"), 0); got != 11 {
		t.Fatalf("unquoted caret target end = %d", got)
	}
	if got := redirectionTargetEnd(nil, 0); got != 0 {
		t.Fatalf("empty target end = %d", got)
	}
}

func TestFORParsingHelpers(t *testing.T) {
	if !isThreeLetters("abc") || isThreeLetters("ab") || isThreeLetters("a1c") {
		t.Fatal("isThreeLetters failed")
	}
	if !validForVariableToken("%%A") || validForVariableToken("%A") || validForVariableToken("%%1") {
		t.Fatal("validForVariableToken failed")
	}
	if !singlePercentForVariable("%A") || singlePercentForVariable("%%A") || singlePercentForVariable("%1") {
		t.Fatal("singlePercentForVariable failed")
	}
	if got := matchingParen([]byte("(a (b) \"(\" ^) c) tail"), 0); got != 15 {
		t.Fatalf("matchingParen = %d", got)
	}
	if got := matchingParen([]byte("(missing"), 0); got != -1 {
		t.Fatalf("unmatched parenthesis = %d", got)
	}
	cases := []struct {
		selection string
		count     int
		valid     bool
	}{
		{"1", 1, true}, {"1,3-5,*", 5, true}, {"", 0, false}, {"*", 1, true},
		{"*,1", 0, false}, {"x", 0, false}, {"0", 0, false}, {"2-1", 0, false},
		{"1-x", 0, false}, {"1-2-3", 0, false},
	}
	for _, test := range cases {
		count, valid := forTokenAllocation(test.selection)
		if count != test.count || valid != test.valid {
			t.Errorf("forTokenAllocation(%q) = %d,%v", test.selection, count, valid)
		}
	}
}

func TestNestedFORActiveVariableLimit(t *testing.T) {
	var source strings.Builder
	for index := 0; index < 53; index++ {
		variable := byte('A' + index%26)
		fmt.Fprintf(&source, "for %%%%%c in (*) do (\n", variable)
	}
	source.WriteString("echo body\n")
	for index := 0; index < 53; index++ {
		source.WriteString(")\n")
	}
	result := ValidateSource("nested-for.cmd", []byte(source.String()), Options{})
	if !hasCode(result, "BAT3009") {
		t.Fatalf("nested FOR diagnostics = %#v", result.Diagnostics)
	}
}

func TestExpansionRecognition(t *testing.T) {
	accepted := []string{
		"echo %0 %9 %*",
		"echo %~1 %~f1 %~dp1 %~dp$PATH:1 %~ftza1",
		"for %%A in (*) do echo %%A %%~fA %%~dpA %%~$PATH:A",
	}
	for _, src := range accepted {
		result := ValidateSource("expansion.cmd", []byte(src), Options{ReportUnsupported: true})
		if result.HasErrors() || hasCode(result, "BAT9002") {
			t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
		}
	}
	for _, src := range []string{"echo %", "echo %%", "echo %%1", "echo %~q1", "echo %%~fa"} {
		result := ValidateSource("unsupported-expansion.cmd", []byte(src), Options{ReportUnsupported: true})
		if result.HasErrors() || !hasCode(result, "BAT9002") {
			t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
		}
	}
	result := ValidateSource("quiet.cmd", []byte("echo %RUNTIME%"), Options{})
	if result.FullyValidated || len(result.Diagnostics) != 0 {
		t.Fatalf("quiet unsupported result = %#v", result)
	}
	result = ValidateSource("escaped.cmd", []byte("echo ^% ^! !unclosed"), Options{ReportUnsupported: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("escaped/unclosed expansion diagnostics = %#v", result.Diagnostics)
	}
	result = ValidateSource("truncated-caret.cmd", []byte("echo ^"), Options{ReportUnsupported: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("truncated caret diagnostics = %#v", result.Diagnostics)
	}
}

func TestExpansionAndByteHelpers(t *testing.T) {
	if end, hard, recognized := expansionEnd([]byte("%~*"), 0); end != 3 || !hard || !recognized {
		t.Fatalf("star expansion = %d,%v,%v", end, hard, recognized)
	}
	if end, hard, recognized := modifierExpansionEnd([]byte("%~$BROKEN"), 0, 2, false); end != 9 || hard || recognized {
		t.Fatalf("broken path modifier = %d,%v,%v", end, hard, recognized)
	}
	if end, hard, recognized := modifierExpansionEnd([]byte("%%~f_"), 0, 3, true); end != 5 || hard || !recognized {
		t.Fatalf("non-uppercase FOR variable = %d,%v,%v", end, hard, recognized)
	}
	if !modifierTerminal([]byte("Q"), 0) || modifierTerminal([]byte("f"), 0) || modifierTerminal([]byte("$"), 0) {
		t.Fatal("modifierTerminal failed")
	}
	if lowerByte('A') != 'a' || lowerByte('z') != 'z' {
		t.Fatal("lowerByte failed")
	}
	if indexByte([]byte("abc"), 'b', 0) != 1 || indexByte([]byte("abc"), 'z', 0) != -1 {
		t.Fatal("indexByte failed")
	}
	if !dynamicBytes([]byte("%X%")) || dynamicBytes([]byte("plain")) || !dynamicText("!X!") || dynamicText("plain") {
		t.Fatal("dynamic text helpers failed")
	}
	if !decimalDigits("123") || decimalDigits("") || decimalDigits("1x") || !signedDecimal("-2") || !signedDecimal("+2") || signedDecimal("-") {
		t.Fatal("decimal helpers failed")
	}
}

func TestDiagnosticSortingDeduplicationAndCapState(t *testing.T) {
	parser := newBatchParser(newSource("diagnostics.cmd", []byte("abc")), Options{ReportUnsupported: true})
	second := Diagnostic{Code: "B", Severity: SeverityWarning, Message: "second", Span: parser.source.span(2, 3)}
	first := Diagnostic{Code: "A", Severity: SeverityInfo, Message: "first", Span: parser.source.span(0, 1)}
	parser.addDiagnostic(second)
	parser.addDiagnostic(first)
	parser.addDiagnostic(first)
	parser.addDiagnostic(Diagnostic{Code: "C", Severity: SeverityInfo, Message: "long", Span: parser.source.span(0, 3)})
	parser.addDiagnostic(Diagnostic{Code: "B", Severity: SeverityInfo, Message: "same-span", Span: parser.source.span(0, 1)})
	parser.addDiagnostic(Diagnostic{Code: "B", Severity: SeverityInfo, Message: "z-message", Span: parser.source.span(0, 1)})
	parser.unsupported("BAT9002", "opaque", 1, 2)
	parser.unsupported("BAT9002", "opaque", 1, 2)
	parser.finish()
	want := []string{"A", "B", "B", "C", "BAT9002", "B"}
	if got := diagnosticCodes(Result{Diagnostics: parser.diagnostics}); !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted codes = %#v", got)
	}
	parser.diagnosticLimitHit = true
	before := len(parser.diagnostics)
	parser.addDiagnostic(second)
	if len(parser.diagnostics) != before {
		t.Fatal("diagnostic was added after cap")
	}
}
