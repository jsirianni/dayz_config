package dayzinit

import "testing"

func TestDiagnosticsSortDeduplicateAndCap(t *testing.T) {
	var found diagnostics
	found.add(Diagnostic{Code: "ZERO", Message: "default position"})
	duplicate := Diagnostic{Code: "DZI9999", Message: "same", Span: Span{Start: Position{Offset: 2, Line: 1, Column: 3}, End: Position{Offset: 3, Line: 1, Column: 4}}}
	found.add(duplicate)
	found.add(duplicate)
	found.add(Diagnostic{Code: "DZI0002", Message: "second", Span: Span{Start: Position{Offset: 1, Line: 1, Column: 2}}})
	found.add(Diagnostic{Code: "DZI0001", Message: "first", Span: Span{Start: Position{Offset: 1, Line: 1, Column: 2}}})
	for index := 0; index < maxDiagnostics+10; index++ {
		found.add(Diagnostic{Code: "CAP", Message: string(rune(index + 32)), Span: Span{Start: Position{Offset: index + 10, Line: 2, Column: index + 1}}})
	}
	items := found.sorted()
	if len(items) != maxDiagnostics {
		t.Fatalf("diagnostic count = %d", len(items))
	}
	if items[0].Code != "ZERO" || items[1].Code != "DZI0001" || items[2].Code != "DZI0002" {
		t.Fatalf("sort order = %#v", items[:3])
	}
	var merged diagnostics
	merged.merge(items[:2])
	if len(merged.items) != 2 {
		t.Fatal("merge failed")
	}
}
