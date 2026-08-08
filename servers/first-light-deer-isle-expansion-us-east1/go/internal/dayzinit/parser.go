package dayzinit

import (
	"fmt"
	"strings"
)

const maxParseDepth = 256

var declarationModifiers = map[string]bool{
	"autoptr": true, "const": true, "event": true, "external": true, "modded": true,
	"native": true, "notnull": true, "override": true, "owned": true, "private": true,
	"protected": true, "proto": true, "ref": true, "reference": true, "sealed": true,
	"static": true, "volatile": true,
}

var parameterModifiers = map[string]bool{
	"autoptr": true, "const": true, "inout": true, "notnull": true, "out": true,
	"owned": true, "ref": true, "reference": true,
}

func parse(source *sourceFile, tokens []token) (*program, []Diagnostic) {
	parser := parserState{source: source, tokens: append([]token(nil), tokens...)}
	result := &program{}
	for !parser.atEnd() && len(parser.found.items) < maxDiagnostics {
		start := parser.index
		declaration := parser.parseDeclaration(false, "")
		if declaration != nil {
			result.Declarations = append(result.Declarations, declaration)
		}
		if parser.index == start {
			parser.errorCurrent("DZI2101", "expected a top-level declaration", "declare a function, class, enum, typedef, or variable")
			parser.advance()
			parser.synchronizeTopLevel()
		}
	}
	return result, parser.found.sorted()
}

type parserState struct {
	source *sourceFile
	tokens []token
	index  int
	depth  int
	found  diagnostics
}

func (parser *parserState) current() token {
	if parser.index >= len(parser.tokens) {
		return parser.tokens[len(parser.tokens)-1]
	}
	return parser.tokens[parser.index]
}

func (parser *parserState) previous() token {
	if parser.index == 0 {
		return parser.current()
	}
	return parser.tokens[parser.index-1]
}

func (parser *parserState) peek(distance int) token {
	index := parser.index + distance
	if index >= len(parser.tokens) {
		return parser.tokens[len(parser.tokens)-1]
	}
	return parser.tokens[index]
}

func (parser *parserState) atEnd() bool {
	return parser.current().kind == tokenEOF
}

func (parser *parserState) advance() token {
	current := parser.current()
	if !parser.atEnd() {
		parser.index++
	}
	return current
}

func (parser *parserState) check(text string) bool {
	return parser.current().text == text
}

func (parser *parserState) match(texts ...string) bool {
	for _, text := range texts {
		if parser.check(text) {
			parser.advance()
			return true
		}
	}
	return false
}

func (parser *parserState) expect(text, message, hint string) token {
	if parser.check(text) {
		return parser.advance()
	}
	parser.errorCurrent("DZI2102", fmt.Sprintf("%s; found %s", message, parser.describeCurrent()), hint)
	return token{kind: tokenSymbol, text: text, start: parser.current().start, end: parser.current().start}
}

func (parser *parserState) expectIdentifier(message string) token {
	if parser.current().kind == tokenIdentifier {
		return parser.advance()
	}
	parser.errorCurrent("DZI2103", fmt.Sprintf("%s; found %s", message, parser.describeCurrent()), "use a valid identifier")
	return token{kind: tokenIdentifier, start: parser.current().start, end: parser.current().start}
}

func (parser *parserState) describeCurrent() string {
	current := parser.current()
	if current.kind == tokenEOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", current.text)
}

func (parser *parserState) errorCurrent(code, message, hint string) {
	current := parser.current()
	parser.found.add(Diagnostic{Code: code, Message: message, Hint: hint, Span: parser.source.span(current.start, current.end)})
}

func (parser *parserState) parseDeclaration(classMember bool, className string) *declaration {
	start := parser.current().start
	parser.skipAttributes()
	modifiers := parser.parseModifiers(declarationModifiers)
	if parser.match("class") {
		return parser.parseClass(start, modifiers)
	}
	if parser.match("enum") {
		return parser.parseEnum(start, modifiers)
	}
	if parser.match("typedef") {
		return parser.parseTypedef(start, modifiers)
	}
	if classMember && parser.match("~") {
		name := parser.expectIdentifier("expected destructor name")
		return parser.parseFunctionAfterName(start, typeRef{Name: "void", Span: parser.source.span(start, start)}, "~"+name.text, modifiers)
	}

	reference, ok := parser.parseType()
	if !ok {
		return nil
	}
	if classMember && parser.check("(") && reference.Name == className {
		return parser.parseFunctionAfterName(start, typeRef{Name: "void", Span: reference.Span}, reference.Name, modifiers)
	}
	if parser.match("~") {
		name := parser.expectIdentifier("expected function name after ~")
		return parser.parseFunctionAfterName(start, reference, "~"+name.text, modifiers)
	}
	name := parser.expectIdentifier("expected declaration name")
	if name.text == "" {
		parser.synchronizeDeclaration()
		return nil
	}
	if parser.check("(") {
		return parser.parseFunctionAfterName(start, reference, name.text, modifiers)
	}
	return parser.parseVariableAfterName(start, reference, name, modifiers)
}

func (parser *parserState) skipAttributes() {
	for parser.check("[") {
		start := parser.advance()
		depth := 1
		for depth > 0 && !parser.atEnd() {
			switch parser.advance().text {
			case "[":
				depth++
			case "]":
				depth--
			}
		}
		if depth != 0 {
			parser.found.add(Diagnostic{Code: "DZI2104", Message: "unterminated attribute list", Hint: "close the attribute list with ]", Span: parser.source.span(start.start, parser.current().end)})
			return
		}
	}
}

func (parser *parserState) parseModifiers(allowed map[string]bool) []string {
	var modifiers []string
	for parser.current().kind == tokenIdentifier && allowed[parser.current().text] {
		modifiers = append(modifiers, parser.advance().text)
	}
	return modifiers
}

func (parser *parserState) parseType() (typeRef, bool) {
	start := parser.current()
	if start.kind != tokenIdentifier {
		parser.errorCurrent("DZI2105", "expected a type", "provide a primitive or named DayZ type")
		return typeRef{}, false
	}
	parser.advance()
	reference := typeRef{Name: start.text, Span: parser.source.span(start.start, start.end)}
	if parser.match("<") {
		for !parser.atEnd() && !parser.check(">") && !parser.check(">>") {
			argument, ok := parser.parseType()
			if !ok {
				break
			}
			reference.Arguments = append(reference.Arguments, argument)
			if !parser.match(",") {
				break
			}
		}
		parser.consumeTypeClose()
	}
	for parser.check("[") && parser.peek(1).text == "]" {
		parser.advance()
		end := parser.advance()
		reference.ArrayDepth++
		reference.Span.End = parser.source.position(end.end)
	}
	return reference, true
}

func (parser *parserState) consumeTypeClose() {
	if parser.match(">") {
		return
	}
	if parser.check(">>") {
		current := &parser.tokens[parser.index]
		current.text = ">"
		current.start++
		return
	}
	parser.expect(">", "expected > after generic type arguments", "close the generic type")
}

func (parser *parserState) parseClass(start int, modifiers []string) *declaration {
	name := parser.expectIdentifier("expected class name")
	result := &declaration{Kind: declarationClass, Name: name.text, Modifiers: modifiers}
	if parser.check("<") {
		parser.skipTypeParameterList()
	}
	if parser.match(":", "extends") {
		base, ok := parser.parseType()
		if ok {
			result.Base = &base
		}
	}
	parser.expect("{", "expected { after class declaration", "open the class body")
	if !parser.enterDepth() {
		parser.synchronizeDeclaration()
		return result
	}
	for !parser.atEnd() && !parser.check("}") {
		memberStart := parser.index
		member := parser.parseDeclaration(true, name.text)
		if member != nil {
			result.Members = append(result.Members, member)
		}
		if parser.index == memberStart {
			parser.errorCurrent("DZI2106", "expected a class member declaration", "declare a field, method, or enum")
			parser.advance()
			parser.synchronizeDeclaration()
		}
	}
	parser.leaveDepth()
	end := parser.expect("}", "expected } after class body", "close the class body")
	if parser.match(";") {
		end = parser.previous()
	}
	result.Span = parser.source.span(start, end.end)
	return result
}

func (parser *parserState) skipTypeParameterList() {
	parser.advance()
	depth := 1
	for !parser.atEnd() && depth > 0 {
		switch parser.current().text {
		case "<":
			depth++
			parser.advance()
		case ">":
			depth--
			parser.advance()
		case ">>":
			depth -= 2
			parser.advance()
		default:
			parser.advance()
		}
	}
	if depth != 0 {
		parser.errorCurrent("DZI2113", "unterminated class type-parameter list", "close the type parameters with >")
	}
}

func (parser *parserState) parseEnum(start int, modifiers []string) *declaration {
	name := parser.expectIdentifier("expected enum name")
	result := &declaration{Kind: declarationEnum, Name: name.text, Modifiers: modifiers}
	if parser.match(":", "extends") {
		base, ok := parser.parseType()
		if ok {
			result.Base = &base
		}
	}
	parser.expect("{", "expected { after enum declaration", "open the enum body")
	for !parser.atEnd() && !parser.check("}") {
		parser.skipAttributes()
		member := parser.expectIdentifier("expected enum member name")
		item := enumMember{Name: member.text, Span: parser.source.span(member.start, member.end)}
		if parser.match("=") {
			item.Value = parser.parseExpression(1)
		}
		result.EnumMembers = append(result.EnumMembers, item)
		if !parser.match(",", ";") && !parser.check("}") {
			parser.errorCurrent("DZI2107", "expected comma, semicolon, or } after enum member", "separate enum members")
			parser.synchronizeEnum()
		}
	}
	end := parser.expect("}", "expected } after enum body", "close the enum body")
	if parser.match(";") {
		end = parser.previous()
	}
	result.Span = parser.source.span(start, end.end)
	return result
}

func (parser *parserState) parseTypedef(start int, modifiers []string) *declaration {
	reference, ok := parser.parseType()
	if !ok {
		parser.synchronizeDeclaration()
		return nil
	}
	name := parser.expectIdentifier("expected typedef alias")
	end := parser.expect(";", "expected ; after typedef", "terminate the typedef with ;")
	return &declaration{Kind: declarationTypedef, Name: name.text, Type: reference, Modifiers: modifiers, Span: parser.source.span(start, end.end)}
}

func (parser *parserState) parseFunctionAfterName(start int, reference typeRef, name string, modifiers []string) *declaration {
	result := &declaration{Kind: declarationFunction, Name: name, Type: reference, Modifiers: modifiers}
	parser.expect("(", "expected ( after function name", "open the parameter list")
	if !parser.check(")") {
		for !parser.atEnd() {
			parameterStart := parser.current().start
			parameterModifiers := parser.parseModifiers(parameterModifiers)
			parameterType, ok := parser.parseType()
			if !ok {
				parser.synchronizeParameters()
				break
			}
			parameterName := parser.expectIdentifier("expected parameter name")
			item := parameter{Name: parameterName.text, Type: parameterType, Modifiers: parameterModifiers}
			for parser.match("[") {
				if !parser.check("]") {
					parser.parseExpression(1)
				}
				parser.expect("]", "expected ] after parameter array declarator", "close the array declarator")
				item.Type.ArrayDepth++
			}
			if parser.match("=") {
				item.Default = parser.parseExpression(1)
			}
			item.Span = parser.source.span(parameterStart, parser.previous().end)
			result.Parameters = append(result.Parameters, item)
			if !parser.match(",") {
				break
			}
			if parser.check(")") {
				break
			}
		}
	}
	parser.expect(")", "expected ) after parameters", "close the parameter list")
	if parser.match(";") {
		result.Span = parser.source.span(start, parser.previous().end)
		return result
	}
	if !parser.check("{") {
		parser.errorCurrent("DZI2108", "expected function body or ;", "add a function body or terminate an external declaration")
		parser.synchronizeDeclaration()
		result.Span = parser.source.span(start, parser.previous().end)
		return result
	}
	result.Body = parser.parseBlock()
	result.HasBody = true
	parser.match(";")
	result.Span = parser.source.span(start, parser.previous().end)
	return result
}

func (parser *parserState) parseVariableAfterName(start int, reference typeRef, first token, modifiers []string) *declaration {
	result := &declaration{Kind: declarationVariable, Type: reference, Modifiers: modifiers}
	currentName := first
	for {
		item := variable{Name: currentName.text, Span: parser.source.span(currentName.start, currentName.end)}
		for parser.match("[") {
			if !parser.check("]") {
				parser.parseExpression(1)
			}
			end := parser.expect("]", "expected ] after array declarator", "close the array declarator")
			item.ArrayDepth++
			item.Span.End = parser.source.position(end.end)
		}
		if parser.match("=") {
			item.Initializer = parser.parseExpression(1)
			if item.Initializer != nil {
				item.Span.End = item.Initializer.Span.End
			}
		}
		result.Variables = append(result.Variables, item)
		if !parser.match(",") {
			break
		}
		currentName = parser.expectIdentifier("expected variable name after comma")
		if currentName.text == "" {
			break
		}
	}
	end := parser.expect(";", "expected ; after variable declaration", "terminate the declaration with ;")
	result.Span = parser.source.span(start, end.end)
	if len(result.Variables) > 0 {
		result.Name = result.Variables[0].Name
	}
	return result
}

func (parser *parserState) parseBlock() *statement {
	start := parser.expect("{", "expected {", "open the statement block")
	result := &statement{Kind: statementBlock}
	if !parser.enterDepth() {
		parser.synchronizeStatement()
		result.Span = parser.source.span(start.start, parser.previous().end)
		return result
	}
	for !parser.atEnd() && !parser.check("}") {
		item := parser.parseStatement()
		if item != nil {
			result.Statements = append(result.Statements, item)
		}
	}
	parser.leaveDepth()
	end := parser.expect("}", "expected } after block", "close the statement block")
	result.Span = parser.source.span(start.start, end.end)
	return result
}

func (parser *parserState) parseStatement() *statement {
	start := parser.current().start
	switch {
	case parser.match(";"):
		return &statement{Kind: statementEmpty, Span: parser.source.span(start, parser.previous().end)}
	case parser.check("{"):
		return parser.parseBlock()
	case parser.match("if"):
		return parser.parseIf(start)
	case parser.match("switch"):
		return parser.parseSwitch(start)
	case parser.match("while"):
		return parser.parseWhile(start)
	case parser.match("for"):
		return parser.parseFor(start)
	case parser.match("foreach"):
		return parser.parseForeach(start)
	case parser.match("return"):
		result := &statement{Kind: statementReturn}
		if !parser.check(";") {
			result.Expression = parser.parseExpression(1)
		}
		end := parser.expect(";", "expected ; after return", "terminate return with ;")
		result.Span = parser.source.span(start, end.end)
		return result
	case parser.match("break"):
		end := parser.expect(";", "expected ; after break", "terminate break with ;")
		return &statement{Kind: statementBreak, Span: parser.source.span(start, end.end)}
	case parser.match("continue"):
		end := parser.expect(";", "expected ; after continue", "terminate continue with ;")
		return &statement{Kind: statementContinue, Span: parser.source.span(start, end.end)}
	case parser.match("delete"):
		expression := parser.parseExpression(1)
		end := parser.expect(";", "expected ; after delete", "terminate delete with ;")
		return &statement{Kind: statementDelete, Expression: expression, Span: parser.source.span(start, end.end)}
	case parser.looksLikeLocalDeclaration():
		declaration := parser.parseDeclaration(false, "")
		return &statement{Kind: statementDeclaration, Declaration: declaration, Span: declaration.Span}
	default:
		expression := parser.parseExpression(1)
		end := parser.expect(";", "expected ; after expression", "terminate the expression statement with ;")
		return &statement{Kind: statementExpression, Expression: expression, Span: parser.source.span(start, end.end)}
	}
}

func (parser *parserState) parseIf(start int) *statement {
	parser.expect("(", "expected ( after if", "open the condition")
	condition := parser.parseExpression(1)
	parser.expect(")", "expected ) after if condition", "close the condition")
	result := &statement{Kind: statementIf, Condition: condition, Then: parser.parseStatement()}
	if parser.match("else") {
		result.Else = parser.parseStatement()
	}
	result.Span = parser.source.span(start, statementEnd(result.Then, result.Else))
	return result
}

func (parser *parserState) parseWhile(start int) *statement {
	parser.expect("(", "expected ( after while", "open the condition")
	condition := parser.parseExpression(1)
	parser.expect(")", "expected ) after while condition", "close the condition")
	body := parser.parseStatement()
	return &statement{Kind: statementWhile, Condition: condition, Then: body, Span: parser.source.span(start, statementEnd(body, nil))}
}

func (parser *parserState) parseFor(start int) *statement {
	parser.expect("(", "expected ( after for", "open the for clauses")
	result := &statement{Kind: statementFor}
	if parser.match(";") {
		result.Initializer = &statement{Kind: statementEmpty, Span: parser.source.span(parser.previous().start, parser.previous().end)}
	} else if parser.looksLikeLocalDeclaration() {
		declaration := parser.parseDeclaration(false, "")
		result.Initializer = &statement{Kind: statementDeclaration, Declaration: declaration, Span: declaration.Span}
	} else {
		expression := parser.parseExpression(1)
		end := parser.expect(";", "expected ; after for initializer", "separate for clauses with ;")
		result.Initializer = &statement{Kind: statementExpression, Expression: expression, Span: parser.source.span(expression.Span.Start.Offset, end.end)}
	}
	if !parser.check(";") {
		result.Condition = parser.parseExpression(1)
	}
	parser.expect(";", "expected ; after for condition", "separate for clauses with ;")
	if !parser.check(")") {
		result.Post = parser.parseExpression(1)
	}
	parser.expect(")", "expected ) after for clauses", "close the for clauses")
	result.Then = parser.parseStatement()
	result.Span = parser.source.span(start, statementEnd(result.Then, nil))
	return result
}

func (parser *parserState) parseForeach(start int) *statement {
	parser.expect("(", "expected ( after foreach", "open the foreach clause")
	result := &statement{Kind: statementForeach}
	for {
		parameterStart := parser.current().start
		reference, ok := parser.parseType()
		if !ok {
			break
		}
		name := parser.expectIdentifier("expected foreach iterator name")
		result.Iterators = append(result.Iterators, parameter{Name: name.text, Type: reference, Span: parser.source.span(parameterStart, name.end)})
		if !parser.match(",") {
			break
		}
	}
	parser.expect(":", "expected : before foreach collection", "separate iterators from the collection with :")
	result.Expression = parser.parseExpression(1)
	parser.expect(")", "expected ) after foreach clause", "close the foreach clause")
	result.Then = parser.parseStatement()
	result.Span = parser.source.span(start, statementEnd(result.Then, nil))
	return result
}

func (parser *parserState) parseSwitch(start int) *statement {
	parser.expect("(", "expected ( after switch", "open the switch expression")
	result := &statement{Kind: statementSwitch, Expression: parser.parseExpression(1)}
	parser.expect(")", "expected ) after switch expression", "close the switch expression")
	parser.expect("{", "expected { after switch", "open the switch body")
	for !parser.atEnd() && !parser.check("}") {
		caseStart := parser.current().start
		item := switchCase{}
		if parser.match("case") {
			item.Expression = parser.parseExpression(1)
			parser.expect(":", "expected : after case value", "terminate the case label with :")
		} else if parser.match("default") {
			item.Default = true
			parser.expect(":", "expected : after default", "terminate the default label with :")
		} else {
			parser.errorCurrent("DZI2111", "expected case, default, or } in switch", "add a switch label")
			parser.synchronizeSwitch()
			continue
		}
		for !parser.atEnd() && !parser.check("case") && !parser.check("default") && !parser.check("}") {
			item.Statements = append(item.Statements, parser.parseStatement())
		}
		item.Span = parser.source.span(caseStart, parser.previous().end)
		result.Cases = append(result.Cases, item)
	}
	end := parser.expect("}", "expected } after switch", "close the switch body")
	result.Span = parser.source.span(start, end.end)
	return result
}

func (parser *parserState) looksLikeLocalDeclaration() bool {
	index := parser.index
	for index < len(parser.tokens) && declarationModifiers[parser.tokens[index].text] {
		index++
	}
	if index >= len(parser.tokens) || parser.tokens[index].kind != tokenIdentifier {
		return false
	}
	index++
	if index < len(parser.tokens) && parser.tokens[index].text == "<" {
		depth := 0
		for index < len(parser.tokens) {
			switch parser.tokens[index].text {
			case "<":
				depth++
			case ">":
				depth--
			case ">>":
				depth -= 2
			}
			index++
			if depth <= 0 {
				break
			}
		}
	}
	for index+1 < len(parser.tokens) && parser.tokens[index].text == "[" && parser.tokens[index+1].text == "]" {
		index += 2
	}
	return index < len(parser.tokens) && parser.tokens[index].kind == tokenIdentifier
}

func statementEnd(primary, secondary *statement) int {
	if secondary != nil {
		return secondary.Span.End.Offset
	}
	if primary != nil {
		return primary.Span.End.Offset
	}
	return 0
}

func (parser *parserState) enterDepth() bool {
	parser.depth++
	if parser.depth <= maxParseDepth {
		return true
	}
	parser.errorCurrent("DZI2112", fmt.Sprintf("syntax nesting exceeds the limit of %d", maxParseDepth), "reduce deeply nested blocks or expressions")
	return false
}

func (parser *parserState) leaveDepth() {
	if parser.depth > 0 {
		parser.depth--
	}
}

func (parser *parserState) synchronizeTopLevel() {
	for !parser.atEnd() {
		if parser.previous().text == ";" || parser.check("class") || parser.check("enum") || parser.check("typedef") {
			return
		}
		parser.advance()
	}
}

func (parser *parserState) synchronizeDeclaration() {
	for !parser.atEnd() && !parser.check("}") {
		if parser.advance().text == ";" {
			return
		}
	}
}

func (parser *parserState) synchronizeStatement() {
	for !parser.atEnd() && !parser.check("}") && !parser.check("case") && !parser.check("default") {
		if parser.advance().text == ";" {
			return
		}
	}
}

func (parser *parserState) synchronizeParameters() {
	for !parser.atEnd() && !parser.check(")") && !parser.check(",") {
		parser.advance()
	}
}

func (parser *parserState) synchronizeEnum() {
	for !parser.atEnd() && !parser.check("}") {
		if parser.match(",", ";") {
			return
		}
		parser.advance()
	}
}

func (parser *parserState) synchronizeSwitch() {
	for !parser.atEnd() && !parser.check("case") && !parser.check("default") && !parser.check("}") {
		parser.advance()
	}
}

func modifierCount(modifiers []string, name string) int {
	count := 0
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, name) {
			count++
		}
	}
	return count
}
