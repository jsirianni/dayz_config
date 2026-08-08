package dayzinit

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenNumber
	tokenString
	tokenSymbol
)

type token struct {
	kind       tokenKind
	text       string
	start, end int
}

var operators = []string{
	">>>=", "<<=", ">>=", "++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"==", "!=", "<=", ">=", "&&", "||", "<<", ">>", "::", "->",
}

func lex(source *sourceFile, data []byte) ([]token, []Diagnostic) {
	lexer := lexerState{source: source, data: data, offset: source.bomBytes}
	lexer.run()
	return lexer.tokens, lexer.found.sorted()
}

type lexerState struct {
	source *sourceFile
	data   []byte
	offset int
	tokens []token
	found  diagnostics
}

func (lexer *lexerState) run() {
	for lexer.offset < len(lexer.data) {
		character := lexer.data[lexer.offset]
		if isSpaceByte(character) {
			lexer.offset++
			continue
		}
		if character == '/' && lexer.peekByte(1) == '/' {
			lexer.skipLineComment()
			continue
		}
		if character == '/' && lexer.peekByte(1) == '*' {
			lexer.skipBlockComment()
			continue
		}
		r, _ := utf8.DecodeRune(lexer.data[lexer.offset:])
		switch {
		case isIdentifierStartRune(r):
			lexer.scanIdentifier()
		case character >= '0' && character <= '9', character == '.' && lexer.peekByte(1) >= '0' && lexer.peekByte(1) <= '9':
			lexer.scanNumber()
		case character == '"':
			lexer.scanString()
		default:
			lexer.scanSymbol()
		}
	}
	lexer.tokens = append(lexer.tokens, token{kind: tokenEOF, start: len(lexer.data), end: len(lexer.data)})
}

func (lexer *lexerState) peekByte(ahead int) byte {
	if lexer.offset+ahead >= len(lexer.data) {
		return 0
	}
	return lexer.data[lexer.offset+ahead]
}

func (lexer *lexerState) skipLineComment() {
	lexer.offset += 2
	for lexer.offset < len(lexer.data) && lexer.data[lexer.offset] != '\n' {
		lexer.offset++
	}
}

func (lexer *lexerState) skipBlockComment() {
	start := lexer.offset
	lexer.offset += 2
	for lexer.offset+1 < len(lexer.data) {
		if lexer.data[lexer.offset] == '*' && lexer.data[lexer.offset+1] == '/' {
			lexer.offset += 2
			return
		}
		lexer.offset++
	}
	lexer.offset = len(lexer.data)
	lexer.found.add(Diagnostic{Code: "DZI1101", Message: "unterminated block comment", Hint: "close the comment with */", Span: lexer.source.span(start, len(lexer.data))})
}

func (lexer *lexerState) scanIdentifier() {
	start := lexer.offset
	for lexer.offset < len(lexer.data) {
		r, size := utf8.DecodeRune(lexer.data[lexer.offset:])
		if !isIdentifierContinueRune(r) {
			break
		}
		lexer.offset += size
	}
	lexer.tokens = append(lexer.tokens, token{kind: tokenIdentifier, text: string(lexer.data[start:lexer.offset]), start: start, end: lexer.offset})
}

func (lexer *lexerState) scanNumber() {
	start := lexer.offset
	if lexer.data[lexer.offset] == '0' && (lexer.peekByte(1) == 'x' || lexer.peekByte(1) == 'X') {
		lexer.offset += 2
		digitStart := lexer.offset
		for isHexByte(lexer.peekByte(0)) {
			lexer.offset++
		}
		if lexer.offset == digitStart {
			lexer.found.add(Diagnostic{Code: "DZI1102", Message: "hexadecimal literal has no digits", Hint: "add hexadecimal digits after 0x", Span: lexer.source.span(start, lexer.offset)})
		}
	} else {
		if lexer.data[lexer.offset] == '.' {
			lexer.offset++
		}
		for lexer.peekByte(0) >= '0' && lexer.peekByte(0) <= '9' {
			lexer.offset++
		}
		if lexer.peekByte(0) == '.' {
			lexer.offset++
			for lexer.peekByte(0) >= '0' && lexer.peekByte(0) <= '9' {
				lexer.offset++
			}
		}
		if lexer.peekByte(0) == 'e' || lexer.peekByte(0) == 'E' {
			exponentStart := lexer.offset
			lexer.offset++
			if lexer.peekByte(0) == '+' || lexer.peekByte(0) == '-' {
				lexer.offset++
			}
			digitStart := lexer.offset
			for lexer.peekByte(0) >= '0' && lexer.peekByte(0) <= '9' {
				lexer.offset++
			}
			if lexer.offset == digitStart {
				lexer.found.add(Diagnostic{Code: "DZI1103", Message: "floating-point exponent has no digits", Hint: "add exponent digits or remove the exponent marker", Span: lexer.source.span(exponentStart, lexer.offset)})
			}
		}
	}
	if lexer.peekByte(0) == 'f' || lexer.peekByte(0) == 'F' {
		lexer.offset++
	}
	lexer.tokens = append(lexer.tokens, token{kind: tokenNumber, text: string(lexer.data[start:lexer.offset]), start: start, end: lexer.offset})
}

func (lexer *lexerState) scanString() {
	start := lexer.offset
	lexer.offset++
	for lexer.offset < len(lexer.data) {
		switch lexer.data[lexer.offset] {
		case '"':
			lexer.offset++
			lexer.tokens = append(lexer.tokens, token{kind: tokenString, text: string(lexer.data[start:lexer.offset]), start: start, end: lexer.offset})
			return
		case '\r', '\n':
			lexer.found.add(Diagnostic{Code: "DZI1104", Message: "unterminated string literal", Hint: "close the string before the end of the line", Span: lexer.source.span(start, lexer.offset)})
			return
		case '\\':
			escapeStart := lexer.offset
			lexer.offset++
			if lexer.offset >= len(lexer.data) {
				lexer.found.add(Diagnostic{Code: "DZI1105", Message: "unterminated string escape", Hint: "complete the escape and close the string", Span: lexer.source.span(escapeStart, lexer.offset)})
				return
			}
			escaped := lexer.data[lexer.offset]
			if !lexer.scanEscape() {
				end := lexer.offset + 1
				if end > len(lexer.data) {
					end = len(lexer.data)
				}
				lexer.found.add(Diagnostic{Code: "DZI1106", Message: fmt.Sprintf("unsupported string escape \\%c", escaped), Hint: "use a standard escaped quote, slash, control character, or hexadecimal/Unicode escape", Span: lexer.source.span(escapeStart, end)})
				if lexer.offset < len(lexer.data) {
					lexer.offset++
				}
			}
		default:
			_, size := utf8.DecodeRune(lexer.data[lexer.offset:])
			lexer.offset += size
		}
	}
	lexer.found.add(Diagnostic{Code: "DZI1104", Message: "unterminated string literal", Hint: "close the string with a double quote", Span: lexer.source.span(start, lexer.offset)})
}

func (lexer *lexerState) scanEscape() bool {
	character := lexer.data[lexer.offset]
	if strings.ContainsRune(`"\\nrt0bf`, rune(character)) {
		lexer.offset++
		return true
	}
	digits := 0
	switch character {
	case 'x':
		digits = 2
	case 'u':
		digits = 4
	default:
		return false
	}
	lexer.offset++
	if lexer.offset+digits > len(lexer.data) {
		return false
	}
	for count := 0; count < digits; count++ {
		if !isHexByte(lexer.data[lexer.offset+count]) {
			return false
		}
	}
	lexer.offset += digits
	return true
}

func (lexer *lexerState) scanSymbol() {
	start := lexer.offset
	remaining := string(lexer.data[start:])
	for _, operator := range operators {
		if strings.HasPrefix(remaining, operator) {
			lexer.offset += len(operator)
			lexer.tokens = append(lexer.tokens, token{kind: tokenSymbol, text: operator, start: start, end: lexer.offset})
			return
		}
	}
	if strings.ContainsRune(`{}()[];,.:?~+-*/%&|^!<>=`, rune(lexer.data[start])) {
		lexer.offset++
		lexer.tokens = append(lexer.tokens, token{kind: tokenSymbol, text: string(lexer.data[start:lexer.offset]), start: start, end: lexer.offset})
		return
	}
	_, size := utf8.DecodeRune(lexer.data[start:])
	lexer.offset += size
	lexer.found.add(Diagnostic{Code: "DZI1107", Message: fmt.Sprintf("unexpected character %q", string(lexer.data[start:lexer.offset])), Hint: "remove or replace the character with valid DayZ Enforce syntax", Span: lexer.source.span(start, lexer.offset)})
}

func isSpaceByte(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n' || character == '\f'
}

func isHexByte(character byte) bool {
	return (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')
}

func isIdentifierStartRune(character rune) bool {
	return character == '_' || unicode.IsLetter(character)
}

func isIdentifierContinueRune(character rune) bool {
	return isIdentifierStartRune(character) || unicode.IsDigit(character)
}
