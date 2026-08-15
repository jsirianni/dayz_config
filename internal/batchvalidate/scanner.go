package batchvalidate

type tokenKind uint8

const (
	tokenText tokenKind = iota
	tokenWhitespace
	tokenNewline
	tokenEscaped
	tokenQuote
	tokenAt
	tokenColon
	tokenLParen
	tokenRParen
	tokenAmp
	tokenAnd
	tokenPipe
	tokenOr
	tokenLess
	tokenGreater
	tokenAppend
	tokenLessAmp
	tokenGreaterAmp
	tokenPercent
	tokenBang
)

type token struct {
	kind       tokenKind
	start, end int
}

func scan(src []byte) []token {
	tokens := make([]token, 0, len(src)/2+1)
	quoted := false
	for offset := 0; offset < len(src); {
		start := offset
		if src[offset] == '\r' || src[offset] == '\n' {
			offset++
			if src[start] == '\r' && offset < len(src) && src[offset] == '\n' {
				offset++
			}
			tokens = append(tokens, token{kind: tokenNewline, start: start, end: offset})
			quoted = false
			continue
		}
		if src[offset] == '^' {
			offset++
			if offset < len(src) && src[offset] != '\r' && src[offset] != '\n' {
				offset++
			}
			tokens = append(tokens, token{kind: tokenEscaped, start: start, end: offset})
			continue
		}
		if src[offset] == '"' {
			offset++
			quoted = !quoted
			tokens = append(tokens, token{kind: tokenQuote, start: start, end: offset})
			continue
		}
		if src[offset] == '%' || src[offset] == '!' {
			kind := tokenPercent
			if src[offset] == '!' {
				kind = tokenBang
			}
			offset++
			tokens = append(tokens, token{kind: kind, start: start, end: offset})
			continue
		}
		if quoted {
			for offset < len(src) && src[offset] != '"' && src[offset] != '^' && src[offset] != '%' && src[offset] != '!' && src[offset] != '\r' && src[offset] != '\n' {
				offset++
			}
			tokens = append(tokens, token{kind: tokenText, start: start, end: offset})
			continue
		}
		switch src[offset] {
		case ' ', '\t', '\v', '\f':
			offset++
			for offset < len(src) && (src[offset] == ' ' || src[offset] == '\t' || src[offset] == '\v' || src[offset] == '\f') {
				offset++
			}
			tokens = append(tokens, token{kind: tokenWhitespace, start: start, end: offset})
		case '@':
			offset++
			tokens = append(tokens, token{kind: tokenAt, start: start, end: offset})
		case ':':
			offset++
			tokens = append(tokens, token{kind: tokenColon, start: start, end: offset})
		case '(':
			offset++
			tokens = append(tokens, token{kind: tokenLParen, start: start, end: offset})
		case ')':
			offset++
			tokens = append(tokens, token{kind: tokenRParen, start: start, end: offset})
		case '&':
			offset++
			kind := tokenAmp
			if offset < len(src) && src[offset] == '&' {
				offset++
				kind = tokenAnd
			}
			tokens = append(tokens, token{kind: kind, start: start, end: offset})
		case '|':
			offset++
			kind := tokenPipe
			if offset < len(src) && src[offset] == '|' {
				offset++
				kind = tokenOr
			}
			tokens = append(tokens, token{kind: kind, start: start, end: offset})
		case '<':
			offset++
			kind := tokenLess
			if offset < len(src) && src[offset] == '&' {
				offset++
				kind = tokenLessAmp
			}
			tokens = append(tokens, token{kind: kind, start: start, end: offset})
		case '>':
			offset++
			kind := tokenGreater
			if offset < len(src) {
				switch src[offset] {
				case '>':
					offset++
					kind = tokenAppend
				case '&':
					offset++
					kind = tokenGreaterAmp
				}
			}
			tokens = append(tokens, token{kind: kind, start: start, end: offset})
		default:
			offset++
			for offset < len(src) && !scannerBoundary(src[offset]) {
				offset++
			}
			tokens = append(tokens, token{kind: tokenText, start: start, end: offset})
		}
	}
	return tokens
}

func scannerBoundary(value byte) bool {
	switch value {
	case '\r', '\n', '^', '"', '%', '!', ' ', '\t', '\v', '\f', '@', ':', '(', ')', '&', '|', '<', '>':
		return true
	default:
		return false
	}
}

type physicalLine struct {
	start, contentEnd, end int
}

func physicalLines(src []byte) []physicalLine {
	if len(src) == 0 {
		return nil
	}
	lines := make([]physicalLine, 0, 16)
	start := 0
	for offset := 0; offset < len(src); {
		if src[offset] != '\r' && src[offset] != '\n' {
			offset++
			continue
		}
		contentEnd := offset
		offset++
		if src[contentEnd] == '\r' && offset < len(src) && src[offset] == '\n' {
			offset++
		}
		lines = append(lines, physicalLine{start: start, contentEnd: contentEnd, end: offset})
		start = offset
	}
	if start < len(src) {
		lines = append(lines, physicalLine{start: start, contentEnd: len(src), end: len(src)})
	}
	return lines
}

func trimSpace(src []byte, start, end int) (int, int) {
	for start < end && isSpace(src[start]) {
		start++
	}
	for end > start && isSpace(src[end-1]) {
		end--
	}
	return start, end
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\v' || value == '\f'
}

type word struct {
	text       string
	start, end int
}

func splitWords(src []byte, base int) []word {
	words := make([]word, 0, 8)
	for offset := 0; offset < len(src); {
		for offset < len(src) && isSpace(src[offset]) {
			offset++
		}
		if offset == len(src) {
			break
		}
		start := offset
		quoted := false
		for offset < len(src) {
			if src[offset] == '^' {
				offset++
				if offset < len(src) {
					offset++
				}
				continue
			}
			if src[offset] == '"' {
				quoted = !quoted
				offset++
				continue
			}
			if !quoted && isSpace(src[offset]) {
				break
			}
			offset++
		}
		words = append(words, word{text: string(src[start:offset]), start: base + start, end: base + offset})
	}
	return words
}

func equalFoldASCII(left string, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

func lowerASCII(value string) string {
	buffer := []byte(value)
	for index, current := range buffer {
		if current >= 'A' && current <= 'Z' {
			buffer[index] = current + ('a' - 'A')
		}
	}
	return string(buffer)
}
