package dayzinit

import (
	"fmt"
	"strconv"
	"strings"
)

type valueKind uint8

const (
	valueUnknown valueKind = iota
	valueVoid
	valueBool
	valueInt
	valueFloat
	valueString
	valueNull
	valueObject
	valueArray
)

type valueType struct {
	kind valueKind
	name string
	elem *valueType
}

type symbol struct {
	typeInfo valueType
	offset   int
}

type scope struct {
	parent  *scope
	symbols map[string]symbol
}

type semanticState struct {
	source    *sourceFile
	program   *program
	found     diagnostics
	functions map[string][]*declaration
	classes   map[string]*declaration
}

type flowContext struct {
	function *declaration
	class    *declaration
	loops    int
	switches int
	future   map[string]int
}

func validateSemantics(source *sourceFile, program *program) []Diagnostic {
	state := semanticState{
		source: source, program: program,
		functions: make(map[string][]*declaration),
		classes:   make(map[string]*declaration),
	}
	state.buildTopLevel()
	state.validateInheritance()
	global := &scope{symbols: make(map[string]symbol)}
	for _, declaration := range program.Declarations {
		if declaration.Kind == declarationVariable {
			state.declareVariables(global, declaration, false)
		}
	}
	for _, declaration := range program.Declarations {
		state.validateDeclaration(declaration, global, nil)
	}
	return state.found.sorted()
}

func (state *semanticState) buildTopLevel() {
	classNames := make(map[string]*declaration)
	globalNames := make(map[string]*declaration)
	functionSignatures := make(map[string]*declaration)
	for _, declaration := range state.program.Declarations {
		state.validateDuplicateModifiers(declaration)
		switch declaration.Kind {
		case declarationClass, declarationEnum:
			if previous := classNames[declaration.Name]; previous != nil {
				state.add("DZI3001", declaration.Span, fmt.Sprintf("duplicate type declaration %q", declaration.Name), "rename or remove one declaration")
			} else {
				classNames[declaration.Name] = declaration
				if declaration.Kind == declarationClass {
					state.classes[declaration.Name] = declaration
				}
			}
		case declarationFunction:
			signature := functionSignature(declaration)
			if functionSignatures[signature] != nil {
				state.add("DZI3002", declaration.Span, fmt.Sprintf("duplicate function signature %s", signature), "remove or change one declaration")
			} else {
				functionSignatures[signature] = declaration
			}
			state.functions[declaration.Name] = append(state.functions[declaration.Name], declaration)
		case declarationVariable:
			for _, variable := range declaration.Variables {
				if globalNames[variable.Name] != nil {
					state.add("DZI3003", variable.Span, fmt.Sprintf("duplicate global variable %q", variable.Name), "rename or remove the duplicate")
				} else {
					globalNames[variable.Name] = declaration
				}
			}
		case declarationTypedef:
			if globalNames[declaration.Name] != nil {
				state.add("DZI3003", declaration.Span, fmt.Sprintf("duplicate global declaration %q", declaration.Name), "rename or remove the duplicate")
			} else {
				globalNames[declaration.Name] = declaration
			}
		}
	}
}

func (state *semanticState) validateDeclaration(declaration *declaration, global *scope, class *declaration) {
	state.validateDuplicateModifiers(declaration)
	if modifierCount(declaration.Modifiers, "modded") > 0 && declaration.Kind != declarationClass {
		state.add("DZI3005", declaration.Span, "modded modifier is only valid on a class declaration", "remove modded or apply it to a class")
	}
	if modifierCount(declaration.Modifiers, "override") > 0 && (declaration.Kind != declarationFunction || class == nil) {
		state.add("DZI3006", declaration.Span, "override modifier requires a class method", "remove override from this declaration")
	}
	if declaration.Kind == declarationFunction {
		if declaration.HasBody && (hasModifier(declaration.Modifiers, "proto") || hasModifier(declaration.Modifiers, "native") || hasModifier(declaration.Modifiers, "external")) {
			state.add("DZI3007", declaration.Span, "proto, native, and external functions cannot have a script body", "remove the body or the external declaration modifier")
		}
		state.validateFunction(declaration, global, class)
		return
	}
	if declaration.Kind == declarationClass {
		state.validateClass(declaration, global)
		return
	}
	if declaration.Kind == declarationEnum {
		seen := make(map[string]bool)
		for _, member := range declaration.EnumMembers {
			if seen[member.Name] {
				state.add("DZI3008", member.Span, fmt.Sprintf("duplicate enum member %q", member.Name), "rename or remove the duplicate member")
			}
			seen[member.Name] = true
			state.inferExpression(member.Value, global, flowContext{})
		}
	}
	if declaration.Kind == declarationVariable {
		state.validateVariableInitializers(declaration, global, flowContext{class: class})
	}
}

func (state *semanticState) validateClass(class *declaration, global *scope) {
	fieldNames := make(map[string]bool)
	methodSignatures := make(map[string]bool)
	for _, member := range class.Members {
		state.validateDuplicateModifiers(member)
		switch member.Kind {
		case declarationVariable:
			for _, variable := range member.Variables {
				if fieldNames[variable.Name] {
					state.add("DZI3009", variable.Span, fmt.Sprintf("duplicate field %q in class %s", variable.Name, class.Name), "rename or remove the duplicate field")
				}
				fieldNames[variable.Name] = true
			}
		case declarationFunction:
			signature := functionSignature(member)
			if methodSignatures[signature] {
				state.add("DZI3010", member.Span, fmt.Sprintf("duplicate method signature %s in class %s", signature, class.Name), "remove or change one method")
			}
			methodSignatures[signature] = true
		}
		state.validateDeclaration(member, global, class)
	}
}

func (state *semanticState) validateFunction(function *declaration, global *scope, class *declaration) {
	functionScope := &scope{parent: global, symbols: make(map[string]symbol)}
	defaultSeen := false
	for _, parameter := range function.Parameters {
		if _, exists := functionScope.symbols[parameter.Name]; exists {
			state.add("DZI3011", parameter.Span, fmt.Sprintf("duplicate parameter %q", parameter.Name), "rename or remove the duplicate parameter")
		} else {
			functionScope.symbols[parameter.Name] = symbol{typeInfo: typeFromRef(parameter.Type), offset: parameter.Span.Start.Offset}
		}
		if parameter.Default != nil {
			defaultSeen = true
			actual := state.inferExpression(parameter.Default, functionScope, flowContext{function: function, class: class})
			state.requireCompatible(typeFromRef(parameter.Type), actual, parameter.Default.Span, "default parameter value")
		} else if defaultSeen {
			state.add("DZI3012", parameter.Span, fmt.Sprintf("required parameter %q follows a default parameter", parameter.Name), "move required parameters before default parameters")
		}
		state.validateParameterModifiers(parameter)
	}
	if !function.HasBody || function.Body == nil {
		return
	}
	context := flowContext{function: function, class: class}
	state.validateStatement(function.Body, functionScope, context)
	if typeFromRef(function.Type).kind != valueVoid && !statementDefinitelyReturns(function.Body) {
		state.add("DZI3013", function.Span, fmt.Sprintf("not all reachable paths in %s return a value", function.Name), "return a compatible value on every reachable path")
	}
}

func (state *semanticState) validateParameterModifiers(parameter parameter) {
	seen := make(map[string]bool)
	for _, modifier := range parameter.Modifiers {
		if seen[modifier] {
			state.add("DZI3014", parameter.Span, fmt.Sprintf("duplicate parameter modifier %q", modifier), "remove the duplicate modifier")
		}
		seen[modifier] = true
	}
	if seen["out"] && seen["inout"] {
		state.add("DZI3015", parameter.Span, "parameter cannot be both out and inout", "choose one parameter modifier")
	}
}

func (state *semanticState) validateDuplicateModifiers(declaration *declaration) {
	seen := make(map[string]bool)
	for _, modifier := range declaration.Modifiers {
		if seen[modifier] {
			state.add("DZI3016", declaration.Span, fmt.Sprintf("duplicate modifier %q", modifier), "remove the duplicate modifier")
		}
		seen[modifier] = true
	}
}

func (state *semanticState) validateVariableInitializers(declaration *declaration, current *scope, context flowContext) {
	expected := typeFromRef(declaration.Type)
	for _, variable := range declaration.Variables {
		actualExpected := expected
		for count := 0; count < variable.ArrayDepth; count++ {
			copy := actualExpected
			actualExpected = valueType{kind: valueArray, elem: &copy}
		}
		actual := state.inferExpression(variable.Initializer, current, context)
		if variable.Initializer != nil {
			state.requireCompatible(actualExpected, actual, variable.Initializer.Span, fmt.Sprintf("initializer for %s", variable.Name))
		}
	}
}

func (state *semanticState) validateStatement(item *statement, current *scope, context flowContext) {
	if item == nil {
		return
	}
	switch item.Kind {
	case statementBlock:
		blockScope := &scope{parent: current, symbols: make(map[string]symbol)}
		future := collectBlockDeclarations(item)
		for _, child := range item.Statements {
			state.validateStatementWithFuture(child, blockScope, context, future)
		}
	case statementDeclaration:
		state.validateVariableInitializers(item.Declaration, current, context)
		state.declareVariables(current, item.Declaration, true)
	case statementExpression, statementDelete:
		state.inferExpression(item.Expression, current, context)
	case statementIf:
		state.validateCondition(item.Condition, current, context)
		state.validateStatement(item.Then, current, context)
		state.validateStatement(item.Else, current, context)
	case statementWhile:
		state.validateCondition(item.Condition, current, context)
		context.loops++
		state.validateStatement(item.Then, current, context)
	case statementFor:
		loopScope := &scope{parent: current, symbols: make(map[string]symbol)}
		state.validateStatement(item.Initializer, loopScope, context)
		state.validateCondition(item.Condition, loopScope, context)
		state.inferExpression(item.Post, loopScope, context)
		context.loops++
		state.validateStatement(item.Then, loopScope, context)
	case statementForeach:
		loopScope := &scope{parent: current, symbols: make(map[string]symbol)}
		for _, iterator := range item.Iterators {
			if _, exists := loopScope.symbols[iterator.Name]; exists {
				state.add("DZI3017", iterator.Span, fmt.Sprintf("duplicate foreach iterator %q", iterator.Name), "use distinct iterator names")
			}
			loopScope.symbols[iterator.Name] = symbol{typeInfo: typeFromRef(iterator.Type), offset: iterator.Span.Start.Offset}
		}
		state.inferExpression(item.Expression, current, context)
		context.loops++
		state.validateStatement(item.Then, loopScope, context)
	case statementSwitch:
		state.inferExpression(item.Expression, current, context)
		state.validateSwitch(item, current, context)
	case statementReturn:
		state.validateReturn(item, current, context)
	case statementBreak:
		if context.loops == 0 && context.switches == 0 {
			state.add("DZI3018", item.Span, "break is only valid inside a loop or switch", "move break into a loop or switch")
		}
	case statementContinue:
		if context.loops == 0 {
			state.add("DZI3019", item.Span, "continue is only valid inside a loop", "move continue into a loop")
		}
	case statementEmpty:
		return
	}
}

func (state *semanticState) validateStatementWithFuture(item *statement, current *scope, context flowContext, future map[string]int) {
	context.future = future
	state.validateStatement(item, current, context)
	if item != nil && item.Kind == statementDeclaration {
		for _, variable := range item.Declaration.Variables {
			delete(future, variable.Name)
		}
	}
}

func (state *semanticState) declareVariables(current *scope, declaration *declaration, reportDuplicate bool) {
	base := typeFromRef(declaration.Type)
	for _, variable := range declaration.Variables {
		typeInfo := base
		for count := 0; count < variable.ArrayDepth; count++ {
			copy := typeInfo
			typeInfo = valueType{kind: valueArray, elem: &copy}
		}
		if _, exists := current.symbols[variable.Name]; exists && reportDuplicate {
			state.add("DZI3020", variable.Span, fmt.Sprintf("duplicate variable %q in the same scope", variable.Name), "rename the variable or use a nested scope")
			continue
		}
		current.symbols[variable.Name] = symbol{typeInfo: typeInfo, offset: variable.Span.Start.Offset}
	}
}

func collectBlockDeclarations(block *statement) map[string]int {
	result := make(map[string]int)
	for _, item := range block.Statements {
		if item != nil && item.Kind == statementDeclaration {
			for _, variable := range item.Declaration.Variables {
				if _, exists := result[variable.Name]; !exists {
					result[variable.Name] = variable.Span.Start.Offset
				}
			}
		}
	}
	return result
}

func (state *semanticState) validateCondition(condition *expression, current *scope, context flowContext) {
	if condition == nil {
		return
	}
	typeInfo := state.inferExpression(condition, current, context)
	if typeInfo.kind != valueUnknown && typeInfo.kind != valueBool && typeInfo.kind != valueObject && typeInfo.kind != valueNull {
		state.add("DZI3021", condition.Span, "condition is not Boolean or an object reference", "use a Boolean expression or a valid reference check")
	}
}

func (state *semanticState) validateReturn(item *statement, current *scope, context flowContext) {
	if context.function == nil {
		state.add("DZI3022", item.Span, "return is only valid inside a function or method", "move return into a function")
		return
	}
	expected := typeFromRef(context.function.Type)
	if item.Expression == nil {
		if expected.kind != valueVoid {
			state.add("DZI3023", item.Span, fmt.Sprintf("non-void function %s returns no value", context.function.Name), "return a compatible value")
		}
		return
	}
	actual := state.inferExpression(item.Expression, current, context)
	if expected.kind == valueVoid {
		state.add("DZI3024", item.Span, fmt.Sprintf("void function %s cannot return a value", context.function.Name), "remove the return value")
		return
	}
	state.requireCompatible(expected, actual, item.Expression.Span, fmt.Sprintf("return value of %s", context.function.Name))
}

func (state *semanticState) validateSwitch(item *statement, current *scope, context flowContext) {
	seen := make(map[string]bool)
	defaultSeen := false
	context.switches++
	for _, itemCase := range item.Cases {
		if itemCase.Default {
			if defaultSeen {
				state.add("DZI3025", itemCase.Span, "switch contains more than one default label", "remove the duplicate default label")
			}
			defaultSeen = true
		} else {
			state.inferExpression(itemCase.Expression, current, context)
			if key, ok := constantKey(itemCase.Expression); ok {
				if seen[key] {
					state.add("DZI3026", itemCase.Span, "switch contains a duplicate constant case value", "remove or change the duplicate case")
				}
				seen[key] = true
			}
		}
		caseScope := &scope{parent: current, symbols: make(map[string]symbol)}
		for _, statement := range itemCase.Statements {
			state.validateStatement(statement, caseScope, context)
		}
	}
}

func (state *semanticState) inferExpression(item *expression, current *scope, context flowContext) valueType {
	if item == nil {
		return valueType{kind: valueUnknown}
	}
	switch item.Kind {
	case expressionIdentifier:
		switch item.Text {
		case "true", "false":
			return valueType{kind: valueBool}
		case "null", "NULL":
			return valueType{kind: valueNull}
		case "this":
			if context.class == nil {
				state.add("DZI3027", item.Span, "this is only valid inside a class method", "remove this or move the code into a method")
				return valueType{kind: valueUnknown}
			}
			return valueType{kind: valueObject, name: context.class.Name}
		case "super":
			if context.class == nil || context.class.Base == nil {
				state.add("DZI3028", item.Span, "super requires a method in a class with a base class", "use super only in a derived class method")
				return valueType{kind: valueUnknown}
			}
			return typeFromRef(*context.class.Base)
		}
		if symbol, ok := lookupSymbol(current, item.Text); ok {
			return symbol.typeInfo
		}
		if offset, declaredLater := context.future[item.Text]; declaredLater && offset > item.Span.Start.Offset {
			state.add("DZI3029", item.Span, fmt.Sprintf("variable %q is used before its declaration", item.Text), "move the declaration before this use")
		}
		return valueType{kind: valueUnknown}
	case expressionLiteral:
		if strings.HasPrefix(item.Text, "\"") {
			return valueType{kind: valueString}
		}
		if strings.ContainsAny(item.Text, ".eEfF") {
			return valueType{kind: valueFloat}
		}
		return valueType{kind: valueInt}
	case expressionNew:
		for _, argument := range item.Args {
			state.inferExpression(argument, current, context)
		}
		return typeFromRef(item.Type)
	case expressionArray:
		return state.inferArray(item, current, context)
	case expressionCast:
		state.inferExpression(item.Right, current, context)
		return typeFromRef(item.Type)
	case expressionMember:
		state.inferExpression(item.Receiver, current, context)
		return valueType{kind: valueUnknown}
	case expressionCall:
		return state.inferCall(item, current, context)
	case expressionIndex:
		return state.inferIndex(item, current, context)
	case expressionUnary:
		return state.inferUnary(item, current, context)
	case expressionBinary:
		return state.inferBinary(item, current, context)
	case expressionTernary:
		state.validateCondition(item.Left, current, context)
		left := state.inferExpression(item.Right, current, context)
		right := state.inferExpression(item.Third, current, context)
		if compatible(left, right, state.classes) {
			return left
		}
		if compatible(right, left, state.classes) {
			return right
		}
		state.add("DZI3030", item.Span, "conditional expression branches have incompatible known types", "make both branches produce compatible values")
		return valueType{kind: valueUnknown}
	}
	return valueType{kind: valueUnknown}
}

func (state *semanticState) inferArray(item *expression, current *scope, context flowContext) valueType {
	element := valueType{kind: valueUnknown}
	for _, argument := range item.Args {
		actual := state.inferExpression(argument, current, context)
		if element.kind == valueUnknown {
			element = actual
		} else if !compatible(element, actual, state.classes) && !compatible(actual, element, state.classes) {
			state.add("DZI3031", argument.Span, "array literal contains incompatible known element types", "use compatible values in the array literal")
			element = valueType{kind: valueUnknown}
		}
	}
	return valueType{kind: valueArray, elem: &element}
}

func (state *semanticState) inferCall(item *expression, current *scope, context flowContext) valueType {
	for _, argument := range item.Args {
		state.inferExpression(argument, current, context)
	}
	if item.Receiver == nil || item.Receiver.Kind != expressionIdentifier {
		state.inferExpression(item.Receiver, current, context)
		return valueType{kind: valueUnknown}
	}
	name := item.Receiver.Text
	candidates := state.functions[name]
	if context.class != nil {
		for _, member := range context.class.Members {
			if member.Kind == declarationFunction && member.Name == name {
				candidates = append(candidates, member)
			}
		}
	}
	if len(candidates) != 1 {
		return valueType{kind: valueUnknown}
	}
	function := candidates[0]
	required := 0
	for _, parameter := range function.Parameters {
		if parameter.Default == nil {
			required++
		}
	}
	if len(item.Args) < required || len(item.Args) > len(function.Parameters) {
		state.add("DZI3032", item.Span, fmt.Sprintf("call to %s has %d arguments; expected %d to %d", name, len(item.Args), required, len(function.Parameters)), "adjust the call arguments")
		return typeFromRef(function.Type)
	}
	for index, argument := range item.Args {
		parameter := function.Parameters[index]
		actual := state.inferExpression(argument, current, context)
		state.requireCompatible(typeFromRef(parameter.Type), actual, argument.Span, fmt.Sprintf("argument %d to %s", index+1, name))
		if (hasModifier(parameter.Modifiers, "out") || hasModifier(parameter.Modifiers, "inout")) && !isLValue(argument) {
			state.add("DZI3033", argument.Span, fmt.Sprintf("argument %d to %s must be assignable for %s", index+1, name, firstIOParameterModifier(parameter.Modifiers)), "pass a variable, field, or indexed location")
		}
	}
	return typeFromRef(function.Type)
}

func (state *semanticState) inferIndex(item *expression, current *scope, context flowContext) valueType {
	receiver := state.inferExpression(item.Receiver, current, context)
	index := state.inferExpression(item.Right, current, context)
	if index.kind != valueUnknown && index.kind != valueInt {
		state.add("DZI3034", item.Right.Span, "index expression must be an integer", "use an integer index")
	}
	if receiver.kind == valueArray && receiver.elem != nil {
		return *receiver.elem
	}
	if receiver.kind == valueString {
		return valueType{kind: valueString}
	}
	if receiver.kind != valueUnknown {
		state.add("DZI3035", item.Receiver.Span, "known non-indexable value is indexed", "index an array or string")
	}
	return valueType{kind: valueUnknown}
}

func (state *semanticState) inferUnary(item *expression, current *scope, context flowContext) valueType {
	operandExpression := item.Right
	if strings.HasPrefix(item.Text, "post") {
		operandExpression = item.Left
	}
	operand := state.inferExpression(operandExpression, current, context)
	if item.Text == "!" {
		if operand.kind != valueUnknown && operand.kind != valueBool && operand.kind != valueObject && operand.kind != valueNull {
			state.add("DZI3036", item.Span, "logical not requires a Boolean or object reference", "apply ! to a Boolean or reference expression")
		}
		return valueType{kind: valueBool}
	}
	if item.Text == "~" && operand.kind != valueUnknown && operand.kind != valueInt {
		state.add("DZI3037", item.Span, "bitwise not requires an integer", "apply ~ to an integer expression")
	}
	if item.Text == "++" || item.Text == "--" || strings.HasPrefix(item.Text, "post") {
		if !isLValue(operandExpression) {
			state.add("DZI3038", item.Span, "increment or decrement target is not assignable", "apply the operator to a variable, field, or indexed location")
		}
	}
	if operand.kind != valueUnknown && operand.kind != valueInt && operand.kind != valueFloat {
		state.add("DZI3039", item.Span, fmt.Sprintf("operator %s requires a numeric operand", item.Text), "use an int or float operand")
	}
	return operand
}

func (state *semanticState) inferBinary(item *expression, current *scope, context flowContext) valueType {
	left := state.inferExpression(item.Left, current, context)
	right := state.inferExpression(item.Right, current, context)
	operator := item.Text
	if binaryPrecedence[operator] == 1 {
		if !isLValue(item.Left) {
			state.add("DZI3040", item.Left.Span, "assignment target is not assignable", "assign to a variable, field, or indexed location")
		}
		state.requireCompatible(left, right, item.Right.Span, "assignment")
		return left
	}
	if (operator == "/" || operator == "%") && constantZero(item.Right) {
		state.add("DZI3041", item.Right.Span, fmt.Sprintf("constant %s by zero", map[bool]string{true: "division", false: "modulo"}[operator == "/"]), "use a non-zero divisor")
	}
	switch operator {
	case "&&", "||":
		state.requireLogicalOperand(left, item.Left.Span)
		state.requireLogicalOperand(right, item.Right.Span)
		return valueType{kind: valueBool}
	case "==", "!=":
		if !compatible(left, right, state.classes) && !compatible(right, left, state.classes) {
			state.add("DZI3042", item.Span, "equality compares incompatible known types", "compare compatible values")
		}
		return valueType{kind: valueBool}
	case "<", "<=", ">", ">=":
		state.requireNumericPair(left, right, item.Span, operator)
		return valueType{kind: valueBool}
	case "&", "|", "^", "<<", ">>":
		if (left.kind != valueUnknown && left.kind != valueInt) || (right.kind != valueUnknown && right.kind != valueInt) {
			state.add("DZI3043", item.Span, fmt.Sprintf("operator %s requires integer operands", operator), "use integer operands")
		}
		return valueType{kind: valueInt}
	case "+":
		if left.kind == valueString && (right.kind == valueString || right.kind == valueUnknown) {
			return valueType{kind: valueString}
		}
		fallthrough
	case "-", "*", "/", "%":
		state.requireNumericPair(left, right, item.Span, operator)
		if left.kind == valueFloat || right.kind == valueFloat {
			return valueType{kind: valueFloat}
		}
		if left.kind == valueUnknown || right.kind == valueUnknown {
			return valueType{kind: valueUnknown}
		}
		return valueType{kind: valueInt}
	}
	return valueType{kind: valueUnknown}
}

func (state *semanticState) requireLogicalOperand(actual valueType, span Span) {
	if actual.kind != valueUnknown && actual.kind != valueBool && actual.kind != valueObject && actual.kind != valueNull {
		state.add("DZI3044", span, "logical operator requires Boolean or object-reference operands", "use Boolean expressions or reference checks")
	}
}

func (state *semanticState) requireNumericPair(left, right valueType, span Span, operator string) {
	if (left.kind != valueUnknown && left.kind != valueInt && left.kind != valueFloat) || (right.kind != valueUnknown && right.kind != valueInt && right.kind != valueFloat) {
		state.add("DZI3045", span, fmt.Sprintf("operator %s requires numeric operands", operator), "use int or float operands")
	}
}

func (state *semanticState) requireCompatible(expected, actual valueType, span Span, subject string) {
	if !compatible(expected, actual, state.classes) {
		state.add("DZI3046", span, fmt.Sprintf("%s has incompatible known type %s; expected %s", subject, typeName(actual), typeName(expected)), "use a compatible value")
	}
}

func (state *semanticState) validateInheritance() {
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		if visiting[name] {
			class := state.classes[name]
			state.add("DZI3047", class.Span, fmt.Sprintf("inheritance cycle includes class %s", name), "remove a base-class link in the cycle")
			return
		}
		visiting[name] = true
		class := state.classes[name]
		if class != nil && class.Base != nil && state.classes[class.Base.Name] != nil {
			visit(class.Base.Name)
		}
		visiting[name] = false
		visited[name] = true
	}
	for name := range state.classes {
		visit(name)
	}
}

func (state *semanticState) add(code string, span Span, message, hint string) {
	state.found.add(Diagnostic{Code: code, Span: span, Message: message, Hint: hint})
}

func typeFromRef(reference typeRef) valueType {
	var result valueType
	switch reference.Name {
	case "void":
		result.kind = valueVoid
	case "bool":
		result.kind = valueBool
	case "int":
		result.kind = valueInt
	case "float":
		result.kind = valueFloat
	case "string":
		result.kind = valueString
	case "auto", "var", "typename", "func", "":
		result.kind = valueUnknown
	case "array":
		result.kind = valueArray
		if len(reference.Arguments) == 1 {
			element := typeFromRef(reference.Arguments[0])
			result.elem = &element
		}
	default:
		result.kind = valueObject
		result.name = reference.Name
	}
	for count := 0; count < reference.ArrayDepth; count++ {
		copy := result
		result = valueType{kind: valueArray, elem: &copy}
	}
	return result
}

func compatible(expected, actual valueType, classes map[string]*declaration) bool {
	if expected.kind == valueUnknown || actual.kind == valueUnknown {
		return true
	}
	if expected.kind == valueFloat && actual.kind == valueInt {
		return true
	}
	if expected.kind == valueObject && actual.kind == valueNull {
		return true
	}
	if expected.kind != actual.kind {
		return false
	}
	if expected.kind == valueArray {
		if expected.elem == nil || actual.elem == nil {
			return true
		}
		return compatible(*expected.elem, *actual.elem, classes)
	}
	if expected.kind != valueObject || expected.name == actual.name {
		return true
	}
	return locallyDerives(actual.name, expected.name, classes)
}

func locallyDerives(actual, expected string, classes map[string]*declaration) bool {
	if actual == "MissionServer" && expected == "Mission" {
		return true
	}
	seen := make(map[string]bool)
	for actual != "" && !seen[actual] {
		seen[actual] = true
		class := classes[actual]
		if class == nil || class.Base == nil {
			return false
		}
		actual = class.Base.Name
		if actual == expected || (actual == "MissionServer" && expected == "Mission") {
			return true
		}
	}
	return false
}

func lookupSymbol(current *scope, name string) (symbol, bool) {
	for current != nil {
		if found, ok := current.symbols[name]; ok {
			return found, true
		}
		current = current.parent
	}
	return symbol{}, false
}

func functionSignature(function *declaration) string {
	result := function.Name + "("
	for index, parameter := range function.Parameters {
		if index > 0 {
			result += ","
		}
		result += parameter.Type.String()
	}
	return result + ")"
}

func hasModifier(modifiers []string, target string) bool {
	for _, modifier := range modifiers {
		if modifier == target {
			return true
		}
	}
	return false
}

func firstIOParameterModifier(modifiers []string) string {
	if hasModifier(modifiers, "out") {
		return "out"
	}
	return "inout"
}

func isLValue(item *expression) bool {
	return item != nil && (item.Kind == expressionIdentifier || item.Kind == expressionMember || item.Kind == expressionIndex)
}

func constantZero(item *expression) bool {
	if item == nil {
		return false
	}
	if item.Kind == expressionUnary && (item.Text == "+" || item.Text == "-") {
		return constantZero(item.Right)
	}
	if item.Kind != expressionLiteral || strings.HasPrefix(item.Text, "\"") {
		return false
	}
	text := strings.TrimSuffix(strings.TrimSuffix(item.Text, "f"), "F")
	value, err := strconv.ParseFloat(text, 64)
	return err == nil && value == 0
}

func constantKey(item *expression) (string, bool) {
	if item == nil {
		return "", false
	}
	if item.Kind == expressionLiteral {
		return item.Text, true
	}
	if item.Kind == expressionIdentifier && (item.Text == "true" || item.Text == "false") {
		return item.Text, true
	}
	if item.Kind == expressionUnary && (item.Text == "+" || item.Text == "-") && item.Right != nil && item.Right.Kind == expressionLiteral {
		return item.Text + item.Right.Text, true
	}
	return "", false
}

func statementDefinitelyReturns(item *statement) bool {
	if item == nil {
		return false
	}
	switch item.Kind {
	case statementReturn:
		return true
	case statementBlock:
		for _, child := range item.Statements {
			if statementDefinitelyReturns(child) {
				return true
			}
		}
	case statementIf:
		return item.Else != nil && statementDefinitelyReturns(item.Then) && statementDefinitelyReturns(item.Else)
	case statementSwitch:
		hasDefault := false
		if len(item.Cases) == 0 {
			return false
		}
		allReturn := true
		nextReturns := false
		for index := len(item.Cases) - 1; index >= 0; index-- {
			itemCase := item.Cases[index]
			hasDefault = hasDefault || itemCase.Default
			caseReturns := false
			caseBreaks := false
			for _, child := range itemCase.Statements {
				if statementDefinitelyReturns(child) {
					caseReturns = true
					break
				}
				if child != nil && child.Kind == statementBreak {
					caseBreaks = true
					break
				}
			}
			if !caseReturns && !caseBreaks {
				caseReturns = nextReturns
			}
			if !caseReturns {
				allReturn = false
			}
			nextReturns = caseReturns
		}
		return hasDefault && allReturn
	}
	return false
}

func typeName(info valueType) string {
	switch info.kind {
	case valueUnknown:
		return "unknown"
	case valueVoid:
		return "void"
	case valueBool:
		return "bool"
	case valueInt:
		return "int"
	case valueFloat:
		return "float"
	case valueString:
		return "string"
	case valueNull:
		return "null"
	case valueObject:
		return info.name
	case valueArray:
		if info.elem == nil {
			return "array"
		}
		return "array<" + typeName(*info.elem) + ">"
	default:
		return "unknown"
	}
}
