package batchvalidate

import (
	"reflect"
	"testing"
)

var fuzzSeeds = [][]byte{
	{},
	[]byte("echo hello"),
	[]byte("if exist x (echo yes) else (echo no)"),
	[]byte("for /f \"tokens=1,2\" %%A in (x) do echo %%A"),
	[]byte("set /a \"A=(1+2)*3\""),
	[]byte("echo x 2>&1 && goto :EOF"),
	[]byte("((((((((echo deep))))))))"),
	[]byte("%%%%!!!!^^^^\"&|<>\""),
	{0xff, 0xfe, 'e', 'c', 'h', 'o', ' ', 0x80},
}

func FuzzValidateNeverPanics(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		ValidateSource("fuzz.cmd", src, Options{})
	})
}

func FuzzScannerMakesProgress(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		previous := 0
		for _, current := range scan(src) {
			if current.start < previous || current.start < 0 || current.end <= current.start || current.end > len(src) {
				t.Fatalf("invalid token %#v", current)
			}
			previous = current.end
		}
	})
}

func FuzzParserSpans(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		script, result := Parse("fuzz.cmd", src, Options{ReportUnsupported: true})
		check := func(span Span) {
			if span.Start.Offset < 0 || span.Start.Offset > span.End.Offset || span.End.Offset > len(src) {
				t.Fatalf("invalid span %#v for %d bytes", span, len(src))
			}
		}
		for _, statement := range script.Statements {
			check(statement.Span)
			if statement.Kind == StatementCommand {
				check(statement.Chain.First.Span)
				for _, link := range statement.Chain.Rest {
					check(link.OpSpan)
					check(link.Command.Span)
				}
			}
		}
		for _, diagnostic := range result.Diagnostics {
			check(diagnostic.Span)
		}
	})
}

func FuzzDeterministicDiagnostics(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		first := ValidateSource("fuzz.cmd", src, Options{ReportUnsupported: true})
		second := ValidateSource("fuzz.cmd", src, Options{ReportUnsupported: true})
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("nondeterministic results: %#v != %#v", first, second)
		}
	})
}

func FuzzNoInvalidUTF8Assumption(f *testing.F) {
	f.Add([]byte{0xff, 'e', 'c', 'h', 'o', 0xfe})
	f.Add([]byte{'g', 'o', 't', 'o', ' ', 0x80})
	f.Fuzz(func(t *testing.T, src []byte) {
		ValidateSource("bytes.cmd", src, Options{})
	})
}
