package dayzinit

import "fmt"

func validateMissionContract(source *sourceFile, program *program) []Diagnostic {
	var found diagnostics
	var mains []*declaration
	var factories []*declaration
	classes := make(map[string]*declaration)
	for _, declaration := range program.Declarations {
		switch {
		case declaration.Kind == declarationFunction && declaration.Name == "main":
			mains = append(mains, declaration)
		case declaration.Kind == declarationFunction && declaration.Name == "CreateCustomMission":
			factories = append(factories, declaration)
		case declaration.Kind == declarationClass:
			classes[declaration.Name] = declaration
		}
	}
	entrySpan := source.span(source.bomBytes, source.bomBytes)
	if len(mains) == 0 {
		found.add(Diagnostic{Code: "DZI4001", Message: "missing top-level DayZ main entry point", Hint: "add void main() with a function body", Span: entrySpan})
	}
	if len(mains) > 1 {
		for _, main := range mains[1:] {
			found.add(Diagnostic{Code: "DZI4002", Message: "duplicate top-level main entry point", Hint: "keep exactly one void main()", Span: main.Span})
		}
	}
	for _, main := range mains {
		if main.Type.Name != "void" || main.Type.ArrayDepth != 0 || len(main.Parameters) != 0 || !main.HasBody {
			found.add(Diagnostic{Code: "DZI4003", Message: "malformed DayZ main entry point", Hint: "declare exactly void main() with no parameters and a body", Span: main.Span})
		}
	}
	if len(factories) == 0 {
		found.add(Diagnostic{Code: "DZI4004", Message: "missing top-level CreateCustomMission factory", Hint: "add Mission CreateCustomMission(string path) with a function body", Span: entrySpan})
	}
	if len(factories) > 1 {
		for _, factory := range factories[1:] {
			found.add(Diagnostic{Code: "DZI4005", Message: "duplicate top-level CreateCustomMission factory", Hint: "keep exactly one mission factory", Span: factory.Span})
		}
	}
	for _, factory := range factories {
		validShape := factory.Type.Name == "Mission" && factory.Type.ArrayDepth == 0 && len(factory.Parameters) == 1 &&
			factory.Parameters[0].Type.Name == "string" && factory.Parameters[0].Type.ArrayDepth == 0 && factory.HasBody
		if !validShape {
			found.add(Diagnostic{Code: "DZI4006", Message: "malformed CreateCustomMission factory", Hint: "declare Mission CreateCustomMission(string path) with a body", Span: factory.Span})
			continue
		}
		for _, returned := range collectReturnExpressions(factory.Body) {
			if returned == nil || returned.Kind != expressionNew {
				continue
			}
			if class := classes[returned.Type.Name]; class != nil && !locallyDerives(class.Name, "Mission", classes) {
				found.add(Diagnostic{Code: "DZI4007", Message: fmt.Sprintf("factory returns %s, which does not derive from Mission or MissionServer", class.Name), Hint: "return an instance of a locally declared MissionServer subclass", Span: returned.Span})
			}
		}
	}
	return found.sorted()
}

func collectReturnExpressions(item *statement) []*expression {
	if item == nil {
		return nil
	}
	var result []*expression
	if item.Kind == statementReturn {
		result = append(result, item.Expression)
	}
	for _, child := range item.Statements {
		result = append(result, collectReturnExpressions(child)...)
	}
	result = append(result, collectReturnExpressions(item.Then)...)
	result = append(result, collectReturnExpressions(item.Else)...)
	for _, itemCase := range item.Cases {
		for _, child := range itemCase.Statements {
			result = append(result, collectReturnExpressions(child)...)
		}
	}
	return result
}
