package batchvalidate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func diagnosticCodes(result Result) []string {
	codes := make([]string, len(result.Diagnostics))
	for index, diagnostic := range result.Diagnostics {
		codes[index] = diagnostic.Code
	}
	return codes
}

func hasCode(result Result, code string) bool {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestEveryHardDiagnostic(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts Options
		code string
	}{
		{"leading operator", "&& echo bad", Options{}, "BAT1001"},
		{"trailing operator", "echo bad ||", Options{}, "BAT1001"},
		{"missing group close", "(\necho x", Options{}, "BAT1002"},
		{"unexpected group close", ")", Options{}, "BAT1003"},
		{"if missing condition", "if", Options{}, "BAT2001"},
		{"if only modifiers", "if /i not", Options{}, "BAT2001"},
		{"if missing number", "if errorlevel", Options{}, "BAT2001"},
		{"if bad number", "if errorlevel nope echo bad", Options{}, "BAT2004"},
		{"if empty equality side", "if a== echo bad", Options{}, "BAT2001"},
		{"if bad comparison", "if /i a XYZ b echo bad", Options{}, "BAT2002"},
		{"if missing body", "if exist file.txt", Options{}, "BAT2001"},
		{"if body missing before else", "if exist file.txt else echo no", Options{}, "BAT2001"},
		{"if else body missing", "if exist file.txt echo yes else", Options{}, "BAT2001"},
		{"if block else body missing", "if exist file.txt (\necho yes\n) else", Options{}, "BAT2001"},
		{"if missing exist operand", "if exist", Options{}, "BAT2001"},
		{"if short comparison", "if left right", Options{}, "BAT2001"},
		{"if nonword operator", "if left + right echo bad", Options{}, "BAT2001"},
		{"if displaced else", "if exist x echo yes\nelse echo no", Options{}, "BAT2003"},
		{"for missing variable", "for", Options{}, "BAT3001"},
		{"for in without variable", "for in (*) do echo", Options{}, "BAT3001"},
		{"for missing in", "for %%A (*) do echo %%A", Options{}, "BAT3002"},
		{"for single percent", "for %A in (*) do echo %A", Options{}, "BAT3001"},
		{"for missing set open", "for %%A in * do echo %%A", Options{}, "BAT3003"},
		{"for missing set close", "for %%A in (* do echo %%A", Options{}, "BAT3003"},
		{"for missing do", "for %%A in (*) echo %%A", Options{}, "BAT3004"},
		{"for missing body", "for %%A in (*) do", Options{}, "BAT3007"},
		{"for bad mode", "for /x %%A in (*) do echo %%A", Options{}, "BAT3006"},
		{"for bad l tuple", "for /l %%A in (1,2) do echo %%A", Options{}, "BAT3005"},
		{"for bad l number", "for /l %%A in (1,x,3) do echo %%A", Options{}, "BAT3005"},
		{"for f eol", "for /f \"eol=ab\" %%A in (x) do echo %%A", Options{}, "BAT3006"},
		{"for f skip", "for /f \"skip=x\" %%A in (x) do echo %%A", Options{}, "BAT3006"},
		{"for f tokens", "for /f \"tokens=2-1\" %%A in (x) do echo %%A", Options{}, "BAT3006"},
		{"for f keyword", "for /f \"mystery=x\" %%A in (x) do echo %%A", Options{}, "BAT3006"},
		{"for allocation boundary", "for /f \"tokens=1,2\" %%Z in (x) do echo %%Z", Options{}, "BAT3008"},
		{"set p assignment", "set /p NAME", Options{}, "BAT4001"},
		{"set a empty", "set /a", Options{}, "BAT4002"},
		{"set a operand", "set /a \"A=1+\"", Options{}, "BAT4003"},
		{"set a lvalue", "set /a \"1=2\"", Options{}, "BAT4003"},
		{"set a close", "set /a \"A=(1+2\"", Options{}, "BAT4003"},
		{"set a bad number", "set /a A=09", Options{}, "BAT4004"},
		{"call missing", "call", Options{}, "BAT5001"},
		{"call empty label", "call :", Options{}, "BAT5001"},
		{"call extension", "call helper.exe", Options{}, "BAT5002"},
		{"modifier star", "echo %~*", Options{}, "BAT5003"},
		{"goto missing", "goto", Options{}, "BAT6001"},
		{"goto absent", "goto missing", Options{}, "BAT6002"},
		{"goto separator", "goto bad=label", Options{}, "BAT6003"},
		{"setlocal option", "setlocal mystery", Options{}, "BAT7001"},
		{"shift missing n", "shift /n", Options{}, "BAT7002"},
		{"shift out of range", "shift /9", Options{}, "BAT7002"},
		{"extensions disabled", "shift /2", Options{InitialCommandExtensions: FeatureDisabled}, "BAT7003"},
		{"redirection missing", "echo x >", Options{}, "BAT8001"},
		{"redirection bad handle", "echo x 2>&x", Options{}, "BAT8002"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := ValidateSource(test.name+".cmd", []byte(test.src), test.opts)
			if !hasCode(result, test.code) {
				t.Fatalf("diagnostics = %#v, want %s", result.Diagnostics, test.code)
			}
			if result.Valid() {
				t.Fatalf("result with %s unexpectedly valid", test.code)
			}
		})
	}
}

func TestDocumentedAcceptedForms(t *testing.T) {
	cases := []string{
		"",
		"\r\n\n\r",
		"@echo off",
		"echo A ^& B & echo C && echo D || echo E | echo F",
		"echo \"(&|<>)\"",
		"rem documented comment",
		"set",
		"set NAME",
		"set NAME=",
		"set NAME=value with spaces",
		"set /p NAME=prompt",
		"set /a A=1+2*3",
		"set /a \"A=(1+2)*3\"",
		"if not errorlevel 1 echo failed",
		"if errorlevel %LEVEL% echo dynamic",
		"if /i A==a echo equal",
		"if A == A echo equal",
		"if exist file.txt (\necho found\n) else (\necho absent\n)",
		"if 1 EQU 1 echo equal",
		"if cmdextversion 1 echo enabled",
		"if defined NAME echo defined",
		"for %%A in (*.txt) do echo %%A",
		"for /d %%A in (*) do echo %%A",
		"for /r C:\\root %%A in (*) do echo %%A",
		"for /l %%A in (5,-1,1) do echo %%A",
		"for /f \"eol=; skip=1 delims=, tokens=1-2,* usebackq\" %%A in (\"a b.txt\") do echo %%A",
		"for /f %%A in (input.txt) do echo %%A",
		"call helper.bat arg",
		"call helper.cmd",
		"call :work\ngoto :EOF\n:work\necho done",
		"goto label\n:LABEL",
		"goto label with spaces\n:label with spaces",
		"setlocal enableextensions enabledelayedexpansion\nendlocal",
		"setlocal disableextensions disabledelayedexpansion\nendlocal",
		"shift",
		"shift /0",
		"shift /n 8",
		"echo x >out.txt",
		"echo x >>\"out file.txt\"",
		"sort 0<input.txt",
		"echo x 2>&1",
		"sort 0<&3",
		"echo ^> ^< ^| ^( ^)",
		"::label-like-line\ngoto :EOF",
		"echo \xff",
	}
	for index, src := range cases {
		t.Run(fmt.Sprintf("case-%02d", index), func(t *testing.T) {
			result := ValidateSource("valid.cmd", []byte(src), Options{InitialCommandExtensions: FeatureEnabled})
			if result.HasErrors() {
				t.Fatalf("%q diagnostics = %#v", src, result.Diagnostics)
			}
		})
	}
}

func TestIndeterminateConstructsAreNotErrors(t *testing.T) {
	cases := []string{
		"vendor-tool.exe --custom={foo:bar}",
		"goto %TARGET%",
		"call %SCRIPT%",
		"call checknew",
		"call :missing",
		"echo %RUNTIME%",
		"echo !DELAYED!",
		"echo \"unmatched",
		"for /l %%A in (%START%,%STEP%,%END%) do echo %%A",
		"for /f \"eol=%E% skip=%N%\" %%A in (x) do echo %%A",
		"shift /n %N%",
		"shift 2",
		"echo x 10>out.txt",
		"echo x 2>&%HANDLE%",
		"setlocal %OPTION%",
		"endlocal extra",
		"echo %~q1",
	}
	for _, src := range cases {
		result := ValidateSource("partial.cmd", []byte(src), Options{ReportUnsupported: true})
		if result.HasErrors() {
			t.Errorf("%q was rejected: %#v", src, result.Diagnostics)
		}
		if result.FullyValidated {
			t.Errorf("%q was unexpectedly fully validated", src)
		}
		if !hasCode(result, "BAT9002") {
			t.Errorf("%q missing unsupported diagnostic: %#v", src, result.Diagnostics)
		}
	}
}

func TestFeatureStateIsConservativeAndScoped(t *testing.T) {
	result := ValidateSource("features.cmd", []byte("setlocal disableextensions & shift /2\nendlocal\nshift /2"), Options{InitialCommandExtensions: FeatureEnabled})
	if got := diagnosticCodes(result); !reflect.DeepEqual(got, []string{"BAT7003"}) {
		t.Fatalf("feature diagnostics = %#v", got)
	}

	result = ValidateSource("merge.cmd", []byte("if exist x echo x\nshift /2"), Options{InitialCommandExtensions: FeatureDisabled})
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "BAT7003" {
			count++
		}
	}
	if count != 0 {
		// IF EXIST itself is not extension-dependent and the merge turns the later
		// state unknown, so there must be no extension-state error.
		t.Fatalf("control-flow merge was not conservative: %#v", result.Diagnostics)
	}
}

func TestInlineValidationBoundaries(t *testing.T) {
	parser := newBatchParser(newSource("inline.cmd", []byte("@vendor")), Options{ReportUnsupported: true})
	parser.validateInline(nil, 0)
	parser.validateInline([]byte("("), 0)
	parser.commandDepth = maxParserDepth
	parser.validateInline([]byte("echo x"), 0)
	parser.commandDepth = 0
	parser.validateInline([]byte("@vendor"), 0)
	result := Result{Diagnostics: parser.diagnostics}
	if !hasCode(result, "BAT9001") || !hasCode(result, "BAT9002") {
		t.Fatalf("inline diagnostics = %#v", parser.diagnostics)
	}
}

func TestStaticTargetsAndEOFFeatureState(t *testing.T) {
	accepted := []string{
		"goto target\n:TARGET",
		"goto %TARGET%",
		"goto :EOF",
		"call :target\n:target",
		"goto repeated\n:repeated\n:repeated",
	}
	for _, src := range accepted {
		if result := ValidateSource("labels.cmd", []byte(src), Options{}); result.HasErrors() {
			t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
		}
	}
	result := ValidateSource("eof-disabled.cmd", []byte("goto :EOF"), Options{InitialCommandExtensions: FeatureDisabled})
	if !hasCode(result, "BAT6002") {
		t.Fatalf("disabled :EOF diagnostic = %#v", result.Diagnostics)
	}
}

func TestRecoveryFindsLaterIndependentErrors(t *testing.T) {
	src := []byte("for %%A (*.txt) do echo bad\necho still-valid\ngoto missing")
	result := ValidateSource("recovery.cmd", src, Options{})
	if !hasCode(result, "BAT3002") || !hasCode(result, "BAT6002") {
		t.Fatalf("recovery diagnostics = %#v", result.Diagnostics)
	}
	for index := 1; index < len(result.Diagnostics); index++ {
		if result.Diagnostics[index-1].Span.Start.Offset > result.Diagnostics[index].Span.Start.Offset {
			t.Fatalf("diagnostics are not source ordered: %#v", result.Diagnostics)
		}
	}
}

func TestValidationDoesNotMutateInputAndIsDeterministic(t *testing.T) {
	src := []byte("echo %VALUE% >out && goto missing\n")
	original := append([]byte(nil), src...)
	options := Options{ReportUnsupported: true}
	first := ValidateSource("stable.cmd", src, options)
	second := ValidateSource("stable.cmd", src, options)
	if !reflect.DeepEqual(src, original) {
		t.Fatalf("source mutated: %q != %q", src, original)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("nondeterministic results:\n%#v\n%#v", first, second)
	}
	for _, diagnostic := range first.Diagnostics {
		if diagnostic.Span.Start.Offset < 0 || diagnostic.Span.Start.Offset > diagnostic.Span.End.Offset || diagnostic.Span.End.Offset > len(src) {
			t.Fatalf("invalid span: %#v", diagnostic)
		}
	}
}

func TestDiagnosticLimitIsAnAnalysisLimit(t *testing.T) {
	src := strings.Repeat("&&\n", maxDiagnostics+20)
	result := ValidateSource("many-errors.cmd", []byte(src), Options{ReportUnsupported: true})
	if len(result.Diagnostics) != maxDiagnostics || !hasCode(result, "BAT9003") || result.FullyValidated {
		t.Fatalf("diagnostic cap result = %d %#v", len(result.Diagnostics), result)
	}
}

func TestParserNestingLimitIsNotSyntaxError(t *testing.T) {
	src := strings.Repeat("(", maxParserDepth+1) + "echo x" + strings.Repeat(")", maxParserDepth+1)
	result := ValidateSource("deep.cmd", []byte(src), Options{ReportUnsupported: true})
	if result.HasErrors() || !hasCode(result, "BAT9001") || result.FullyValidated {
		t.Fatalf("nesting limit result = %#v", result)
	}
}
