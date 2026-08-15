package batchvalidate

import "strings"

type arithmeticTokenKind uint8

const (
	arithmeticOperand arithmeticTokenKind = iota
	arithmeticOperator
	arithmeticLeftParen
	arithmeticRightParen
	arithmeticEOF
)

type arithmeticToken struct {
	kind       arithmeticTokenKind
	text       string
	start, end int
	lvalue     bool
}

type arithmeticParser struct {
	owner  *batchParser
	tokens []arithmeticToken
	index  int
	depth  int
	failed bool
}

type expressionInfo struct{ lvalue bool }

func (parser *batchParser) validateArithmetic(src []byte, base int) {
	start, end := trimSpace(src, 0, len(src))
	if end-start >= 2 && src[start] == '"' && src[end-1] == '"' {
		start++
		end--
	}
	start, end = trimSpace(src, start, end)
	if start == end {
		parser.error("BAT4002", "SET /A requires an expression", base+start, base+end)
		return
	}
	tokens := parser.scanArithmetic(src[start:end], base+start)
	state := arithmeticParser{owner: parser, tokens: tokens}
	state.parseExpression(1)
	if !state.failed && state.peek().kind != arithmeticEOF {
		current := state.peek()
		parser.error("BAT4003", "unexpected token in SET /A expression", current.start, current.end)
	}
}

func (parser *batchParser) scanArithmetic(src []byte, base int) []arithmeticToken {
	tokens := make([]arithmeticToken, 0, len(src)/2+1)
	for offset := 0; offset < len(src); {
		if isSpace(src[offset]) {
			offset++
			continue
		}
		start := offset
		if src[offset] >= '0' && src[offset] <= '9' {
			offset++
			for offset < len(src) && (src[offset] >= '0' && src[offset] <= '9' || src[offset] >= 'a' && src[offset] <= 'z' || src[offset] >= 'A' && src[offset] <= 'Z') {
				offset++
			}
			text := string(src[start:offset])
			if !validArithmeticNumber(text) {
				parser.error("BAT4004", "SET /A numeric literal is not valid decimal, hexadecimal, or octal syntax", base+start, base+offset)
			}
			tokens = append(tokens, arithmeticToken{kind: arithmeticOperand, text: text, start: base + start, end: base + offset})
			continue
		}
		if src[offset] == '%' {
			closing := indexByte(src, '%', offset+1)
			if closing >= 0 {
				offset = closing + 1
				parser.unsupported("BAT9002", "dynamic SET /A expansion was not resolved", base+start, base+offset)
				tokens = append(tokens, arithmeticToken{kind: arithmeticOperand, text: string(src[start:offset]), start: base + start, end: base + offset, lvalue: true})
				continue
			}
		}
		if src[offset] == '(' || src[offset] == ')' {
			kind := arithmeticLeftParen
			if src[offset] == ')' {
				kind = arithmeticRightParen
			}
			offset++
			tokens = append(tokens, arithmeticToken{kind: kind, text: string(src[start:offset]), start: base + start, end: base + offset})
			continue
		}
		if operator := arithmeticOperatorAt(src[offset:]); operator != "" {
			offset += len(operator)
			tokens = append(tokens, arithmeticToken{kind: arithmeticOperator, text: operator, start: base + start, end: base + offset})
			continue
		}
		offset++
		for offset < len(src) && !isSpace(src[offset]) && src[offset] != '(' && src[offset] != ')' && arithmeticOperatorAt(src[offset:]) == "" {
			offset++
		}
		tokens = append(tokens, arithmeticToken{kind: arithmeticOperand, text: string(src[start:offset]), start: base + start, end: base + offset, lvalue: true})
	}
	eof := base + len(src)
	return append(tokens, arithmeticToken{kind: arithmeticEOF, start: eof, end: eof})
}

func arithmeticOperatorAt(src []byte) string {
	for _, operator := range []string{"<<=", ">>=", "&&", "||", "*=", "/=", "%=", "+=", "-=", "&=", "^=", "|=", "<<", ">>", "!", "~", "-", "+", "*", "/", "%", "&", "^", "|", "=", ","} {
		if len(src) >= len(operator) && string(src[:len(operator)]) == operator {
			return operator
		}
	}
	return ""
}

func validArithmeticNumber(value string) bool {
	if strings.HasPrefix(lowerASCII(value), "0x") {
		if len(value) == 2 {
			return false
		}
		for index := 2; index < len(value); index++ {
			current := lowerByte(value[index])
			if (current < '0' || current > '9') && (current < 'a' || current > 'f') {
				return false
			}
		}
		return true
	}
	if len(value) > 1 && value[0] == '0' {
		for index := 1; index < len(value); index++ {
			if value[index] < '0' || value[index] > '7' {
				return false
			}
		}
		return true
	}
	return decimalDigits(value)
}

func (parser *arithmeticParser) parseExpression(minimumPrecedence int) expressionInfo {
	left, ok := parser.parsePrefix()
	if !ok {
		return expressionInfo{}
	}
	for !parser.failed {
		current := parser.peek()
		precedence, rightAssociative, assignment := arithmeticPrecedence(current.text)
		if current.kind != arithmeticOperator || precedence < minimumPrecedence {
			break
		}
		parser.index++
		nextPrecedence := precedence + 1
		if rightAssociative {
			nextPrecedence = precedence
		}
		if assignment && !left.lvalue {
			parser.owner.error("BAT4003", "SET /A assignment requires a variable on the left", current.start, current.end)
			parser.failed = true
			return expressionInfo{}
		}
		parser.parseExpression(nextPrecedence)
		left = expressionInfo{}
	}
	return left
}

func (parser *arithmeticParser) parsePrefix() (expressionInfo, bool) {
	current := parser.peek()
	if current.kind == arithmeticOperator && (current.text == "!" || current.text == "~" || current.text == "-" || current.text == "+") {
		if current.text == "+" {
			parser.owner.unsupported("BAT9002", "unary plus is outside the documented SET /A operator table", current.start, current.end)
		}
		parser.index++
		parser.depth++
		if parser.depth > maxParserDepth {
			parser.owner.unsupported("BAT9001", "SET /A expression nesting limit reached", current.start, current.end)
			parser.failed = true
			parser.depth--
			return expressionInfo{}, false
		}
		_, ok := parser.parsePrefix()
		parser.depth--
		return expressionInfo{}, ok
	}
	if current.kind == arithmeticOperand {
		parser.index++
		return expressionInfo{lvalue: current.lvalue}, true
	}
	if current.kind == arithmeticLeftParen {
		parser.index++
		parser.depth++
		if parser.depth > maxParserDepth {
			parser.owner.unsupported("BAT9001", "SET /A expression nesting limit reached", current.start, current.end)
			parser.failed = true
			parser.depth--
			return expressionInfo{}, false
		}
		inside := parser.parseExpression(1)
		parser.depth--
		if parser.failed {
			return expressionInfo{}, false
		}
		if parser.peek().kind != arithmeticRightParen {
			end := parser.peek().start
			parser.owner.error("BAT4003", "SET /A expression is missing a closing parenthesis", end, end)
			parser.failed = true
			return expressionInfo{}, false
		}
		parser.index++
		return inside, true
	}
	parser.owner.error("BAT4003", "SET /A expression requires an operand", current.start, current.end)
	parser.failed = true
	return expressionInfo{}, false
}

func (parser *arithmeticParser) peek() arithmeticToken { return parser.tokens[parser.index] }

func arithmeticPrecedence(operator string) (precedence int, rightAssociative, assignment bool) {
	switch operator {
	case ",":
		return 1, false, false
	case "=", "*=", "/=", "%=", "+=", "-=", "&=", "^=", "|=", "<<=", ">>=":
		return 2, true, true
	case "||":
		return 3, false, false
	case "&&":
		return 4, false, false
	case "|":
		return 5, false, false
	case "^":
		return 6, false, false
	case "&":
		return 7, false, false
	case "<<", ">>":
		return 8, false, false
	case "+", "-":
		return 9, false, false
	case "*", "/", "%":
		return 10, false, false
	default:
		return 0, false, false
	}
}
