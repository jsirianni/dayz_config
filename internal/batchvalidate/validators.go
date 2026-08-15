package batchvalidate

import (
	"path/filepath"
	"strconv"
	"strings"
)

func (parser *batchParser) validateCommand(name string, args []byte, base int, nameSpan Span) bool {
	known := true
	switch name {
	case "if":
		parser.validateIf(args, base)
	case "for":
		parser.validateFor(args, base)
	case "set":
		parser.validateSet(args, base)
	case "call":
		parser.validateCall(args, base)
	case "goto":
		parser.validateGoto(args, base)
	case "setlocal":
		parser.validateSetlocal(args, base)
	case "endlocal":
		parser.validateEndlocal(args, base)
	case "shift":
		parser.validateShift(args, base)
	case "echo", "rem":
		// Their documented argument text is intentionally permissive.
	default:
		known = false
	}
	if known && (name == "if" || name == "for" || name == "goto" || name == "call") {
		parser.extensions = FeatureUnknown
		parser.delayed = FeatureUnknown
	}
	_ = nameSpan
	return known
}

func (parser *batchParser) validateIf(args []byte, base int) {
	words := splitWords(args, base)
	if len(words) == 0 {
		parser.error("BAT2001", "IF requires a condition", base, base)
		return
	}
	index := 0
	caseInsensitive := false
	for index < len(words) {
		switch lowerASCII(words[index].text) {
		case "/i":
			caseInsensitive = true
			index++
		case "not":
			index++
		default:
			goto condition
		}
	}

condition:
	if index >= len(words) {
		parser.error("BAT2001", "IF requires a condition operand", base+len(args), base+len(args))
		return
	}
	conditionWord := lowerASCII(words[index].text)
	switch conditionWord {
	case "errorlevel", "cmdextversion":
		if conditionWord == "cmdextversion" {
			parser.requireExtensions(words[index].start, words[index].end)
		}
		index++
		if index >= len(words) {
			parser.error("BAT2001", "IF numeric condition requires a number", base+len(args), base+len(args))
			return
		}
		if dynamicText(words[index].text) {
			parser.unsupported("BAT9002", "dynamic IF number was not resolved", words[index].start, words[index].end)
		} else if !decimalDigits(words[index].text) {
			parser.error("BAT2004", "IF condition requires a literal decimal number", words[index].start, words[index].end)
		}
		index++
		parser.requireIfBody(args, base, words, index)
	case "exist", "defined":
		if conditionWord == "defined" {
			parser.requireExtensions(words[index].start, words[index].end)
		}
		index++
		if index >= len(words) {
			parser.error("BAT2001", "IF condition requires an operand", base+len(args), base+len(args))
			return
		}
		index++
		parser.requireIfBody(args, base, words, index)
	default:
		if strings.Contains(words[index].text, "==") {
			parts := strings.SplitN(words[index].text, "==", 2)
			if parts[0] == "" || parts[1] == "" {
				parser.error("BAT2001", "IF equality requires operands on both sides of ==", words[index].start, words[index].end)
				return
			}
			index++
			parser.requireIfBody(args, base, words, index)
			return
		}
		if index+2 < len(words) && words[index+1].text == "==" {
			parser.requireIfBody(args, base, words, index+3)
			return
		}
		if index+2 >= len(words) {
			parser.error("BAT2001", "IF comparison requires two operands and a command", words[index].start, words[index].end)
			return
		}
		operator := lowerASCII(words[index+1].text)
		if !ifComparisonOperator(operator) {
			if caseInsensitive || isThreeLetters(operator) {
				parser.error("BAT2002", "IF comparison operator is not documented", words[index+1].start, words[index+1].end)
			} else {
				parser.error("BAT2001", "IF condition does not match a documented form", words[index].start, words[index+1].end)
			}
			return
		}
		parser.requireExtensions(words[index+1].start, words[index+1].end)
		parser.requireIfBody(args, base, words, index+3)
	}
}

func (parser *batchParser) requireIfBody(args []byte, base int, words []word, index int) {
	if index >= len(words) {
		parser.error("BAT2001", "IF requires a command body", base+len(args), base+len(args))
		return
	}
	if equalFoldASCII(words[index].text, "else") {
		parser.error("BAT2001", "IF requires a command body before ELSE", words[index].start, words[index].end)
		return
	}
	for bodyIndex := index + 1; bodyIndex < len(words); bodyIndex++ {
		if equalFoldASCII(words[bodyIndex].text, "else") && bodyIndex == len(words)-1 {
			parser.error("BAT2001", "ELSE requires a command body", words[bodyIndex].end, words[bodyIndex].end)
			return
		}
	}
	bodyStart := words[index].start
	if parser.source.Bytes[bodyStart] != '(' {
		parser.validateInline(parser.source.Bytes[bodyStart:base+len(args)], bodyStart)
	}
}

func ifComparisonOperator(value string) bool {
	switch value {
	case "equ", "neq", "lss", "leq", "gtr", "geq":
		return true
	default:
		return false
	}
}

func isThreeLetters(value string) bool {
	if len(value) != 3 {
		return false
	}
	for index := range value {
		if value[index] < 'a' || value[index] > 'z' {
			return false
		}
	}
	return true
}

func (parser *batchParser) validateFor(args []byte, base int) {
	words := splitWords(args, base)
	if len(words) == 0 {
		parser.error("BAT3001", "FOR requires a batch variable", base, base)
		return
	}
	mode := ""
	first := 0
	if strings.HasPrefix(words[0].text, "/") {
		mode = lowerASCII(words[0].text)
		first++
		switch mode {
		case "/d", "/r", "/l", "/f":
			parser.requireExtensions(words[0].start, words[0].end)
		default:
			parser.error("BAT3006", "FOR option is not documented", words[0].start, words[0].end)
		}
	}
	inIndex := -1
	for index := first; index < len(words); index++ {
		if equalFoldASCII(words[index].text, "in") {
			inIndex = index
			break
		}
	}
	if inIndex < 0 {
		parser.error("BAT3002", "FOR requires IN", base+len(args), base+len(args))
		return
	}
	variableIndex := inIndex - 1
	if variableIndex < first || !validForVariableToken(words[variableIndex].text) {
		spanStart, spanEnd := base, base
		if variableIndex >= first {
			spanStart, spanEnd = words[variableIndex].start, words[variableIndex].end
		}
		if variableIndex >= first && singlePercentForVariable(words[variableIndex].text) {
			parser.error("BAT3001", "FOR variables in batch files require doubled percent signs", spanStart, spanEnd)
		} else {
			parser.error("BAT3001", "FOR requires a documented batch variable", spanStart, spanEnd)
		}
		return
	}
	setStart := words[inIndex].end - base
	for setStart < len(args) && isSpace(args[setStart]) {
		setStart++
	}
	if setStart >= len(args) || args[setStart] != '(' {
		parser.error("BAT3003", "FOR set requires parentheses", base+setStart, base+minInt(setStart+1, len(args)))
		return
	}
	setEnd := matchingParen(args, setStart)
	if setEnd < 0 {
		parser.error("BAT3003", "FOR set is missing its closing parenthesis", base+len(args), base+len(args))
		return
	}
	afterSet := splitWords(args[setEnd+1:], base+setEnd+1)
	if len(afterSet) == 0 || !equalFoldASCII(afterSet[0].text, "do") {
		spanStart := base + setEnd + 1
		spanEnd := spanStart
		if len(afterSet) > 0 {
			spanStart, spanEnd = afterSet[0].start, afterSet[0].end
		}
		parser.error("BAT3004", "FOR requires DO", spanStart, spanEnd)
		return
	}
	bodyStart := afterSet[0].end - base
	for bodyStart < len(args) && isSpace(args[bodyStart]) {
		bodyStart++
	}
	if bodyStart == len(args) {
		parser.error("BAT3007", "FOR requires a command body", base+len(args), base+len(args))
		return
	}
	setText := args[setStart+1 : setEnd]
	allocations := 1
	if mode == "/l" {
		parser.validateForL(setText, base+setStart+1)
	}
	if mode == "/f" {
		allocations = parser.validateForF(words[first:variableIndex], base)
	}
	variable := words[variableIndex].text[len(words[variableIndex].text)-1]
	if variable >= 'A' && variable <= 'Z' && allocations > int('Z'-variable)+1 || variable >= 'a' && variable <= 'z' && allocations > int('z'-variable)+1 {
		parser.error("BAT3008", "FOR token allocation exceeds the documented variable range", words[variableIndex].start, words[variableIndex].end)
	}
	if parser.activeForAllocations()+allocations > 52 {
		parser.error("BAT3009", "more than 52 simultaneously active FOR variables are proven", words[variableIndex].start, words[variableIndex].end)
	}
	if args[bodyStart] == '(' {
		parser.lineForAllocations = append(parser.lineForAllocations, allocations)
		return
	}
	parser.activeFors = append(parser.activeFors, activeForFrame{depth: parser.groupDepth, allocations: allocations})
	parser.validateInline(args[bodyStart:], base+bodyStart)
	parser.activeFors = parser.activeFors[:len(parser.activeFors)-1]
}

func validForVariableToken(value string) bool {
	return len(value) == 3 && value[0] == '%' && value[1] == '%' && (value[2] < '0' || value[2] > '9')
}

func singlePercentForVariable(value string) bool {
	return len(value) == 2 && value[0] == '%' && (value[1] < '0' || value[1] > '9')
}

func matchingParen(src []byte, open int) int {
	depth := 0
	quoted := false
	for offset := open; offset < len(src); offset++ {
		if src[offset] == '^' {
			offset++
			continue
		}
		if src[offset] == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if src[offset] == '(' {
			depth++
		} else if src[offset] == ')' {
			depth--
			if depth == 0 {
				return offset
			}
		}
	}
	return -1
}

func (parser *batchParser) validateForL(set []byte, base int) {
	if dynamicBytes(set) {
		parser.unsupported("BAT9002", "dynamic FOR /L tuple was not resolved", base, base+len(set))
		return
	}
	parts := strings.Split(string(set), ",")
	if len(parts) != 3 {
		parser.error("BAT3005", "FOR /L requires a start, step, and end tuple", base, base+len(set))
		return
	}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || !signedDecimal(trimmed) {
			parser.error("BAT3005", "FOR /L tuple values must be literal decimal numbers when static", base, base+len(set))
			return
		}
	}
}

func (parser *batchParser) validateForF(optionWords []word, base int) int {
	allocations := 1
	if len(optionWords) == 0 {
		return allocations
	}
	optionText := make([]string, 0, len(optionWords))
	for _, current := range optionWords {
		optionText = append(optionText, strings.Trim(current.text, "\""))
	}
	for _, option := range strings.Fields(strings.Join(optionText, " ")) {
		lower := lowerASCII(option)
		switch {
		case lower == "usebackq":
		case strings.HasPrefix(lower, "eol="):
			value := option[len("eol="):]
			if dynamicText(value) {
				parser.unsupported("BAT9002", "dynamic FOR /F eol value was not resolved", optionWords[0].start, optionWords[len(optionWords)-1].end)
			} else if len(value) != 1 {
				parser.error("BAT3006", "FOR /F eol= requires exactly one character", optionWords[0].start, optionWords[len(optionWords)-1].end)
			}
		case strings.HasPrefix(lower, "skip="):
			value := option[len("skip="):]
			if dynamicText(value) {
				parser.unsupported("BAT9002", "dynamic FOR /F skip value was not resolved", optionWords[0].start, optionWords[len(optionWords)-1].end)
			} else if !decimalDigits(value) {
				parser.error("BAT3006", "FOR /F skip= requires a literal decimal number", optionWords[0].start, optionWords[len(optionWords)-1].end)
			}
		case strings.HasPrefix(lower, "delims="):
		case strings.HasPrefix(lower, "tokens="):
			count, valid := forTokenAllocation(option[len("tokens="):])
			if !valid {
				parser.error("BAT3006", "FOR /F tokens= selection is malformed", optionWords[0].start, optionWords[len(optionWords)-1].end)
			} else {
				allocations = count
			}
		default:
			parser.error("BAT3006", "FOR /F parsing keyword is not documented", optionWords[0].start, optionWords[len(optionWords)-1].end)
		}
	}
	return allocations
}

func forTokenAllocation(selection string) (int, bool) {
	if selection == "" {
		return 0, false
	}
	count := 0
	parts := strings.Split(selection, ",")
	for index, part := range parts {
		if part == "*" {
			if index != len(parts)-1 {
				return 0, false
			}
			count++
			continue
		}
		bounds := strings.Split(part, "-")
		if len(bounds) > 2 || !decimalDigits(bounds[0]) {
			return 0, false
		}
		start, _ := strconv.Atoi(bounds[0])
		if start < 1 {
			return 0, false
		}
		end := start
		if len(bounds) == 2 {
			if !decimalDigits(bounds[1]) {
				return 0, false
			}
			end, _ = strconv.Atoi(bounds[1])
			if end < start {
				return 0, false
			}
		}
		count += end - start + 1
	}
	return count, count > 0
}

func (parser *batchParser) validateSet(args []byte, base int) {
	start, end := trimSpace(args, 0, len(args))
	if start == end {
		return
	}
	words := splitWords(args[start:end], base+start)
	mode := ""
	if len(words) > 0 && (equalFoldASCII(words[0].text, "/a") || equalFoldASCII(words[0].text, "/p")) {
		mode = lowerASCII(words[0].text)
		start = words[0].end - base
		for start < end && isSpace(args[start]) {
			start++
		}
	}
	switch mode {
	case "/p":
		assignment := args[start:end]
		equals := strings.IndexByte(string(assignment), '=')
		if equals <= 0 {
			parser.error("BAT4001", "SET /P requires variable=prompt syntax", base+start, base+end)
		}
	case "/a":
		if start == end {
			parser.error("BAT4002", "SET /A requires an expression", base+end, base+end)
			return
		}
		parser.validateArithmetic(args[start:end], base+start)
	default:
		// SET, SET name, and SET name=value are all documented forms.
	}
}

func (parser *batchParser) validateCall(args []byte, base int) {
	words := splitWords(args, base)
	if len(words) == 0 {
		parser.error("BAT5001", "CALL requires a target", base, base)
		return
	}
	target := strings.Trim(words[0].text, "\"")
	if dynamicText(target) {
		parser.unsupported("BAT9002", "dynamic CALL target was not resolved", words[0].start, words[0].end)
		return
	}
	if strings.HasPrefix(target, ":") {
		parser.requireExtensions(words[0].start, words[0].end)
		label := strings.TrimSpace(strings.TrimPrefix(target, ":"))
		if label == "" {
			parser.error("BAT5001", "CALL label target is empty", words[0].start, words[0].end)
			return
		}
		parser.targets = append(parser.targets, staticTarget{text: label, span: parser.source.span(words[0].start, words[0].end), state: parser.extensions, call: true})
		return
	}
	extension := lowerASCII(filepath.Ext(target))
	if extension != "" && extension != ".bat" && extension != ".cmd" {
		parser.error("BAT5002", "literal CALL file target must use a .bat or .cmd extension", words[0].start, words[0].end)
	} else if extension == "" {
		parser.unsupported("BAT9002", "extensionless CALL target is accepted because Microsoft examples conflict with the parameter table", words[0].start, words[0].end)
	}
}

func (parser *batchParser) validateGoto(args []byte, base int) {
	start, end := trimSpace(args, 0, len(args))
	if start == end {
		parser.error("BAT6001", "GOTO requires a target label", base+start, base+end)
		return
	}
	target := strings.TrimSpace(string(args[start:end]))
	span := parser.source.span(base+start, base+end)
	if dynamicText(target) {
		parser.targets = append(parser.targets, staticTarget{text: target, span: span, dynamic: true, state: parser.extensions})
		parser.unsupported("BAT9002", "dynamic GOTO target was not resolved", base+start, base+end)
		return
	}
	if strings.ContainsAny(target, ";=") {
		parser.error("BAT6003", "GOTO label parameter cannot contain semicolon or equals separators", base+start, base+end)
		return
	}
	bare := strings.TrimPrefix(target, ":")
	parser.targets = append(parser.targets, staticTarget{
		text: bare, span: span, eof: equalFoldASCII(target, ":EOF"), state: parser.extensions,
	})
}

func (parser *batchParser) validateSetlocal(args []byte, base int) {
	words := splitWords(args, base)
	parser.featureStack = append(parser.featureStack, featureSnapshot{extensions: parser.extensions, delayed: parser.delayed})
	for _, current := range words {
		lower := lowerASCII(current.text)
		if dynamicText(lower) {
			parser.unsupported("BAT9002", "dynamic SETLOCAL option was not resolved", current.start, current.end)
			continue
		}
		switch lower {
		case "enableextensions":
			parser.extensions = FeatureEnabled
		case "disableextensions":
			parser.extensions = FeatureDisabled
		case "enabledelayedexpansion":
			parser.delayed = FeatureEnabled
		case "disabledelayedexpansion":
			parser.delayed = FeatureDisabled
		default:
			parser.error("BAT7001", "SETLOCAL option is not documented", current.start, current.end)
		}
	}
}

func (parser *batchParser) validateEndlocal(args []byte, base int) {
	start, end := trimSpace(args, 0, len(args))
	if start != end {
		parser.unsupported("BAT9002", "ENDLOCAL arguments are outside the documented syntax", base+start, base+end)
	}
	if len(parser.featureStack) == 0 {
		return
	}
	last := parser.featureStack[len(parser.featureStack)-1]
	parser.featureStack = parser.featureStack[:len(parser.featureStack)-1]
	parser.extensions, parser.delayed = last.extensions, last.delayed
}

func (parser *batchParser) validateShift(args []byte, base int) {
	words := splitWords(args, base)
	if len(words) == 0 {
		return
	}
	parser.requireExtensions(words[0].start, words[0].end)
	value := ""
	spanStart, spanEnd := words[0].start, words[0].end
	if equalFoldASCII(words[0].text, "/n") {
		if len(words) < 2 {
			parser.error("BAT7002", "SHIFT /N requires a value from 0 through 8", spanStart, spanEnd)
			return
		}
		value = words[1].text
		spanEnd = words[1].end
	} else if strings.HasPrefix(words[0].text, "/") {
		value = words[0].text[1:]
	} else {
		parser.unsupported("BAT9002", "SHIFT argument form is outside the documented syntax", spanStart, spanEnd)
		return
	}
	if dynamicText(value) {
		parser.unsupported("BAT9002", "dynamic SHIFT value was not resolved", spanStart, spanEnd)
		return
	}
	if len(value) != 1 || value[0] < '0' || value[0] > '8' {
		parser.error("BAT7002", "SHIFT value must be in the documented range 0 through 8", spanStart, spanEnd)
	}
}

func (parser *batchParser) requireExtensions(start, end int) {
	if parser.extensions == FeatureDisabled {
		parser.error("BAT7003", "construct requires command extensions, which are provably disabled", start, end)
	}
}

func (parser *batchParser) validateInline(src []byte, base int) {
	start, end := trimSpace(src, 0, len(src))
	if start == end || src[start] == '(' {
		return
	}
	parser.commandDepth++
	if parser.commandDepth > maxParserDepth {
		parser.unsupported("BAT9001", "parser nesting limit reached", base+start, base+minInt(start+1, end))
		parser.commandDepth--
		return
	}
	if src[start] == '@' {
		start++
	}
	nameEnd := start
	for nameEnd < end && !isSpace(src[nameEnd]) && src[nameEnd] != '(' && src[nameEnd] != ')' {
		nameEnd++
	}
	name, attached := commandName(string(src[start:nameEnd]))
	argsStart := nameEnd - len(attached)
	for argsStart < end && isSpace(src[argsStart]) {
		argsStart++
	}
	savedExtensions, savedDelayed := parser.extensions, parser.delayed
	known := parser.validateCommand(name, src[argsStart:end], base+argsStart, parser.source.span(base+start, base+nameEnd-len(attached)))
	parser.extensions, parser.delayed = savedExtensions, savedDelayed
	if !known {
		parser.unsupported("BAT9002", "command body was left opaque", base+start, base+nameEnd)
	}
	parser.commandDepth--
}

func (parser *batchParser) validateStaticTargets() {
	for _, target := range parser.targets {
		if target.dynamic || target.eof && target.state != FeatureDisabled {
			continue
		}
		if parser.labels[lowerASCII(strings.TrimSpace(target.text))] {
			continue
		}
		if target.call {
			parser.unsupported("BAT9002", "missing static CALL label was left indeterminate because CALL does not document missing-label behavior", target.span.Start.Offset, target.span.End.Offset)
		} else {
			parser.addDiagnostic(Diagnostic{Code: "BAT6002", Severity: SeverityError, Message: "static GOTO target label was not found", Span: target.span})
		}
	}
}

func (parser *batchParser) validateExpansions() {
	src := parser.source.Bytes
	for offset := 0; offset < len(src); {
		if src[offset] == '^' {
			offset += 2
			if offset > len(src) {
				offset = len(src)
			}
			continue
		}
		if src[offset] == '!' {
			end := indexByte(src, '!', offset+1)
			if end >= 0 {
				parser.unsupported("BAT9002", "delayed expansion value was not resolved", offset, end+1)
				offset = end + 1
				continue
			}
			offset++
			continue
		}
		if src[offset] != '%' {
			offset++
			continue
		}
		end, hardError, recognized := expansionEnd(src, offset)
		if hardError {
			parser.error("BAT5003", "%~ modifiers cannot be used with %*", offset, end)
		} else if !recognized {
			parser.unsupported("BAT9002", "percent expansion was outside the documented static subset", offset, end)
		}
		offset = end
	}
}

func expansionEnd(src []byte, start int) (end int, hardError, recognized bool) {
	if start+1 >= len(src) {
		return start + 1, false, false
	}
	if src[start+1] == '%' {
		if start+2 >= len(src) {
			return start + 2, false, false
		}
		if src[start+2] != '~' {
			return start + 3, false, src[start+2] < '0' || src[start+2] > '9'
		}
		return modifierExpansionEnd(src, start, start+3, true)
	}
	if src[start+1] >= '0' && src[start+1] <= '9' || src[start+1] == '*' {
		return start + 2, false, true
	}
	if src[start+1] == '~' {
		return modifierExpansionEnd(src, start, start+2, false)
	}
	closing := indexByte(src, '%', start+1)
	if closing >= 0 {
		return closing + 1, false, false
	}
	return start + 1, false, false
}

func modifierExpansionEnd(src []byte, start, index int, forVariable bool) (int, bool, bool) {
	valid := true
	for index < len(src) {
		current := src[index]
		if !forVariable && current == '*' {
			return index + 1, true, true
		}
		if !forVariable && current >= '0' && current <= '9' {
			return index + 1, false, valid
		}
		if forVariable && current >= 'A' && current <= 'Z' {
			return index + 1, false, valid
		}
		if forVariable && (current < '0' || current > '9') && modifierTerminal(src, index) {
			return index + 1, false, valid
		}
		if strings.ContainsRune("fdpnxsatz", rune(lowerByte(current))) {
			index++
			continue
		}
		if current == '$' {
			colon := indexByte(src, ':', index+1)
			if colon < 0 {
				return len(src), false, false
			}
			index = colon + 1
			continue
		}
		valid = false
		index++
	}
	return len(src), false, false
}

func modifierTerminal(src []byte, index int) bool {
	return src[index] != '$' && !strings.ContainsRune("fdpnxsatz", rune(lowerByte(src[index])))
}

func lowerByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func indexByte(src []byte, target byte, start int) int {
	for index := start; index < len(src); index++ {
		if src[index] == target {
			return index
		}
	}
	return -1
}

func dynamicBytes(value []byte) bool { return strings.ContainsAny(string(value), "%!") }
func dynamicText(value string) bool  { return strings.ContainsAny(value, "%!") }

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func signedDecimal(value string) bool {
	if strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		value = value[1:]
	}
	return decimalDigits(value)
}
