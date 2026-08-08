package dayzinit

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalid identifies source that is not a valid DayZ mission init.c.
var ErrInvalid = errors.New("invalid DayZ init.c")

// Position identifies a byte and human-readable source location.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span identifies a half-open source range.
type Span struct {
	Start Position
	End   Position
}

// Diagnostic describes one independently inspectable validation failure.
type Diagnostic struct {
	Code    string
	Message string
	Hint    string
	Span    Span
}

// ValidationError contains every safely collected validation diagnostic.
type ValidationError struct {
	Path        string
	Diagnostics []Diagnostic
}

func (err *ValidationError) Error() string {
	if err == nil {
		return ErrInvalid.Error()
	}
	if len(err.Diagnostics) == 0 {
		return fmt.Sprintf("%s: %s", err.Path, ErrInvalid)
	}
	var result strings.Builder
	for index, diagnostic := range err.Diagnostics {
		if index > 0 {
			result.WriteByte('\n')
		}
		fmt.Fprintf(&result, "%s:%d:%d [%s] %s", err.Path, diagnostic.Span.Start.Line, diagnostic.Span.Start.Column, diagnostic.Code, diagnostic.Message)
		if diagnostic.Hint != "" {
			result.WriteString("\nhint: ")
			result.WriteString(diagnostic.Hint)
		}
	}
	return result.String()
}

// Unwrap allows errors.Is(err, ErrInvalid).
func (err *ValidationError) Unwrap() error {
	return ErrInvalid
}

const maxDiagnostics = 100

type diagnostics struct {
	items []Diagnostic
	seen  map[string]struct{}
}

func (list *diagnostics) add(item Diagnostic) {
	if len(list.items) >= maxDiagnostics {
		return
	}
	if item.Span.Start.Line == 0 {
		item.Span.Start.Line, item.Span.Start.Column = 1, 1
	}
	if item.Span.End.Line == 0 {
		item.Span.End = item.Span.Start
	}
	key := fmt.Sprintf("%s:%d:%d:%s", item.Code, item.Span.Start.Offset, item.Span.End.Offset, item.Message)
	if list.seen == nil {
		list.seen = make(map[string]struct{})
	}
	if _, exists := list.seen[key]; exists {
		return
	}
	list.seen[key] = struct{}{}
	list.items = append(list.items, item)
}

func (list *diagnostics) merge(items []Diagnostic) {
	for _, item := range items {
		list.add(item)
	}
}

func (list *diagnostics) sorted() []Diagnostic {
	items := append([]Diagnostic(nil), list.items...)
	sort.SliceStable(items, func(left, right int) bool {
		if items[left].Span.Start.Offset != items[right].Span.Start.Offset {
			return items[left].Span.Start.Offset < items[right].Span.Start.Offset
		}
		if items[left].Code != items[right].Code {
			return items[left].Code < items[right].Code
		}
		return items[left].Message < items[right].Message
	})
	return items
}
