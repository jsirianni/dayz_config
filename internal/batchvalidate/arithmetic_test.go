package batchvalidate

import (
	"strings"
	"testing"
)

func TestSetArithmeticDocumentedOperators(t *testing.T) {
	expressions := []string{
		"A=1",
		"A=(1+2)*3",
		"A=!B",
		"A=~B",
		"A=-B",
		"A=+B",
		"A=B/C%D",
		"A=B+C-D",
		"A=B<<2",
		"A=B>>2",
		"A=B&C",
		"A=B^C",
		"A=B|C",
		"A=B&&C",
		"A=B||C",
		"A*=2",
		"A/=2",
		"A%=2",
		"A+=2",
		"A-=2",
		"A&=2",
		"A^=2",
		"A|=2",
		"A<<=2",
		"A>>=2",
		"A=1,B=2",
		"A=B=C",
	}
	for _, expression := range expressions {
		src := []byte("set /a \"" + expression + "\"")
		if result := ValidateSource("operators.cmd", src, Options{}); result.HasErrors() {
			t.Errorf("%q diagnostics = %#v", expression, result.Diagnostics)
		}
	}
}

func TestSetArithmeticNumbersAndErrors(t *testing.T) {
	for _, number := range []string{"0", "42", "0x12", "0XfF", "022"} {
		result := ValidateSource("numbers.cmd", []byte("set /a A="+number), Options{})
		if result.HasErrors() {
			t.Errorf("number %q diagnostics = %#v", number, result.Diagnostics)
		}
	}
	for _, number := range []string{"0x", "0xG", "08", "1z"} {
		result := ValidateSource("numbers.cmd", []byte("set /a A="+number), Options{})
		if !hasCode(result, "BAT4004") {
			t.Errorf("number %q diagnostics = %#v", number, result.Diagnostics)
		}
	}
	invalid := []string{
		"set /a \"*1\"",
		"set /a \"A=1 2\"",
		"set /a \"A=(1+2\"",
		"set /a \"A=()\"",
	}
	for _, src := range invalid {
		if result := ValidateSource("invalid-expression.cmd", []byte(src), Options{}); !hasCode(result, "BAT4003") {
			t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
		}
	}
}

func TestSetArithmeticTruncatedQuoteRegression(t *testing.T) {
	result := ValidateSource("fuzz-regression.cmd", []byte("set /A \""), Options{ReportUnsupported: true})
	if result.HasErrors() || result.FullyValidated || !hasCode(result, "BAT9002") {
		t.Fatalf("truncated quote result = %#v", result)
	}
}

func TestArithmeticDynamicExpansionAndResourceLimit(t *testing.T) {
	empty := ValidateSource("empty-expression.cmd", []byte("set /a \"\""), Options{})
	if !hasCode(empty, "BAT4002") {
		t.Fatalf("quoted empty expression result = %#v", empty)
	}
	result := ValidateSource("dynamic-expression.cmd", []byte("set /a \"A=%VALUE%+1\""), Options{ReportUnsupported: true})
	if result.HasErrors() || result.FullyValidated || !hasCode(result, "BAT9002") {
		t.Fatalf("dynamic expression result = %#v", result)
	}
	expression := strings.Repeat("(", maxParserDepth+1) + "A" + strings.Repeat(")", maxParserDepth+1)
	result = ValidateSource("deep-expression.cmd", []byte("set /a \""+expression+"\""), Options{ReportUnsupported: true})
	if result.HasErrors() || !hasCode(result, "BAT9001") {
		t.Fatalf("deep expression result = %#v", result)
	}
	unary := strings.Repeat("!", maxParserDepth+1) + "A"
	result = ValidateSource("deep-unary.cmd", []byte("set /a \""+unary+"\""), Options{ReportUnsupported: true})
	if result.HasErrors() || !hasCode(result, "BAT9001") {
		t.Fatalf("deep unary result = %#v", result)
	}
}

func TestArithmeticHelpers(t *testing.T) {
	for _, number := range []string{"0", "1", "077", "0xA"} {
		if !validArithmeticNumber(number) {
			t.Errorf("validArithmeticNumber(%q) = false", number)
		}
	}
	for _, number := range []string{"", "09", "0x", "0xz", "12a"} {
		if validArithmeticNumber(number) {
			t.Errorf("validArithmeticNumber(%q) = true", number)
		}
	}
	operators := []string{"<<=", ">>=", "&&", "||", "*=", "/=", "%=", "+=", "-=", "&=", "^=", "|=", "<<", ">>", "!", "~", "-", "+", "*", "/", "%", "&", "^", "|", "=", ","}
	for _, operator := range operators {
		if got := arithmeticOperatorAt([]byte(operator + "tail")); got != operator {
			t.Errorf("arithmeticOperatorAt(%q) = %q", operator, got)
		}
		precedence, _, _ := arithmeticPrecedence(operator)
		if operator != "!" && operator != "~" && precedence == 0 {
			t.Errorf("operator %q has no infix precedence", operator)
		}
	}
	if got := arithmeticOperatorAt([]byte("name")); got != "" {
		t.Fatalf("ordinary identifier yielded operator %q", got)
	}
	if precedence, right, assignment := arithmeticPrecedence("?"); precedence != 0 || right || assignment {
		t.Fatalf("unknown precedence = %d,%v,%v", precedence, right, assignment)
	}
}
