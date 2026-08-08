package dayzinit

import "testing"

func TestMissionContractDiagnostics(t *testing.T) {
	program, found := parseText(t, "class Nothing {};")
	if len(found) != 0 {
		t.Fatalf("parse diagnostics: %#v", found)
	}
	contract := validateMissionContract(newSourceFile("init.c", []byte("class Nothing {};")), program)
	assertCodes(t, contract, "DZI4001", "DZI4004")

	text := `int main(int argument); void main() {}
int CreateCustomMission(); Mission CreateCustomMission(string path) { return null; }`
	program, found = parseText(t, text)
	if len(found) != 0 {
		t.Fatalf("parse diagnostics: %#v", found)
	}
	contract = validateMissionContract(newSourceFile("init.c", []byte(text)), program)
	assertCodes(t, contract, "DZI4002", "DZI4003", "DZI4005", "DZI4006")

	badFactory := `void main() {}
class NotAMission {};
Mission CreateCustomMission(string path) { if (path == "") return new NotAMission(); else return new NotAMission(); }`
	program, found = parseText(t, badFactory)
	if len(found) != 0 {
		t.Fatalf("parse diagnostics: %#v", found)
	}
	contract = validateMissionContract(newSourceFile("init.c", []byte(badFactory)), program)
	assertCodes(t, contract, "DZI4007")
	if len(collectReturnExpressions(nil)) != 0 {
		t.Fatal("nil returns were collected")
	}
	caseReturn := &expression{Kind: expressionLiteral, Text: "1"}
	collected := collectReturnExpressions(&statement{Kind: statementSwitch, Cases: []switchCase{{Statements: []*statement{{Kind: statementReturn, Expression: caseReturn}}}}})
	if len(collected) != 1 || collected[0] != caseReturn {
		t.Fatalf("switch returns = %#v", collected)
	}
}

func TestMissionContractDoesNotRequireOptionalGameplay(t *testing.T) {
	text := `void main() {}
class ServerMission: MissionServer {};
Mission CreateCustomMission(string arbitraryName) { return new ServerMission; }`
	if err := ValidateSource("init.c", []byte(text)); err != nil {
		t.Fatalf("minimal mission: %v", err)
	}
}
