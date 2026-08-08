package dayzinit

import (
	"strings"
	"testing"
)

func TestSemanticDuplicateDeclarations(t *testing.T) {
	text := `class A {}; class A {};
enum E { ONE, ONE };
int globalValue; int globalValue;
typedef int Alias; typedef int Alias;
void Same(int value) {} void Same(int other) {}
class Members
{
    int field; int field;
    void Method() {} void Method() {}
};`
	found := semanticText(t, text)
	assertCodes(t, found, "DZI3001", "DZI3002", "DZI3003", "DZI3008", "DZI3009", "DZI3010")
}

func TestSemanticModifiersAndParameters(t *testing.T) {
	text := `modded int invalidModded;
override void TopOverride() {}
proto void ProtoBody() {}
static static int duplicateModifier;
void Parameters(out out inout int duplicate, int optional = 1, int required, int required) {}`
	found := semanticText(t, text)
	assertCodes(t, found, "DZI3005", "DZI3006", "DZI3007", "DZI3011", "DZI3012", "DZI3014", "DZI3015", "DZI3016")
}

func TestSemanticScopesControlAndReturns(t *testing.T) {
	text := `int MissingReturn(bool condition) { if (condition) return 1; }
int EmptyReturn() { return; }
void ValuedReturn() { return 1; }
void ScopeAndFlow()
{
    int before = later;
    int later = 1;
    int later = 2;
    if (1) { ; }
    break;
    continue;
    array<string> values;
    foreach (int same, string same: values) { continue; }
    while (true) { break; }
}`
	found := semanticText(t, text)
	assertCodes(t, found, "DZI3013", "DZI3017", "DZI3018", "DZI3019", "DZI3020", "DZI3021", "DZI3023", "DZI3024", "DZI3029")

	state := semanticState{source: newSourceFile("init.c", []byte("return;"))}
	state.validateStatement(&statement{Kind: statementReturn, Span: state.source.span(0, 7)}, &scope{symbols: map[string]symbol{}}, flowContext{})
	if !diagnosticsContain(state.found.items, "DZI3022") {
		t.Fatalf("return outside function diagnostics = %#v", state.found.items)
	}
}

func TestSemanticThisSuperSwitchAndInheritance(t *testing.T) {
	text := `class CycleA: CycleB {}; class CycleB: CycleA {};
void Context()
{
    this;
    super;
    switch (1)
    {
        case 1: break;
        case 1: break;
        default: break;
        default: break;
    }
}`
	found := semanticText(t, text)
	assertCodes(t, found, "DZI3025", "DZI3026", "DZI3027", "DZI3028", "DZI3047")
}

func TestSemanticExpressionsAndCalls(t *testing.T) {
	text := `void Target(out int output, int value = 2) { output = value; }
void Expressions()
{
    Target(1, "bad", 3);
    Target(1, "bad");
    1 = 2;
    int divide = 1 / -0;
    int modulo = 1 % 0;
    bool equality = 1 == "one";
    int bits = 1 & 1.5;
    bool logical = 1 && true;
    int arithmetic = "one" - 1;
    array<int> mixed = { 1, "two" };
    string text = "abc";
    string character = text[1.5];
    int scalar = 1;
    int indexed = scalar[0];
    bool negated = !1;
    int inverted = ~1.5;
    ++1;
    -"one";
    int choice = true ? 1 : "two";
}`
	found := semanticText(t, text)
	assertCodes(t, found,
		"DZI3030", "DZI3031", "DZI3032", "DZI3033", "DZI3034", "DZI3035", "DZI3036", "DZI3037", "DZI3038", "DZI3039",
		"DZI3040", "DZI3041", "DZI3042", "DZI3043", "DZI3044", "DZI3045", "DZI3046")
}

func TestSemanticKnownCallsDefaultsAndTruthiness(t *testing.T) {
	text := `class Object {};
int Add(int left, int right = 1) { return left + right; }
void Valid()
{
    int value = Add(1);
    Object object = new Object(1);
    if (object) value++;
    string text = "abc";
    string first = text[0];
    array<int> values = { 1, 2 };
    int second = values[1];
    bool same = null == object;
    float widened = value;
    ExternalType external = ExternalCall();
    external.UnknownMethod(value);
}`
	found := semanticText(t, text)
	if len(found) != 0 {
		t.Fatalf("valid semantic diagnostics: %#v", found)
	}
}

func TestSemanticRemainingExpressionPaths(t *testing.T) {
	text := `class Parent {};
class Child: Parent
{
    void Context() { this; super; }
};
void More()
{
    string joined = "a" + "b";
    float floating = 1.0 + 2;
    External unknown = UnknownCall();
    auto unknownResult = UnknownCall() + 1;
    float reversed = true ? 1 : 2.0;
}`
	found := semanticText(t, text)
	if len(found) != 0 {
		t.Fatalf("remaining valid paths: %#v", found)
	}

	state := semanticState{source: newSourceFile("init.c", nil), classes: map[string]*declaration{}}
	current := &scope{symbols: map[string]symbol{}}
	state.validateCondition(nil, current, flowContext{})
	if got := state.inferExpression(&expression{Kind: expressionKind(255)}, current, flowContext{}); got.kind != valueUnknown {
		t.Fatalf("unknown expression kind = %#v", got)
	}
	left := &expression{Kind: expressionLiteral, Text: "1"}
	right := &expression{Kind: expressionLiteral, Text: "2"}
	if got := state.inferBinary(&expression{Kind: expressionBinary, Text: "???", Left: left, Right: right}, current, flowContext{}); got.kind != valueUnknown {
		t.Fatalf("unknown binary = %#v", got)
	}
}

func TestSemanticHelpers(t *testing.T) {
	classes := map[string]*declaration{
		"Child": {Name: "Child", Base: &typeRef{Name: "MissionServer"}},
	}
	if !compatible(valueType{kind: valueFloat}, valueType{kind: valueInt}, classes) ||
		!compatible(valueType{kind: valueObject, name: "Mission"}, valueType{kind: valueObject, name: "Child"}, classes) ||
		compatible(valueType{kind: valueInt}, valueType{kind: valueString}, classes) {
		t.Fatal("compatible")
	}
	if locallyDerives("Missing", "Mission", classes) || locallyDerives("Child", "Other", classes) {
		t.Fatal("locallyDerives")
	}
	if !locallyDerives("MissionServer", "Mission", classes) {
		t.Fatal("known MissionServer relationship")
	}
	cycle := map[string]*declaration{
		"A": {Name: "A", Base: &typeRef{Name: "B"}},
		"B": {Name: "B", Base: &typeRef{Name: "A"}},
	}
	if locallyDerives("A", "Never", cycle) {
		t.Fatal("cycle unexpectedly derived")
	}
	if constantZero(nil) {
		t.Fatal("nil is constant zero")
	}
	if !constantZero(&expression{Kind: expressionUnary, Text: "+", Right: &expression{Kind: expressionLiteral, Text: "0F"}}) || constantZero(&expression{Kind: expressionLiteral, Text: `"0"`}) {
		t.Fatal("constantZero")
	}
	if key, ok := constantKey(&expression{Kind: expressionUnary, Text: "-", Right: &expression{Kind: expressionLiteral, Text: "1"}}); !ok || key != "-1" {
		t.Fatalf("constantKey = %q, %v", key, ok)
	}
	if _, ok := constantKey(&expression{Kind: expressionMember}); ok {
		t.Fatal("nonconstant expression returned constant key")
	}
	if _, ok := constantKey(nil); ok {
		t.Fatal("nil returned constant key")
	}
	if key, ok := constantKey(&expression{Kind: expressionIdentifier, Text: "true"}); !ok || key != "true" {
		t.Fatal("Boolean constant key")
	}
	if typeName(valueType{kind: valueKind(255)}) != "unknown" || typeName(valueType{kind: valueArray}) != "array" {
		t.Fatal("typeName fallback")
	}
	if firstIOParameterModifier([]string{"inout"}) != "inout" || !hasModifier([]string{"out"}, "out") {
		t.Fatal("modifier helpers")
	}
	for _, reference := range []typeRef{{Name: "auto"}, {Name: "array"}, {Name: "int", ArrayDepth: 1}} {
		_ = typeFromRef(reference)
	}
	if !compatible(valueType{kind: valueArray}, valueType{kind: valueArray, elem: &valueType{kind: valueInt}}, classes) {
		t.Fatal("unspecified array element compatibility")
	}
	for _, kind := range []valueKind{valueUnknown, valueVoid, valueBool, valueInt, valueFloat, valueString, valueNull, valueObject, valueArray, valueKind(255)} {
		info := valueType{kind: kind, name: "Object"}
		if kind == valueArray {
			info.elem = &valueType{kind: valueInt}
		}
		if typeName(info) == "" {
			t.Fatalf("blank type name for %d", kind)
		}
	}
	if statementDefinitelyReturns(nil) || statementDefinitelyReturns(&statement{Kind: statementSwitch}) {
		t.Fatal("empty statement return analysis")
	}
	if statementDefinitelyReturns(&statement{Kind: statementSwitch, Cases: []switchCase{{Default: true, Statements: []*statement{{Kind: statementBreak}}}}}) {
		t.Fatal("breaking switch reported as returning")
	}
	if statementDefinitelyReturns(&statement{Kind: statementEmpty}) {
		t.Fatal("empty statement reported as returning")
	}
}

func semanticText(t *testing.T, text string) []Diagnostic {
	t.Helper()
	program, found := parseText(t, text)
	if len(found) != 0 {
		t.Fatalf("parse diagnostics: %#v", found)
	}
	return validateSemantics(newSourceFile("init.c", []byte(text)), program)
}

func assertCodes(t *testing.T, items []Diagnostic, expected ...string) {
	t.Helper()
	codes := diagnosticCodes(items)
	for _, code := range expected {
		if !strings.Contains(codes, code) {
			t.Fatalf("codes %s do not contain %s; diagnostics=%#v", codes, code, items)
		}
	}
}
