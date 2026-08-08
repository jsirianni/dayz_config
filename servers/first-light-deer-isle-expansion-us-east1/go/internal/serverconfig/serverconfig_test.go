package serverconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseValidateAndListValidConfig(t *testing.T) {
	data := readFixture(t, "valid.cfg")

	document, err := ParseData(data)
	if err != nil {
		t.Fatalf("ParseData returned error: %v", err)
	}
	if len(document.Assignments) != len(documentedFields) {
		t.Fatalf("assignment count = %d, want %d", len(document.Assignments), len(documentedFields))
	}
	if document.Classes[0].Path != "Missions.DayZ" || document.Classes[1].Path != "Missions" {
		t.Fatalf("classes = %#v", document.Classes)
	}
	if err := ValidateData(data, "valid.cfg"); err != nil {
		t.Fatalf("ValidateData returned error: %v", err)
	}

	path := writeTempConfig(t, string(data))
	values, err := ListFieldsFile(path)
	if err != nil {
		t.Fatalf("ListFieldsFile returned error: %v", err)
	}
	values = SortFieldValues(values)
	assertEqual(t, values[0].Field, "hostname")
	assertEqual(t, values[0].Value, "Fixture Server")
	assertEqual(t, values[len(values)-1].Field, "clientPort")
	assertContains(t, strings.Join(FieldNames(), ","), "Missions.DayZ.template")
	assertEqual(t, ConfigFileName(filepath.Join("x", "serverDZ.cfg")), "serverDZ.cfg")
}

func TestValidateReportsDocumentedSchemaErrors(t *testing.T) {
	data := []byte(`unknownTop = 1;
hostname = 1;
timeStampFormat = "Medium";
motd = { "line" };
motd[] = { 1 };
disableBanlist = 0;
enableWhitelist = 2;
verifySignatures = 1;
maxPlayers = 1.5;
serverTimeAcceleration = 0;
serverNightTimeAcceleration = "fast";
networkObjectBatchBandwidthLimit = -0.1;
lightingConfig = 3;
class Other {};
class Missions { class OtherDayZ {}; };
`)

	err := ValidateData(data, "bad.cfg")
	if err == nil {
		t.Fatal("ValidateData err = nil")
	}
	message := err.Error()
	for _, want := range []string{
		`unknown field "unknownTop" requires annotation comment`,
		`hostname expected string`,
		`timeStampFormat must be one of Full, Short`,
		`motd must be written as motd[]`,
		`motd expected string array item`,
		`disableBanlist expected boolean`,
		`enableWhitelist must be 0 or 1`,
		`verifySignatures must be 2`,
		`maxPlayers expected integer, got "1.5"`,
		`serverTimeAcceleration must be between 0.1 and 64`,
		`serverNightTimeAcceleration expected number`,
		`networkObjectBatchBandwidthLimit must be greater than or equal to 0`,
		`lightingConfig must be one of 0, 1, 2`,
		`unknown class "Other"`,
		`unknown class "Missions.OtherDayZ"`,
	} {
		assertContains(t, message, want)
	}
}

func TestAnnotatedUnknownFieldsValidate(t *testing.T) {
	data := []byte(`// VPP Admin Tools option
vppDisablePassword = 1;
storeHouseStateDisabled = false; // DayZ server extension option
/* custom block annotation */
enableCfgGameplayFile = 1;
`)

	if err := ValidateData(data, "annotated.cfg"); err != nil {
		t.Fatalf("ValidateData returned error: %v", err)
	}

	document, err := ParseData(data)
	if err != nil {
		t.Fatalf("ParseData returned error: %v", err)
	}
	if !hasAnnotationComment(document.Data, document.Assignments[0]) {
		t.Fatal("preceding line annotation was not detected")
	}
	if !hasAnnotationComment(document.Data, document.Assignments[1]) {
		t.Fatal("trailing annotation was not detected")
	}
	if !hasAnnotationComment(document.Data, document.Assignments[2]) {
		t.Fatal("block annotation was not detected")
	}
	if _, end := previousNonBlankLine([]byte("\n\n"), 2); end != -1 {
		t.Fatal("previousNonBlankLine found a line in blank input")
	}
	start, end := previousNonBlankLine([]byte("comment\r\n\n"), 9)
	assertEqual(t, string([]byte("comment\r\n\n")[start:end]), "comment")
}

func TestSyntaxErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "invalid character", data: "@", want: "unexpected character"},
		{name: "unterminated block comment", data: "/* nope", want: "unterminated block comment"},
		{name: "unterminated string newline", data: "hostname = \"bad\n", want: "unterminated string"},
		{name: "unterminated string escape", data: "hostname = \"bad\\", want: "unterminated string escape"},
		{name: "unexpected brace", data: "};", want: "unexpected }"},
		{name: "class eof", data: "class Missions {", want: "expected } before end of file"},
		{name: "class name", data: "class = { };", want: "expected identifier"},
		{name: "class brace", data: "class Missions = {}; ", want: "expected {"},
		{name: "missing semicolon", data: `hostname = "x"`, want: "expected ;"},
		{name: "missing equal", data: `hostname "x";`, want: "expected ="},
		{name: "invalid value", data: "hostname = SystemTime;", want: "expected value"},
		{name: "missing bracket", data: `motd[ = { "x" };`, want: "expected ]"},
		{name: "array close", data: `motd[] = { "x";`, want: "expected }"},
		{name: "array comma missing value", data: `motd[] = { "x", ;`, want: "expected value"},
		{name: "array trailing comma", data: `motd[] = { "x", };`, want: "expected value"},
		{name: "unterminated string eof", data: `hostname = "bad`, want: "unterminated string"},
		{name: "class semicolon", data: `class Missions {}`, want: "expected ;"},
		{name: "class child error", data: `class Missions { hostname = ; };`, want: "expected value"},
		{name: "slash", data: `/`, want: "unexpected character"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseData([]byte(test.data))
			if err == nil {
				t.Fatal("ParseData err = nil")
			}
			assertContains(t, err.Error(), test.want)
		})
	}

	for _, kind := range []tokenKind{tokenEOF, tokenIdent, tokenString, tokenNumber, tokenEqual, tokenSemicolon, tokenLBrace, tokenRBrace, tokenLBracket, tokenRBracket, tokenComma, tokenKind(99)} {
		if tokenName(kind) == "" {
			t.Fatalf("tokenName(%d) returned blank", kind)
		}
	}
	assertEqual(t, DisplayValue(Value{Kind: ValueKind(99)}), "")
	assertEqual(t, renderValue(Value{Kind: ValueKind(99)}), "")

	escaped, err := ParseData([]byte(`hostname = "a\\b";`))
	if err != nil {
		t.Fatalf("ParseData escaped string returned error: %v", err)
	}
	assertEqual(t, escaped.Assignments[0].Value.String, `a\b`)
}

func TestUpdateFieldExistingAndDuplicateAssignments(t *testing.T) {
	data := []byte("    hostname = \"Old\"; // keep comment\r\nhostname = \"Old\";\r\n")

	updated, changed, existed, err := UpdateField(data, "hostname", []string{`New "Name"`})
	if err != nil {
		t.Fatalf("UpdateField returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertEqual(t, existed, true)
	output := string(updated)
	assertContains(t, output, `    hostname = "New \"Name\""; // keep comment`)
	assertContains(t, output, "\r\nhostname = \"New \\\"Name\\\"\";")

	unchanged, changed, existed, err := UpdateField([]byte(`hostname = "Same";`), "hostname", []string{"Same"})
	if err != nil {
		t.Fatalf("UpdateField same returned error: %v", err)
	}
	assertEqual(t, string(unchanged), `hostname = "Same";`)
	assertEqual(t, changed, false)
	assertEqual(t, existed, true)
}

func TestUpdateExistingUnknownField(t *testing.T) {
	data := []byte("vppDisablePassword = 1; // VPP setting\ncustomName = \"Old\";\ncustomBool = false;\ncustomArray[] = { \"a\", \"b\" };\n")

	updated, changed, existed, err := UpdateField(data, "vppDisablePassword", []string{"0"})
	if err != nil {
		t.Fatalf("UpdateField number returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertEqual(t, existed, true)
	assertContains(t, string(updated), "vppDisablePassword = 0; // VPP setting")

	updated, _, _, err = UpdateField(updated, "customName", []string{"New"})
	if err != nil {
		t.Fatalf("UpdateField string returned error: %v", err)
	}
	assertContains(t, string(updated), `customName = "New";`)

	updated, _, _, err = UpdateField(updated, "customBool", []string{"true"})
	if err != nil {
		t.Fatalf("UpdateField bool returned error: %v", err)
	}
	assertContains(t, string(updated), `customBool = true;`)

	updated, _, _, err = UpdateField(updated, "customArray[]", []string{"c", "d"})
	if err != nil {
		t.Fatalf("UpdateField array returned error: %v", err)
	}
	assertContains(t, string(updated), `customArray[] = { "c", "d" };`)

	if _, _, _, err := UpdateField(updated, "vppDisablePassword", []string{"not-a-number"}); err == nil {
		t.Fatal("UpdateField bad unknown number err = nil")
	}
	if _, _, _, err := UpdateField(updated, "customBool", []string{"yes"}); err == nil {
		t.Fatal("UpdateField bad unknown bool err = nil")
	}
	if _, _, _, err := UpdateField(updated, "customName", []string{"a", "b"}); err == nil {
		t.Fatal("UpdateField scalar multi-value err = nil")
	}
}

func TestUpdateFieldMissingInDocumentOrderAndMissionBlock(t *testing.T) {
	data := []byte("maxPlayers = 60;\nclass Missions\n{\n};\n")

	updated, changed, existed, err := UpdateField(data, "description", []string{"A server"})
	if err != nil {
		t.Fatalf("UpdateField description returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertEqual(t, existed, false)
	assertContains(t, string(updated), "description = \"A server\";\nmaxPlayers")

	updated, _, _, err = UpdateField([]byte("hostname = \"Only\";"), "respawnTime", []string{"5"})
	if err != nil {
		t.Fatalf("UpdateField respawnTime returned error: %v", err)
	}
	assertContains(t, string(updated), "hostname = \"Only\";\nrespawnTime = 5;")

	updated, _, existed, err = UpdateField([]byte("hostname = \"Only\";\n"), "Missions.DayZ.template", []string{"empty.deerisle"})
	if err != nil {
		t.Fatalf("UpdateField template returned error: %v", err)
	}
	assertEqual(t, existed, false)
	assertContains(t, string(updated), "class Missions\n{\n    class DayZ")
	assertContains(t, string(updated), `template = "empty.deerisle";`)

	updated, _, _, err = UpdateField([]byte("hostname = \"Only\";\n"), "motd[]", []string{"one", "two"})
	if err != nil {
		t.Fatalf("UpdateField motd returned error: %v", err)
	}
	assertContains(t, string(updated), `motd[] = { "one", "two" };`)

	updated, _, _, err = UpdateField([]byte("hostname = \"Only\";\nclass Missions\n{\n    class DayZ\n    {\n        template = \"old\";\n    };\n};\n"), "respawnTime", []string{"7"})
	if err != nil {
		t.Fatalf("UpdateField before class returned error: %v", err)
	}
	assertContains(t, string(updated), "respawnTime = 7;\nclass Missions")

	updated, _, _, err = UpdateField([]byte("hostname = \"Only\";\n"), "disableBanlist", []string{"true"})
	if err != nil {
		t.Fatalf("UpdateField bool returned error: %v", err)
	}
	assertContains(t, string(updated), "disableBanlist = true;")
}

func TestInsertUnknownFieldRequiresAllow(t *testing.T) {
	data := []byte("hostname = \"Server\";\nclass Missions\n{\n};\n")

	if _, _, _, err := UpdateField(data, "customOption", []string{"1"}); err == nil {
		t.Fatal("unknown insert without allow err = nil")
	}
	if _, _, _, err := UpdateField(data, "Mods.Custom.option", []string{"1"}, UpdateFieldOptions{AllowUnknown: true}); err == nil {
		t.Fatal("unknown nested insert err = nil")
	}

	updated, changed, existed, err := UpdateField(data, "customOption", []string{"1"}, UpdateFieldOptions{AllowUnknown: true})
	if err != nil {
		t.Fatalf("UpdateField unknown insert returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertEqual(t, existed, false)
	assertContains(t, string(updated), "customOption = 1; // #dzcli:allow-unknown\nclass Missions")
	if err := ValidateData(updated, "custom.cfg"); err != nil {
		t.Fatalf("ValidateData annotated unknown returned error: %v", err)
	}

	updated, _, _, err = UpdateField([]byte("hostname = \"Server\";\n"), "customText", []string{"alpha"}, UpdateFieldOptions{AllowUnknown: true})
	if err != nil {
		t.Fatalf("UpdateField unknown text returned error: %v", err)
	}
	assertContains(t, string(updated), `customText = "alpha"; // #dzcli:allow-unknown`)

	updated, _, _, err = UpdateField([]byte("hostname = \"Server\";\n"), "customArray[]", []string{"one", "two"}, UpdateFieldOptions{AllowUnknown: true})
	if err != nil {
		t.Fatalf("UpdateField unknown array returned error: %v", err)
	}
	assertContains(t, string(updated), `customArray[] = { "one", "two" }; // #dzcli:allow-unknown`)
}

func TestUpdateFieldErrors(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		values []string
		data   string
		want   string
	}{
		{name: "unsupported", field: "custom", values: []string{"1"}, data: "", want: "unsupported server config field"},
		{name: "missing value", field: "hostname", values: nil, data: "", want: "--value is required"},
		{name: "too many values", field: "hostname", values: []string{"a", "b"}, data: "", want: "accepts exactly one --value"},
		{name: "bad bool", field: "disableBanlist", values: []string{"0"}, data: "", want: "expected true or false"},
		{name: "bad int", field: "maxPlayers", values: []string{"many"}, data: "", want: "expected integer"},
		{name: "bad float", field: "speedhackDetection", values: []string{"fast"}, data: "", want: "expected number"},
		{name: "syntax", field: "hostname", values: []string{"x"}, data: "{", want: "expected identifier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := UpdateField([]byte(test.data), test.field, test.values)
			if err == nil {
				t.Fatal("UpdateField err = nil")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestPostMutationParseErrors(t *testing.T) {
	_, _, _, err := UpdateField([]byte(`hostname = "Old";`), "hostname", []string{"bad\nvalue"})
	if err == nil {
		t.Fatal("UpdateField replacement post-parse err = nil")
	}
	assertContains(t, err.Error(), "unterminated string")

	_, _, _, err = UpdateField([]byte(`maxPlayers = 60;`), "description", []string{"bad\nvalue"})
	if err == nil {
		t.Fatal("UpdateField insert post-parse err = nil")
	}
	assertContains(t, err.Error(), "unterminated string")

	original := parseUpdatedData
	defer func() { parseUpdatedData = original }()
	parseUpdatedData = func([]byte) (Document, error) {
		return Document{}, errors.New("post-parse failed")
	}
	if _, _, err := DeleteField([]byte(`hostname = "Old";`), "hostname"); err == nil {
		t.Fatal("DeleteField post-parse err = nil")
	}
}

func TestDeleteField(t *testing.T) {
	data := []byte("hostname = \"Server\";\nvppDisablePassword = 1; // remove this\nclass Missions\n{\n    class DayZ\n    {\n        template = \"empty.deerisle\";\n    };\n};")

	updated, changed, err := DeleteField(data, "vppDisablePassword")
	if err != nil {
		t.Fatalf("DeleteField unknown returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertNotContains(t, string(updated), "vppDisablePassword")

	updated, changed, err = DeleteField(updated, "Missions.DayZ.template")
	if err != nil {
		t.Fatalf("DeleteField template returned error: %v", err)
	}
	assertEqual(t, changed, true)
	assertNotContains(t, string(updated), "template")

	unchanged, changed, err := DeleteField(updated, "missing")
	if err != nil {
		t.Fatalf("DeleteField missing returned error: %v", err)
	}
	assertEqual(t, string(unchanged), string(updated))
	assertEqual(t, changed, false)

	if _, _, err := DeleteField([]byte("{"), "hostname"); err == nil {
		t.Fatal("DeleteField syntax err = nil")
	}
}

func TestFileOperations(t *testing.T) {
	path := writeTempConfig(t, `hostname = "Old";`)

	mutation, existed, err := UpdateFieldFile(path, "hostname", []string{"New"})
	if err != nil {
		t.Fatalf("UpdateFieldFile returned error: %v", err)
	}
	assertEqual(t, existed, true)
	if err := WriteFileMutation(path, mutation); err != nil {
		t.Fatalf("WriteFileMutation returned error: %v", err)
	}
	data, _ := os.ReadFile(path)
	assertContains(t, string(data), `hostname = "New";`)

	missing, err := DeleteFieldFile(path, "missing")
	if err != nil {
		t.Fatalf("DeleteFieldFile missing returned error: %v", err)
	}
	assertEqual(t, missing.Changed, false)
	if err := WriteFileMutation(path, missing); err != nil {
		t.Fatalf("WriteFileMutation unchanged returned error: %v", err)
	}

	if err := ValidateFile(path); err != nil {
		t.Fatalf("ValidateFile returned error: %v", err)
	}
	if _, err := ListFieldsFile(filepath.Join(t.TempDir(), "missing.cfg")); err == nil {
		t.Fatal("ListFieldsFile missing err = nil")
	}
	if err := ValidateFile(filepath.Join(t.TempDir(), "missing.cfg")); err == nil {
		t.Fatal("ValidateFile missing err = nil")
	}
	if _, err := DeleteFieldFile(filepath.Join(t.TempDir(), "missing.cfg"), "hostname"); err == nil {
		t.Fatal("DeleteFieldFile missing err = nil")
	}
	if _, _, err := UpdateFieldFile(filepath.Join(t.TempDir(), "missing.cfg"), "hostname", []string{"x"}); err == nil {
		t.Fatal("UpdateFieldFile missing err = nil")
	}
	if _, err := ListFieldsFile(writeTempConfig(t, "{")); err == nil {
		t.Fatal("ListFieldsFile parse err = nil")
	}
	if err := ValidateData([]byte("{"), "bad.cfg"); err == nil {
		t.Fatal("ValidateData parse err = nil")
	}
	if _, _, err := UpdateFieldFile(path, "verifySignatures", []string{"1"}); err == nil {
		t.Fatal("UpdateFieldFile validation err = nil")
	}
	if _, err := DeleteFieldFile(writeTempConfig(t, "{"), "hostname"); err == nil {
		t.Fatal("DeleteFieldFile parse err = nil")
	}
}

func TestInjectedFileOperationErrors(t *testing.T) {
	originalRead := readFile
	originalWrite := writeFile
	defer func() {
		readFile = originalRead
		writeFile = originalWrite
	}()

	readFile = func(string) ([]byte, error) {
		return nil, errors.New("read failed")
	}
	if _, _, err := UpdateFieldFile(writeTempConfig(t, `hostname = "x";`), "hostname", []string{"x"}); err == nil {
		t.Fatal("UpdateFieldFile read err = nil")
	}

	writeFile = func(string, []byte, fs.FileMode) error {
		return errors.New("write failed")
	}
	err := WriteFileMutation("serverDZ.cfg", FileMutation{Data: []byte("x"), Mode: 0o600, Changed: true})
	if err == nil {
		t.Fatal("WriteFileMutation err = nil")
	}
}

func TestSortFieldValuesOrdersDocumentedFieldsBeforeUnknownTies(t *testing.T) {
	values := SortFieldValues([]FieldValue{
		{Field: "zzz", Value: "1"},
		{Field: "hostname", Value: "server"},
		{Field: "aaa", Value: "2"},
	})

	got := strings.Join([]string{values[0].Field, values[1].Field, values[2].Field}, ",")
	assertEqual(t, got, "hostname,aaa,zzz")
	assertEqual(t, NormalizeField("motd[]"), "motd")

	assertEqual(t, DisplayValue(Value{Kind: ValueBool, Bool: true}), "true")
	assertEqual(t, renderAssignment(fieldSpec{Name: "custom"}, Value{Kind: ValueNumber, Text: "1"}, "  "), "  custom = 1;")
	assertEqual(t, renderValue(Value{Kind: ValueBool, Bool: true}), "true")
	assertEqual(t, renderValue(Value{Kind: ValueBool}), "false")
	assertEqual(t, string(removeRanges([]byte(`hostname = "x";`), []span{{start: 0, end: len(`hostname = "x";`)}})), "")
	assertEqual(t, string(removeRanges([]byte("a = 1;\nb = 2;\n"), []span{{start: 7, end: 13}, {start: 0, end: 6}})), "")
	assertEqual(t, string(insertAt([]byte("ab"), 1, "X", "\n")), "a\nX\nb")
	assertEqual(t, lineStart([]byte("abc"), 10), 0)
	assertEqual(t, lineEndWithBreak([]byte("abc"), 1), 3)
	assertEqual(t, eofInsertOffset([]byte(" \r\n\t")), 0)

	if _, err := parseRawFieldValue(fieldSpec{Name: "custom", Type: fieldType(99)}, "x"); err == nil {
		t.Fatal("parseRawFieldValue unknown type err = nil")
	}
	for _, test := range []struct {
		spec  fieldSpec
		value Value
		want  string
	}{
		{spec: fieldSpec{Name: "motd", Type: fieldStringArray}, value: Value{Kind: ValueString, String: "line"}, want: "expected string array"},
		{spec: fieldSpec{Name: "enableWhitelist", Type: fieldIntBool}, value: Value{Kind: ValueString, String: "yes"}, want: "expected integer"},
		{spec: fieldSpec{Name: "verifySignatures", Type: fieldFixedInt}, value: Value{Kind: ValueString, String: "two"}, want: "expected integer"},
		{spec: fieldSpec{Name: "networkObjectBatchBandwidthLimit", Type: fieldFloatMin}, value: Value{Kind: ValueString, String: "bad"}, want: "expected number"},
		{spec: fieldSpec{Name: "lightingConfig", Type: fieldIntEnum, IntEnum: []int{0, 1, 2}}, value: Value{Kind: ValueString, String: "dark"}, want: "expected integer"},
		{spec: fieldSpec{Name: "timeStampFormat", Type: fieldStringEnum, StringEnum: []string{"Full", "Short"}}, value: Value{Kind: ValueNumber, Text: "1"}, want: "expected string"},
	} {
		err := validateValue(test.spec, test.value)
		if err == nil {
			t.Fatalf("validateValue(%s) err = nil", test.spec.Name)
		}
		assertContains(t, err.Error(), test.want)
	}
	if _, err := ValueFromStrings(fieldSpec{Name: "disableBanlist", Type: fieldBool}, []string{"false"}); err != nil {
		t.Fatalf("ValueFromStrings false returned error: %v", err)
	}
	if _, err := UnknownValueFromStrings(nil, nil); err == nil {
		t.Fatal("UnknownValueFromStrings empty err = nil")
	}
	boolValue, err := UnknownValueFromStrings([]string{"true"}, nil)
	if err != nil {
		t.Fatalf("UnknownValueFromStrings bool returned error: %v", err)
	}
	assertEqual(t, boolValue.Kind, ValueBool)
	numberArray := Value{Kind: ValueArray, Array: []Value{{Kind: ValueNumber, Text: "1"}}}
	if _, err := UnknownValueFromStrings([]string{"bad"}, &numberArray); err == nil {
		t.Fatal("UnknownValueFromStrings bad array number err = nil")
	}
	if _, err := unknownScalarFromKind("maybe", ValueKind(99)); err != nil {
		t.Fatalf("unknownScalarFromKind fallback returned error: %v", err)
	}
	if _, err := parseUnknownNumber(" "); err == nil {
		t.Fatal("parseUnknownNumber blank err = nil")
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "testdata", "serverconfig"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serverDZ.cfg")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%q contains %q", haystack, needle)
	}
}

func assertEqual[T comparable](t *testing.T, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
