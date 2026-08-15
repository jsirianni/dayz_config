package batchvalidate

import (
	"os"
	"sort"
)

// Severity describes the effect of a diagnostic on validity.
type Severity uint8

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// Position is a byte offset and its one-based physical line and column.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span is a half-open source range.
type Span struct {
	Start Position
	End   Position
}

// Diagnostic reports a documented violation or an analysis limitation.
type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Span     Span
}

// FeatureState represents what static analysis can prove about a cmd feature.
type FeatureState uint8

const (
	FeatureUnknown FeatureState = iota
	FeatureEnabled
	FeatureDisabled
)

// Options configures the assumptions made at batch-file entry.
type Options struct {
	InitialCommandExtensions FeatureState
	InitialDelayedExpansion  FeatureState
	ReportUnsupported        bool
}

// Result is the outcome of validation. FullyValidated is false when any source
// region intentionally remained opaque or runtime-dependent.
type Result struct {
	Diagnostics    []Diagnostic
	FullyValidated bool
}

// HasErrors reports whether any diagnostic has error severity.
func (r Result) HasErrors() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Valid reports whether no supported, documented rule was violated.
func (r Result) Valid() bool { return !r.HasErrors() }

// Source stores source bytes and a physical-line index. Bytes are not decoded
// or transcoded, so legacy encodings and invalid UTF-8 remain representable.
type Source struct {
	Name       string
	Bytes      []byte
	lineStarts []int
}

func newSource(name string, src []byte) Source {
	starts := []int{0}
	for offset := 0; offset < len(src); {
		switch src[offset] {
		case '\r':
			offset++
			if offset < len(src) && src[offset] == '\n' {
				offset++
			}
			starts = append(starts, offset)
		case '\n':
			offset++
			starts = append(starts, offset)
		default:
			offset++
		}
	}
	return Source{Name: name, Bytes: src, lineStarts: starts}
}

func (source Source) position(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source.Bytes) {
		offset = len(source.Bytes)
	}
	line := sort.Search(len(source.lineStarts), func(index int) bool {
		return source.lineStarts[index] > offset
	}) - 1
	return Position{Offset: offset, Line: line + 1, Column: offset - source.lineStarts[line] + 1}
}

func (source Source) span(start, end int) Span {
	if end < start {
		end = start
	}
	return Span{Start: source.position(start), End: source.position(end)}
}

// StatementKind identifies a high-level, source-preserving AST statement.
type StatementKind uint8

const (
	StatementCommand StatementKind = iota
	StatementLabel
	StatementGroup
)

// ChainOp identifies a documented command-chain operator.
type ChainOp uint8

const (
	ChainSequence ChainOp = iota
	ChainAnd
	ChainOr
	ChainPipe
)

// Redirection is a shell-level redirection attached to a command.
type Redirection struct {
	Handle   string
	Operator string
	Target   string
	Span     Span
}

// Command is a source-preserving shell command. Unknown commands are retained
// with Opaque set instead of being rejected.
type Command struct {
	Name         string
	Span         Span
	NameSpan     Span
	ArgsSpan     Span
	Redirections []Redirection
	Opaque       bool
}

// CommandLink joins another command to a command chain.
type CommandLink struct {
	Op      ChainOp
	OpSpan  Span
	Command Command
}

// CommandChain preserves operators without assuming undocumented precedence.
type CommandChain struct {
	First Command
	Rest  []CommandLink
}

// Statement is a high-level AST node with an exact source span.
type Statement struct {
	Kind  StatementKind
	Span  Span
	Label string
	Chain CommandChain
}

// Script is the parsed, source-ordered batch-file AST.
type Script struct {
	Statements []Statement
}

// ValidateFile reads and validates a batch file without executing it.
func ValidateFile(path string, opts Options) (Result, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	return ValidateSource(path, src, opts), nil
}

// ValidateSource validates caller-owned bytes without modifying them.
func ValidateSource(name string, src []byte, opts Options) Result {
	_, result := Parse(name, src, opts)
	return result
}

// Parse returns a source-preserving AST together with validation diagnostics.
func Parse(name string, src []byte, opts Options) (Script, Result) {
	parser := newBatchParser(newSource(name, src), opts)
	parser.parse()
	parser.validateStaticTargets()
	parser.validateExpansions()
	parser.finish()
	return parser.script, Result{Diagnostics: parser.diagnostics, FullyValidated: parser.fullyValidated}
}
