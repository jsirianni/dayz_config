package dayzinit

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxConditionalSymbols = 6

type sourceLine struct {
	start      int
	contentEnd int
	end        int
	directive  string
	argument   string
}

type conditionalFrame struct {
	parentActive bool
	condition    bool
	active       bool
	elseSeen     bool
	line         sourceLine
}

func analyzePreprocessor(source *sourceFile) ([][]byte, []Diagnostic) {
	lines := scanSourceLines(source.data, source.bomBytes)
	var found diagnostics
	symbolSet := make(map[string]struct{})
	var stack []conditionalFrame

	for _, line := range lines {
		if line.directive == "" {
			continue
		}
		fields := strings.Fields(line.argument)
		switch line.directive {
		case "include":
			if !validIncludeArgument(line.argument) {
				found.add(preprocessorDiagnostic(source, line, "DZI2001", "invalid #include directive", "use #include \"path\" or #include <path>"))
			}
		case "define":
			if len(fields) == 0 || !validIdentifierText(fields[0]) {
				found.add(preprocessorDiagnostic(source, line, "DZI2002", "#define requires an identifier", "add a macro name after #define"))
			}
		case "ifdef", "ifndef":
			if len(fields) != 1 || !validIdentifierText(fields[0]) {
				found.add(preprocessorDiagnostic(source, line, "DZI2003", fmt.Sprintf("#%s requires exactly one identifier", line.directive), "provide one preprocessor symbol"))
				continue
			}
			symbolSet[fields[0]] = struct{}{}
			stack = append(stack, conditionalFrame{line: line})
		case "else":
			if strings.TrimSpace(line.argument) != "" {
				found.add(preprocessorDiagnostic(source, line, "DZI2004", "#else does not accept an argument", "remove text after #else"))
			}
			if len(stack) == 0 {
				found.add(preprocessorDiagnostic(source, line, "DZI2005", "#else has no matching conditional", "add a preceding #ifdef or #ifndef"))
			} else if stack[len(stack)-1].elseSeen {
				found.add(preprocessorDiagnostic(source, line, "DZI2006", "conditional contains more than one #else", "remove the duplicate #else"))
			} else {
				stack[len(stack)-1].elseSeen = true
			}
		case "endif":
			if strings.TrimSpace(line.argument) != "" {
				found.add(preprocessorDiagnostic(source, line, "DZI2007", "#endif does not accept an argument", "remove text after #endif"))
			}
			if len(stack) == 0 {
				found.add(preprocessorDiagnostic(source, line, "DZI2008", "#endif has no matching conditional", "remove it or add a preceding conditional"))
			} else {
				stack = stack[:len(stack)-1]
			}
		default:
			found.add(preprocessorDiagnostic(source, line, "DZI2009", fmt.Sprintf("unsupported preprocessor directive #%s", line.directive), "use #include, #define, #ifdef, #ifndef, #else, or #endif"))
		}
	}
	for _, frame := range stack {
		found.add(preprocessorDiagnostic(source, frame.line, "DZI2010", fmt.Sprintf("#%s is missing #endif", frame.line.directive), "close the conditional with #endif"))
	}
	if len(symbolSet) > maxConditionalSymbols {
		found.add(Diagnostic{
			Code:    "DZI2011",
			Message: fmt.Sprintf("preprocessor uses %d conditional symbols; safety limit is %d", len(symbolSet), maxConditionalSymbols),
			Hint:    "reduce independent conditional symbols in init.c",
			Span:    source.span(source.bomBytes, source.bomBytes),
		})
	}
	if len(found.items) != 0 {
		return nil, found.sorted()
	}

	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	variantCount := 1 << len(symbols)
	variants := make([][]byte, 0, variantCount)
	for bits := 0; bits < variantCount; bits++ {
		environment := make(map[string]bool, len(symbols))
		for index, symbol := range symbols {
			environment[symbol] = bits&(1<<index) != 0
		}
		variants = append(variants, renderConditionalVariant(source.data, lines, environment))
	}
	return variants, nil
}

func scanSourceLines(data []byte, start int) []sourceLine {
	var lines []sourceLine
	inBlockComment := false
	for offset := start; offset < len(data); {
		lineStart := offset
		for offset < len(data) && data[offset] != '\n' {
			offset++
		}
		contentEnd := offset
		if contentEnd > lineStart && data[contentEnd-1] == '\r' {
			contentEnd--
		}
		if offset < len(data) {
			offset++
		}
		line := sourceLine{start: lineStart, contentEnd: contentEnd, end: offset}
		content := data[lineStart:contentEnd]
		first := firstCodeOffset(content, inBlockComment)
		if first >= 0 && content[first] == '#' {
			rest := strings.TrimSpace(string(content[first+1:]))
			nameEnd := 0
			for nameEnd < len(rest) && ((rest[nameEnd] >= 'a' && rest[nameEnd] <= 'z') || (rest[nameEnd] >= 'A' && rest[nameEnd] <= 'Z')) {
				nameEnd++
			}
			line.directive = strings.ToLower(rest[:nameEnd])
			line.argument = strings.TrimSpace(rest[nameEnd:])
		}
		inBlockComment = blockCommentState(content, inBlockComment)
		lines = append(lines, line)
	}
	return lines
}

func firstCodeOffset(line []byte, inBlockComment bool) int {
	for offset := 0; offset < len(line); {
		if inBlockComment {
			end := strings.Index(string(line[offset:]), "*/")
			if end < 0 {
				return -1
			}
			offset += end + 2
			inBlockComment = false
			continue
		}
		if line[offset] == ' ' || line[offset] == '\t' {
			offset++
			continue
		}
		if offset+1 < len(line) && line[offset] == '/' && line[offset+1] == '*' {
			inBlockComment = true
			offset += 2
			continue
		}
		if offset+1 < len(line) && line[offset] == '/' && line[offset+1] == '/' {
			return -1
		}
		return offset
	}
	return -1
}

func blockCommentState(line []byte, inBlock bool) bool {
	for offset := 0; offset < len(line); {
		if inBlock {
			end := strings.Index(string(line[offset:]), "*/")
			if end < 0 {
				return true
			}
			offset += end + 2
			inBlock = false
			continue
		}
		if offset+1 < len(line) && line[offset] == '/' && line[offset+1] == '/' {
			return false
		}
		if offset+1 < len(line) && line[offset] == '/' && line[offset+1] == '*' {
			inBlock = true
			offset += 2
			continue
		}
		if line[offset] == '"' {
			offset++
			for offset < len(line) && line[offset] != '"' {
				if line[offset] == '\\' && offset+1 < len(line) {
					offset += 2
				} else {
					offset++
				}
			}
		}
		offset++
	}
	return inBlock
}

func renderConditionalVariant(original []byte, lines []sourceLine, environment map[string]bool) []byte {
	result := append([]byte(nil), original...)
	active := true
	var stack []conditionalFrame
	for _, line := range lines {
		if line.directive != "" {
			maskLine(result, line)
			fields := strings.Fields(line.argument)
			switch line.directive {
			case "define":
				if active && len(fields) > 0 {
					environment[fields[0]] = true
				}
			case "ifdef", "ifndef":
				condition := environment[fields[0]]
				if line.directive == "ifndef" {
					condition = !condition
				}
				frame := conditionalFrame{parentActive: active, condition: condition, active: active && condition}
				stack = append(stack, frame)
				active = frame.active
			case "else":
				frame := &stack[len(stack)-1]
				frame.active = frame.parentActive && !frame.condition
				active = frame.active
			case "endif":
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				active = frame.parentActive
			}
			continue
		}
		if !active {
			maskLine(result, line)
		}
	}
	return result
}

func maskLine(data []byte, line sourceLine) {
	for offset := line.start; offset < line.contentEnd; offset++ {
		if data[offset] != '\r' {
			data[offset] = ' '
		}
	}
}

func validIncludeArgument(argument string) bool {
	argument = strings.TrimSpace(argument)
	if len(argument) < 2 {
		return false
	}
	return (argument[0] == '"' && argument[len(argument)-1] == '"') || (argument[0] == '<' && argument[len(argument)-1] == '>')
}

func validIdentifierText(text string) bool {
	if text == "" {
		return false
	}
	first, size := utf8.DecodeRuneInString(text)
	if !isIdentifierStartRune(first) {
		return false
	}
	for _, character := range text[size:] {
		if !isIdentifierContinueRune(character) {
			return false
		}
	}
	return true
}

func preprocessorDiagnostic(source *sourceFile, line sourceLine, code, message, hint string) Diagnostic {
	return Diagnostic{Code: code, Message: message, Hint: hint, Span: source.span(line.start, line.contentEnd)}
}
