package batchvalidate

import (
	"sort"
	"strings"
)

const (
	maxParserDepth = 128
	maxDiagnostics = 256
)

type staticTarget struct {
	text    string
	span    Span
	dynamic bool
	eof     bool
	state   FeatureState
	call    bool
}

type featureSnapshot struct {
	extensions FeatureState
	delayed    FeatureState
}

type activeForFrame struct {
	depth, allocations int
}

type batchParser struct {
	source             Source
	options            Options
	script             Script
	diagnostics        []Diagnostic
	fullyValidated     bool
	diagnosticLimitHit bool
	unsupportedOffsets map[int]bool
	labels             map[string]bool
	targets            []staticTarget
	extensions         FeatureState
	delayed            FeatureState
	featureStack       []featureSnapshot
	commandDepth       int
	groupDepth         int
	activeFors         []activeForFrame
	lineForAllocations []int
}

func newBatchParser(source Source, options Options) *batchParser {
	return &batchParser{
		source:             source,
		options:            options,
		fullyValidated:     true,
		unsupportedOffsets: make(map[int]bool),
		labels:             make(map[string]bool),
		extensions:         options.InitialCommandExtensions,
		delayed:            options.InitialDelayedExpansion,
	}
}

func (parser *batchParser) parse() {
	parser.validateGroups()
	parser.validateQuoteCompleteness()
	for _, line := range physicalLines(parser.source.Bytes) {
		parser.lineForAllocations = parser.lineForAllocations[:0]
		parser.parseLine(line)
		newDepth := parser.groupDepth + parenthesisDelta(parser.source.Bytes[line.start:line.contentEnd])
		if newDepth < 0 {
			newDepth = 0
		}
		for _, allocations := range parser.lineForAllocations {
			if newDepth > parser.groupDepth {
				parser.activeFors = append(parser.activeFors, activeForFrame{depth: newDepth, allocations: allocations})
			}
		}
		parser.groupDepth = newDepth
		for len(parser.activeFors) > 0 && parser.activeFors[len(parser.activeFors)-1].depth > parser.groupDepth {
			parser.activeFors = parser.activeFors[:len(parser.activeFors)-1]
		}
	}
}

func (parser *batchParser) validateGroups() {
	depth := 0
	limitReported := false
	for _, current := range scan(parser.source.Bytes) {
		switch current.kind {
		case tokenLParen:
			depth++
			if depth > maxParserDepth && !limitReported {
				parser.unsupported("BAT9001", "parser nesting limit reached", current.start, current.end)
				limitReported = true
			}
		case tokenRParen:
			if depth == 0 {
				parser.error("BAT1003", "unexpected closing parenthesis", current.start, current.end)
			} else {
				depth--
			}
		}
	}
	if depth > 0 {
		parser.error("BAT1002", "missing closing parenthesis", len(parser.source.Bytes), len(parser.source.Bytes))
	}
}

func (parser *batchParser) parseLine(line physicalLine) {
	start, end := trimSpace(parser.source.Bytes, line.start, line.contentEnd)
	if start == end {
		return
	}
	if parser.source.Bytes[start] == ':' {
		nameStart, nameEnd := trimSpace(parser.source.Bytes, start+1, end)
		label := string(parser.source.Bytes[nameStart:nameEnd])
		parser.labels[lowerASCII(label)] = true
		parser.script.Statements = append(parser.script.Statements, Statement{
			Kind: StatementLabel, Span: parser.source.span(start, end), Label: label,
		})
		return
	}
	lowerLine := lowerASCII(string(parser.source.Bytes[start:end]))
	if startsElseOnNewLine(lowerLine) {
		elseStart := start
		if parser.source.Bytes[start] == ')' {
			elseStart++
			for elseStart < end && isSpace(parser.source.Bytes[elseStart]) {
				elseStart++
			}
			if equalFoldASCII(strings.TrimSpace(string(parser.source.Bytes[elseStart:end])), "else") {
				parser.error("BAT2001", "ELSE requires a command body", end, end)
			}
		} else {
			parser.error("BAT2003", "ELSE must be on the same line as the preceding IF command", elseStart, minInt(elseStart+4, end))
		}
		parser.script.Statements = append(parser.script.Statements, Statement{Kind: StatementGroup, Span: parser.source.span(start, end)})
		return
	}
	segments, operators := splitChain(parser.source.Bytes, start, end)
	commands := make([]Command, len(segments))
	present := make([]bool, len(segments))
	for index, segment := range segments {
		segmentStart, segmentEnd := trimSpace(parser.source.Bytes, segment.start, segment.end)
		if segmentStart == segmentEnd {
			if index > 0 && !(index == 1 && !present[0]) {
				operator := operators[index-1]
				parser.error("BAT1001", "command operator requires an operand", operator.start, operator.end)
			} else if index == 0 && len(operators) > 0 {
				parser.error("BAT1001", "command operator requires an operand", operators[0].start, operators[0].end)
			}
			continue
		}
		command, ok := parser.parseCommandSegment(segmentStart, segmentEnd)
		if ok {
			commands[index] = command
			present[index] = true
		}
	}
	firstIndex := -1
	for index := range present {
		if present[index] {
			firstIndex = index
			break
		}
	}
	if firstIndex < 0 {
		parser.script.Statements = append(parser.script.Statements, Statement{Kind: StatementGroup, Span: parser.source.span(start, end)})
		return
	}
	chain := CommandChain{First: commands[firstIndex]}
	previous := firstIndex
	for index := firstIndex + 1; index < len(commands); index++ {
		if !present[index] {
			continue
		}
		opIndex := index - 1
		if previous != index-1 {
			opIndex = previous
		}
		chain.Rest = append(chain.Rest, CommandLink{
			Op: chainOperator(operators[opIndex].kind), OpSpan: parser.source.span(operators[opIndex].start, operators[opIndex].end), Command: commands[index],
		})
		previous = index
	}
	parser.script.Statements = append(parser.script.Statements, Statement{
		Kind: StatementCommand, Span: parser.source.span(start, end), Chain: chain,
	})
}

type sourceSegment struct{ start, end int }

func splitChain(src []byte, start, end int) ([]sourceSegment, []token) {
	segments := make([]sourceSegment, 0, 2)
	operators := make([]token, 0, 1)
	segmentStart := start
	for _, current := range scan(src[start:end]) {
		if current.kind != tokenAmp && current.kind != tokenAnd && current.kind != tokenPipe && current.kind != tokenOr {
			continue
		}
		current.start += start
		current.end += start
		segments = append(segments, sourceSegment{start: segmentStart, end: current.start})
		operators = append(operators, current)
		segmentStart = current.end
	}
	segments = append(segments, sourceSegment{start: segmentStart, end: end})
	return segments, operators
}

func chainOperator(kind tokenKind) ChainOp {
	switch kind {
	case tokenAnd:
		return ChainAnd
	case tokenOr:
		return ChainOr
	case tokenPipe:
		return ChainPipe
	default:
		return ChainSequence
	}
}

func (parser *batchParser) parseCommandSegment(start, end int) (Command, bool) {
	clean := append([]byte(nil), parser.source.Bytes[start:end]...)
	redirections := parser.parseRedirections(clean, start)
	commandStart, commandEnd := trimSpace(clean, 0, len(clean))
	for commandStart < commandEnd && (clean[commandStart] == '(' || clean[commandStart] == ')') {
		commandStart++
		for commandStart < commandEnd && isSpace(clean[commandStart]) {
			commandStart++
		}
	}
	if commandStart < commandEnd && clean[commandStart] == '@' {
		commandStart++
	}
	for commandStart < commandEnd && isSpace(clean[commandStart]) {
		commandStart++
	}
	if commandStart == commandEnd {
		return Command{}, false
	}
	nameEnd := commandStart
	for nameEnd < commandEnd && !isSpace(clean[nameEnd]) && clean[nameEnd] != '(' && clean[nameEnd] != ')' {
		nameEnd++
	}
	nameText := string(clean[commandStart:nameEnd])
	if nameEnd == commandStart {
		return Command{}, false
	}
	name, attachedArgs := commandName(nameText)
	actualNameEnd := commandStart + len(nameText) - len(attachedArgs)
	argsStart := actualNameEnd
	for argsStart < commandEnd && isSpace(clean[argsStart]) {
		argsStart++
	}
	command := Command{
		Name: name, Span: parser.source.span(start, end),
		NameSpan:     parser.source.span(start+commandStart, start+actualNameEnd),
		ArgsSpan:     parser.source.span(start+argsStart, start+commandEnd),
		Redirections: redirections,
	}
	known := parser.validateCommand(name, clean[argsStart:commandEnd], start+argsStart, command.NameSpan)
	command.Opaque = !known
	if command.Opaque {
		parser.unsupported("BAT9002", "external or undocumented command arguments were left opaque", start+commandStart, start+actualNameEnd)
	}
	return command, true
}

func commandName(text string) (string, string) {
	lower := lowerASCII(text)
	for _, prefix := range []string{"goto:", "echo:"} {
		if strings.HasPrefix(lower, prefix) {
			return prefix[:len(prefix)-1], text[len(prefix)-1:]
		}
	}
	return lower, ""
}

func (parser *batchParser) parseRedirections(clean []byte, base int) []Redirection {
	redirections := make([]Redirection, 0, 2)
	quoted := false
	for offset := 0; offset < len(clean); {
		if clean[offset] == '^' {
			offset += 2
			if offset > len(clean) {
				offset = len(clean)
			}
			continue
		}
		if clean[offset] == '"' {
			quoted = !quoted
			offset++
			continue
		}
		if quoted || (clean[offset] != '<' && clean[offset] != '>') {
			offset++
			continue
		}
		opStart := offset
		offset++
		operator := string(clean[opStart:offset])
		if offset < len(clean) && ((clean[opStart] == '>' && clean[offset] == '>') || clean[offset] == '&') {
			offset++
			operator = string(clean[opStart:offset])
		}
		handleStart := opStart
		handle := ""
		if opStart > 0 && clean[opStart-1] >= '0' && clean[opStart-1] <= '9' {
			if opStart == 1 || clean[opStart-2] < '0' || clean[opStart-2] > '9' {
				handleStart = opStart - 1
				handle = string(clean[handleStart:opStart])
			} else {
				parser.unsupported("BAT9002", "multi-digit redirection handles are outside the documented subset", base+opStart-2, base+opStart)
			}
		}
		for offset < len(clean) && isSpace(clean[offset]) {
			offset++
		}
		targetStart := offset
		if operator == ">&" || operator == "<&" {
			if offset < len(clean) && clean[offset] >= '0' && clean[offset] <= '9' {
				offset++
			} else if offset < len(clean) && (clean[offset] == '%' || clean[offset] == '!') {
				offset = redirectionTargetEnd(clean, offset)
				parser.unsupported("BAT9002", "dynamic redirection handle was not resolved", base+targetStart, base+offset)
			} else {
				end := minInt(offset+1, len(clean))
				parser.error("BAT8002", "handle redirection requires a handle from 0 through 9", base+targetStart, base+end)
			}
		} else {
			offset = redirectionTargetEnd(clean, offset)
		}
		if targetStart == offset {
			parser.error("BAT8001", "redirection operator requires a target", base+opStart, base+opStart+len(operator))
		}
		target := string(clean[targetStart:offset])
		spanEnd := offset
		redirections = append(redirections, Redirection{
			Handle: handle, Operator: operator, Target: target, Span: parser.source.span(base+handleStart, base+spanEnd),
		})
		for index := handleStart; index < spanEnd; index++ {
			clean[index] = ' '
		}
	}
	return redirections
}

func redirectionTargetEnd(src []byte, offset int) int {
	if offset >= len(src) {
		return offset
	}
	if src[offset] == '"' {
		offset++
		for offset < len(src) {
			if src[offset] == '^' && offset+1 < len(src) {
				offset += 2
				continue
			}
			offset++
			if src[offset-1] == '"' {
				break
			}
		}
		return offset
	}
	for offset < len(src) && !isSpace(src[offset]) && src[offset] != '&' && src[offset] != '|' && src[offset] != '<' && src[offset] != '>' && src[offset] != '(' && src[offset] != ')' {
		if src[offset] == '^' && offset+1 < len(src) {
			offset += 2
		} else {
			offset++
		}
	}
	return offset
}

func startsElseOnNewLine(lowerLine string) bool {
	trimmed := lowerASCII(strings.TrimSpace(lowerLine))
	if strings.HasPrefix(trimmed, ")") {
		trimmed = strings.TrimSpace(trimmed[1:])
		return trimmed == "else" || strings.HasPrefix(trimmed, "else ") || strings.HasPrefix(trimmed, "else(")
	}
	return trimmed == "else" || strings.HasPrefix(trimmed, "else ") || strings.HasPrefix(trimmed, "else(")
}

func (parser *batchParser) validateQuoteCompleteness() {
	for _, line := range physicalLines(parser.source.Bytes) {
		quoted := false
		quoteStart := line.start
		for offset := line.start; offset < line.contentEnd; offset++ {
			if parser.source.Bytes[offset] == '^' {
				offset++
				continue
			}
			if parser.source.Bytes[offset] == '"' {
				if !quoted {
					quoteStart = offset
				}
				quoted = !quoted
			}
		}
		if quoted {
			parser.unsupported("BAT9002", "unmatched quote was preserved without assuming it is invalid", quoteStart, line.contentEnd)
		}
	}
}

func parenthesisDelta(src []byte) int {
	delta := 0
	for _, current := range scan(src) {
		if current.kind == tokenLParen {
			delta++
		} else if current.kind == tokenRParen {
			delta--
		}
	}
	return delta
}

func (parser *batchParser) activeForAllocations() int {
	total := 0
	for _, frame := range parser.activeFors {
		total += frame.allocations
	}
	return total
}

func (parser *batchParser) error(code, message string, start, end int) {
	parser.addDiagnostic(Diagnostic{Code: code, Severity: SeverityError, Message: message, Span: parser.source.span(start, end)})
}

func (parser *batchParser) unsupported(code, message string, start, end int) {
	parser.fullyValidated = false
	if !parser.options.ReportUnsupported || parser.unsupportedOffsets[start] {
		return
	}
	parser.unsupportedOffsets[start] = true
	parser.addDiagnostic(Diagnostic{Code: code, Severity: SeverityInfo, Message: message, Span: parser.source.span(start, end)})
}

func (parser *batchParser) addDiagnostic(diagnostic Diagnostic) {
	if parser.diagnosticLimitHit {
		return
	}
	if len(parser.diagnostics) == maxDiagnostics-1 {
		parser.diagnosticLimitHit = true
		parser.fullyValidated = false
		parser.diagnostics = append(parser.diagnostics, Diagnostic{
			Code: "BAT9003", Severity: SeverityInfo, Message: "diagnostic limit reached", Span: parser.source.span(diagnostic.Span.Start.Offset, diagnostic.Span.Start.Offset),
		})
		return
	}
	parser.diagnostics = append(parser.diagnostics, diagnostic)
}

func (parser *batchParser) finish() {
	sort.SliceStable(parser.diagnostics, func(left, right int) bool {
		a, b := parser.diagnostics[left], parser.diagnostics[right]
		if a.Span.Start.Offset != b.Span.Start.Offset {
			return a.Span.Start.Offset < b.Span.Start.Offset
		}
		if a.Span.End.Offset != b.Span.End.Offset {
			return a.Span.End.Offset < b.Span.End.Offset
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	if len(parser.diagnostics) < 2 {
		return
	}
	unique := parser.diagnostics[:1]
	for _, diagnostic := range parser.diagnostics[1:] {
		previous := unique[len(unique)-1]
		if diagnostic.Code == previous.Code && diagnostic.Message == previous.Message && diagnostic.Span == previous.Span {
			continue
		}
		unique = append(unique, diagnostic)
	}
	parser.diagnostics = unique
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
