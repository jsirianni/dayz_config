package dayzinit

var binaryPrecedence = map[string]int{
	"=": 1, "+=": 1, "-=": 1, "*=": 1, "/=": 1, "%=": 1, "&=": 1, "|=": 1, "^=": 1, "<<=": 1, ">>=": 1,
	"||": 3, "&&": 4, "|": 5, "^": 6, "&": 7, "==": 8, "!=": 8,
	"<": 9, "<=": 9, ">": 9, ">=": 9, "<<": 10, ">>": 10,
	"+": 11, "-": 11, "*": 12, "/": 12, "%": 12,
}

func (parser *parserState) parseExpression(minimumPrecedence int) *expression {
	if !parser.enterDepth() {
		parser.leaveDepth()
		return nil
	}
	left := parser.parsePrefixExpression()
	if left == nil {
		parser.leaveDepth()
		return nil
	}
	left = parser.parsePostfixExpression(left)
	for {
		if parser.check("?") && minimumPrecedence <= 2 {
			parser.advance()
			whenTrue := parser.parseExpression(1)
			parser.expect(":", "expected : in conditional expression", "separate the true and false expressions with :")
			whenFalse := parser.parseExpression(2)
			left = &expression{Kind: expressionTernary, Left: left, Right: whenTrue, Third: whenFalse, Span: parser.source.span(left.Span.Start.Offset, expressionEnd(whenFalse, left))}
			continue
		}
		operator := parser.current().text
		precedence, ok := binaryPrecedence[operator]
		if !ok || precedence < minimumPrecedence {
			break
		}
		parser.advance()
		nextMinimum := precedence + 1
		if precedence == 1 {
			nextMinimum = precedence
		}
		right := parser.parseExpression(nextMinimum)
		left = &expression{Kind: expressionBinary, Text: operator, Left: left, Right: right, Span: parser.source.span(left.Span.Start.Offset, expressionEnd(right, left))}
	}
	parser.leaveDepth()
	return left
}

func (parser *parserState) parsePrefixExpression() *expression {
	current := parser.current()
	if parser.match("!", "~", "+", "-", "++", "--") {
		right := parser.parseExpression(13)
		return &expression{Kind: expressionUnary, Text: current.text, Right: right, Span: parser.source.span(current.start, expressionEnd(right, nil))}
	}
	if parser.match("new") {
		reference, ok := parser.parseType()
		if !ok {
			return nil
		}
		result := &expression{Kind: expressionNew, Type: reference, Span: parser.source.span(current.start, reference.Span.End.Offset)}
		if parser.check("(") {
			result.Args = parser.parseCallArguments()
			result.Span.End = parser.source.position(parser.previous().end)
		}
		return result
	}
	if parser.match("{") {
		result := &expression{Kind: expressionArray}
		if !parser.check("}") {
			for {
				result.Args = append(result.Args, parser.parseExpression(1))
				if !parser.match(",") {
					break
				}
				if parser.check("}") {
					break
				}
			}
		}
		end := parser.expect("}", "expected } after array literal", "close the array literal")
		result.Span = parser.source.span(current.start, end.end)
		return result
	}
	if parser.match("(") {
		if parser.looksLikeCast() {
			reference, _ := parser.parseType()
			parser.expect(")", "expected ) after cast type", "close the cast type")
			value := parser.parseExpression(13)
			return &expression{Kind: expressionCast, Type: reference, Right: value, Span: parser.source.span(current.start, expressionEnd(value, nil))}
		}
		result := parser.parseExpression(1)
		end := parser.expect(")", "expected ) after expression", "close the parenthesized expression")
		if result != nil {
			result.Span = parser.source.span(current.start, end.end)
		}
		return result
	}
	if current.kind == tokenIdentifier {
		parser.advance()
		return &expression{Kind: expressionIdentifier, Text: current.text, Span: parser.source.span(current.start, current.end)}
	}
	if current.kind == tokenNumber || current.kind == tokenString {
		parser.advance()
		return &expression{Kind: expressionLiteral, Text: current.text, Span: parser.source.span(current.start, current.end)}
	}
	parser.errorCurrent("DZI2201", "expected an expression; found "+parser.describeCurrent(), "provide a literal, identifier, call, or operator expression")
	if !parser.atEnd() {
		parser.advance()
	}
	return nil
}

func (parser *parserState) parsePostfixExpression(result *expression) *expression {
	for {
		switch {
		case parser.check("("):
			args := parser.parseCallArguments()
			result = &expression{Kind: expressionCall, Receiver: result, Args: args, Span: parser.source.span(result.Span.Start.Offset, parser.previous().end)}
		case parser.match(".", "->", "::"):
			member := parser.expectIdentifier("expected member name after access operator")
			result = &expression{Kind: expressionMember, Text: member.text, Receiver: result, Span: parser.source.span(result.Span.Start.Offset, member.end)}
		case parser.match("["):
			index := parser.parseExpression(1)
			end := parser.expect("]", "expected ] after index expression", "close the index expression")
			result = &expression{Kind: expressionIndex, Receiver: result, Right: index, Span: parser.source.span(result.Span.Start.Offset, end.end)}
		case parser.match("++", "--"):
			operator := parser.previous()
			result = &expression{Kind: expressionUnary, Text: "post" + operator.text, Left: result, Span: parser.source.span(result.Span.Start.Offset, operator.end)}
		default:
			return result
		}
	}
}

func (parser *parserState) parseCallArguments() []*expression {
	parser.expect("(", "expected (", "open the argument list")
	var arguments []*expression
	if !parser.check(")") {
		for {
			if parser.current().kind == tokenIdentifier && parser.peek(1).text == ":" {
				parser.advance()
				parser.advance()
			}
			arguments = append(arguments, parser.parseExpression(1))
			if !parser.match(",") {
				break
			}
			if parser.check(")") {
				break
			}
		}
	}
	parser.expect(")", "expected ) after arguments", "close the argument list")
	return arguments
}

func (parser *parserState) looksLikeCast() bool {
	index := parser.index
	if index >= len(parser.tokens) || parser.tokens[index].kind != tokenIdentifier {
		return false
	}
	index++
	if index < len(parser.tokens) && parser.tokens[index].text == "<" {
		depth := 1
		index++
		for index < len(parser.tokens) && depth > 0 {
			switch parser.tokens[index].text {
			case "<":
				depth++
			case ">":
				depth--
			case ">>":
				depth -= 2
			}
			index++
		}
	}
	for index+1 < len(parser.tokens) && parser.tokens[index].text == "[" && parser.tokens[index+1].text == "]" {
		index += 2
	}
	if index >= len(parser.tokens) || parser.tokens[index].text != ")" {
		return false
	}
	if index+1 >= len(parser.tokens) {
		return false
	}
	return startsExpression(parser.tokens[index+1])
}

func startsExpression(item token) bool {
	if item.kind == tokenIdentifier || item.kind == tokenNumber || item.kind == tokenString {
		return true
	}
	switch item.text {
	case "(", "{", "!", "~", "+", "-", "++", "--":
		return true
	default:
		return false
	}
}

func expressionEnd(primary, fallback *expression) int {
	if primary != nil {
		return primary.Span.End.Offset
	}
	if fallback != nil {
		return fallback.Span.End.Offset
	}
	return 0
}
