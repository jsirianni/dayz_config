package gameplayconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateFixtures(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "valid current", file: "valid.json"},
		{name: "older alias", file: "older_alias.json"},
		{name: "minimal", file: "minimal.json"},
		{name: "malformed", file: "malformed.json", want: "EOF"},
		{name: "duplicate", file: "duplicate.json", want: "duplicate key"},
		{name: "unknown", file: "unknown.json", want: `unknown field "ModdedData"`},
		{name: "invalid type", file: "invalid_type.json", want: "must be a boolean"},
		{name: "invalid enum", file: "invalid_enum.json", want: "must be one of 0, 1, 2"},
		{name: "invalid array length", file: "invalid_array_length.json", want: "exactly 12"},
		{name: "invalid argb", file: "invalid_argb.json", want: "0xAARRGGBB"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateFile(fixturePath(t, test.file))
			if test.want == "" && err != nil {
				t.Fatalf("ValidateFile returned error: %v", err)
			}
			if test.want != "" {
				assertErrorContains(t, err, test.want)
			}
		})
	}
}

func TestParseDataErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "empty", data: "", want: "empty JSON document"},
		{name: "multiple", data: `{} {}`, want: "expected one JSON document"},
		{name: "trailing invalid", data: `{} x`, want: "invalid character"},
		{name: "root array", data: `[]`, want: "root must be an object"},
		{name: "close object error", data: `{"x": 1`, want: "EOF"},
		{name: "close array error", data: `{"x": [1`, want: "EOF"},
		{name: "object key error", data: `{"x": 1,`, want: "EOF"},
		{name: "array child error", data: `{"x": [1,`, want: "empty JSON document"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseData([]byte(test.data))
			assertErrorContains(t, err, test.want)
		})
	}
}

func TestValidateDataAndDocumentErrors(t *testing.T) {
	assertErrorContains(t, ValidateData([]byte(`{"GeneralData":{"disableBaseDamage":null}}`), "inline"), "must be a boolean")
	assertErrorContains(t, ValidateData([]byte(`{"version":{}}`), "inline"), "must be an integer")
	assertErrorContains(t, ValidateData([]byte(`{"version":true}`), "inline"), "must be an integer")
	assertErrorContains(t, ValidateData([]byte(`{"version":999999999999999999999999999999}`), "inline"), "must be an integer")
	assertErrorContains(t, ValidateData([]byte(`{"version":1.2}`), "inline"), "must be an integer")
	assertErrorContains(t, ValidateData([]byte(`{"PlayerData":{"MovementData":{"timeToSprint":0}}}`), "inline"), "greater than or equal to 0.01")
	assertErrorContains(t, ValidateData([]byte(`{"UIData":{"HitIndicationData":{"hitDirectionBreakPointRelative":2}}}`), "inline"), "less than or equal to 1")
	assertErrorContains(t, ValidateData([]byte(`{"UIData":{"HitIndicationData":{"hitDirectionIndicatorColorStr":123}}}`), "inline"), "must be a string")
	assertErrorContains(t, ValidateData([]byte(`{"VehicleData":{"boatDecayMultiplier":true}}`), "inline"), "must be a number")
	assertErrorContains(t, ValidateData([]byte(`{"WorldsData":{"objectSpawnersArr":true}}`), "inline"), "must be an array")
	assertErrorContains(t, ValidateData([]byte(`{"WorldsData":{"environmentMinTemps":[1,true,3,4,5,6,7,8,9,10,11,12]}}`), "inline"), "[1] must be a number")
	assertErrorContains(t, ValidateData([]byte(`{"WorldsData":{"objectSpawnersArr":[1]}}`), "inline"), "[0] must be a string")
	assertErrorContains(t, ValidateData([]byte(`{"Modded":true}`), "inline"), `unknown field "Modded"`)
	assertErrorContains(t, ValidateDocument(nil), "root must be an object")
	if err := ValidateData([]byte(`{"PlayerData":{"StaminaData":{"staminaMax":0,"staminaMinCap":0}}}`), "inline"); err != nil {
		t.Fatalf("zero stamina warning case returned error: %v", err)
	}
}

func TestListAndGetFields(t *testing.T) {
	document := mustParse(t, `{
		"Unknown": {"leaf": true},
		"version": 123,
		"PlayerData": {"StaminaData": {"staminaMax": 100}},
		"WorldsData": {"environmentMinTemps": [1,2,3,4,5,6,7,8,9,10,11,12]}
	}`)
	values := ListFields(document)
	got := joinFields(values)
	assertContains(t, got, "version=123")
	assertContains(t, got, "PlayerData.StaminaData.staminaMax=100")
	assertContains(t, got, "Unknown.leaf=true")
	if !strings.HasPrefix(got, "version=123") {
		t.Fatalf("fields not sorted by schema order: %s", got)
	}

	value, ok, err := GetFieldFile(writeTempGameplay(t, `{"PlayerData":{"StaminaData":{"staminaMax":100}}}`), "PlayerData.StaminaData.staminaMax")
	if err != nil || !ok || value.Value != "100" {
		t.Fatalf("GetFieldFile = %#v, %v, %v", value, ok, err)
	}
	_, ok, err = GetFieldFile(writeTempGameplay(t, `{"version":123}`), "missing")
	if err != nil || ok {
		t.Fatalf("missing GetFieldFile = %v, %v", ok, err)
	}
	valuesFromFile, err := ListFieldsFile(writeTempGameplay(t, `{"version":123}`))
	if err != nil || len(valuesFromFile) != 1 {
		t.Fatalf("ListFieldsFile = %#v, %v", valuesFromFile, err)
	}
	assertErrorContains(t, func() error {
		_, err := ListFieldsFile(filepath.Join(t.TempDir(), "missing.json"))
		return err
	}(), "read")
	assertErrorContains(t, func() error {
		_, err := ListFieldsFile(writeTempGameplay(t, `{`))
		return err
	}(), "parse")
	assertErrorContains(t, func() error {
		_, _, err := GetFieldFile(writeTempGameplay(t, `{`), "version")
		return err
	}(), "parse")
	assertErrorContains(t, func() error {
		_, _, err := GetFieldFile(filepath.Join(t.TempDir(), "missing.json"), "version")
		return err
	}(), "read")
}

func TestUpdateFieldScalarArraysAndAliases(t *testing.T) {
	data := []byte(`{"PlayerData":{"StaminaData":{"staminaMax":100},"spawnGearPresetFiles":["old"]},"BaseBuildingData":{"HologramData":{"disableColdAreaBuildingCheck":false}}}`)
	updated, changed, existed, err := UpdateField(data, "PlayerData.StaminaData.staminaMax", []string{"150"}, UpdateFieldOptions{})
	if err != nil || !changed || !existed {
		t.Fatalf("UpdateField scalar = changed %v existed %v err %v", changed, existed, err)
	}
	assertContains(t, string(updated), `"staminaMax": 150`)

	updated, changed, existed, err = UpdateField(updated, "PlayerData.spawnGearPresetFiles", []string{"a.json", "b.json"}, UpdateFieldOptions{})
	if err != nil || !changed || !existed {
		t.Fatalf("UpdateField string array = changed %v existed %v err %v", changed, existed, err)
	}
	assertContains(t, string(updated), `"spawnGearPresetFiles": ["a.json", "b.json"]`)

	updated, changed, existed, err = UpdateField(updated, "WorldsData.environmentMinTemps", twelveValues(), UpdateFieldOptions{})
	if err != nil || !changed || existed {
		t.Fatalf("UpdateField inserted numeric array = changed %v existed %v err %v", changed, existed, err)
	}
	assertContains(t, string(updated), `"environmentMinTemps": [-3, -2, 0, 4, 9, 14, 18, 17, 12, 7, 4, 0]`)

	updated, _, existed, err = UpdateField(updated, "BaseBuildingData.HologramData.disableColdAreaPlacementCheck", []string{"true"}, UpdateFieldOptions{})
	if err != nil || !existed {
		t.Fatalf("UpdateField alias = existed %v err %v", existed, err)
	}
	assertContains(t, string(updated), `"disableColdAreaBuildingCheck": true`)

	updated, changed, existed, err = UpdateField(updated, "UIData.use3DMap", []string{"false"}, UpdateFieldOptions{})
	if err != nil || !changed || existed {
		t.Fatalf("UpdateField inserted bool = changed %v existed %v err %v", changed, existed, err)
	}
	assertContains(t, string(updated), `"use3DMap": false`)

	updated, changed, _, err = UpdateField(updated, "UIData.HitIndicationData.hitDirectionIndicatorColorStr", []string{"0XFFAABBCC"}, UpdateFieldOptions{})
	if err != nil || !changed {
		t.Fatalf("UpdateField string = changed %v err %v", changed, err)
	}
	assertContains(t, string(updated), `"hitDirectionIndicatorColorStr": "0XFFAABBCC"`)
}

func TestUpdateFieldErrorsAndNoChange(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		field  string
		values []string
		opts   UpdateFieldOptions
		want   string
	}{
		{name: "parse", data: `{`, field: "version", values: []string{"1"}, want: "EOF"},
		{name: "unsupported", data: `{}`, field: "Unknown.value", values: []string{"1"}, want: "unsupported gameplay field"},
		{name: "missing value", data: `{}`, field: "version", want: "--value is required"},
		{name: "bad bool", data: `{}`, field: "UIData.use3DMap", values: []string{"1"}, want: "must be true or false"},
		{name: "bad int", data: `{}`, field: "version", values: []string{"1.2"}, want: "not an integer"},
		{name: "bad float", data: `{}`, field: "VehicleData.boatDecayMultiplier", values: []string{"x"}, want: "not a number"},
		{name: "bad float array", data: `{}`, field: "WorldsData.environmentMinTemps", values: []string{"x"}, want: "not a number"},
		{name: "bad enum", data: `{}`, field: "WorldsData.lightingConfig", values: []string{"4"}, want: "one of 0, 1, 2"},
		{name: "multiple scalar", data: `{}`, field: "version", values: []string{"1", "2"}, want: "exactly one --value"},
		{name: "clear scalar", data: `{}`, field: "version", opts: UpdateFieldOptions{Clear: true}, want: "--clear is only valid"},
		{name: "clear exact numeric array", data: `{}`, field: "WorldsData.environmentMinTemps", opts: UpdateFieldOptions{Clear: true}, want: "exactly 12"},
		{name: "parent scalar", data: `{"PlayerData":1}`, field: "PlayerData.StaminaData.staminaMax", values: []string{"1"}, want: "PlayerData is not an object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := UpdateField([]byte(test.data), test.field, test.values, test.opts)
			assertErrorContains(t, err, test.want)
		})
	}

	updated, changed, existed, err := UpdateField([]byte("{\r\n\t\"version\": 123\r\n}"), "version", []string{"123"}, UpdateFieldOptions{})
	if err != nil || changed || !existed {
		t.Fatalf("same value update = changed %v existed %v err %v", changed, existed, err)
	}
	assertContains(t, string(updated), "\r\n")

	originalParseUpdated := parseUpdatedData
	defer func() { parseUpdatedData = originalParseUpdated }()
	parseUpdatedData = func([]byte) (*Value, error) { return nil, errors.New("post parse failed") }
	_, _, _, err = UpdateField([]byte(`{}`), "version", []string{"1"}, UpdateFieldOptions{})
	assertErrorContains(t, err, "post parse failed")

	if _, err := scalarValueFromString(specMap["WorldsData.objectSpawnersArr"], "x"); err == nil {
		t.Fatal("scalarValueFromString array err = nil")
	}
	if _, err := parseFloatValue(&Value{Kind: valueNumber, Number: "bad"}); err == nil {
		t.Fatal("parseFloatValue bad err = nil")
	}
}

func TestDeleteField(t *testing.T) {
	data := []byte(`{"version":123,"Unknown":{"leaf":true},"BaseBuildingData":{"HologramData":{"disableColdAreaBuildingCheck":false}}}`)
	updated, changed, err := DeleteField(data, "Unknown.leaf")
	if err != nil || !changed {
		t.Fatalf("DeleteField unknown = changed %v err %v", changed, err)
	}
	assertNotContains(t, string(updated), `"leaf"`)

	updated, changed, err = DeleteField(updated, "BaseBuildingData.HologramData.disableColdAreaPlacementCheck")
	if err != nil || !changed {
		t.Fatalf("DeleteField alias = changed %v err %v", changed, err)
	}
	assertNotContains(t, string(updated), `disableColdAreaBuildingCheck`)

	updated, changed, err = DeleteField(updated, "missing")
	if err != nil || changed {
		t.Fatalf("DeleteField missing = changed %v err %v", changed, err)
	}

	_, _, err = DeleteField([]byte(`{`), "version")
	assertErrorContains(t, err, "EOF")

	originalParseUpdated := parseUpdatedData
	defer func() { parseUpdatedData = originalParseUpdated }()
	parseUpdatedData = func([]byte) (*Value, error) { return nil, errors.New("post delete failed") }
	_, _, err = DeleteField([]byte(`{"version":123}`), "version")
	assertErrorContains(t, err, "post delete failed")
}

func TestFileMutationHelpers(t *testing.T) {
	path := writeTempGameplay(t, `{"version":123}`)
	mutation, existed, err := UpdateFieldFile(path, "version", []string{"124"})
	if err != nil || !existed || !mutation.Changed {
		t.Fatalf("UpdateFieldFile = %#v existed %v err %v", mutation, existed, err)
	}
	if err := WriteFileMutation(path, mutation); err != nil {
		t.Fatalf("WriteFileMutation returned error: %v", err)
	}
	assertFileContains(t, path, `"version": 124`)

	mutation, err = DeleteFieldFile(path, "version")
	if err != nil || !mutation.Changed {
		t.Fatalf("DeleteFieldFile = %#v err %v", mutation, err)
	}
	if err := WriteFileMutation(path, FileMutation{Changed: false}); err != nil {
		t.Fatalf("no-change WriteFileMutation returned error: %v", err)
	}

	assertErrorContains(t, ValidateFile(filepath.Join(t.TempDir(), "missing.json")), "read")
	assertErrorContains(t, func() error {
		_, _, err := UpdateFieldFile(filepath.Join(t.TempDir(), "missing.json"), "version", []string{"1"})
		return err
	}(), "stat")
	assertErrorContains(t, func() error {
		_, _, err := UpdateFieldFile(path, "missing", []string{"1"})
		return err
	}(), "unsupported gameplay field")
	originalStat := statFile
	originalRead := readFile
	statFile = func(string) (os.FileInfo, error) { return fakeInfo{}, nil }
	readFile = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	assertErrorContains(t, func() error {
		_, _, err := UpdateFieldFile("cfggameplay.json", "version", []string{"1"})
		return err
	}(), "read failed")
	assertErrorContains(t, func() error {
		_, err := DeleteFieldFile("cfggameplay.json", "version")
		return err
	}(), "read failed")
	statFile = originalStat
	readFile = originalRead
	assertErrorContains(t, func() error {
		_, err := DeleteFieldFile(filepath.Join(t.TempDir(), "missing.json"), "version")
		return err
	}(), "stat")
	assertErrorContains(t, func() error {
		_, err := DeleteFieldFile(writeTempGameplay(t, `{`), "version")
		return err
	}(), "EOF")

	originalWrite := writeFile
	defer func() { writeFile = originalWrite }()
	writeFile = func(string, []byte, fs.FileMode) error { return errors.New("write failed") }
	assertErrorContains(t, WriteFileMutation(path, FileMutation{Data: []byte("{}"), Mode: 0o600, Changed: true}), "write failed")
}

func TestHelpers(t *testing.T) {
	document := mustParse(t, `{"a":{"b":[true,null,"x",1]}}`)
	if got := DisplayValue(findPath(document, "a")); got != `{"b":[true,null,"x",1]}` {
		t.Fatalf("DisplayValue object = %q", got)
	}
	if got := DisplayValue(&Value{Kind: valueArray, Array: []*Value{{Kind: valueNumber, Number: "1"}, {Kind: valueNumber, Number: "2"}}}); got != `[1,2]` {
		t.Fatalf("DisplayValue array = %q", got)
	}
	if got := DisplayValue(&Value{Kind: valueObject, Object: map[string]*Value{"a": {Kind: valueNumber, Number: "1"}, "b": {Kind: valueNumber, Number: "2"}}, Order: []string{"a", "b"}}); got != `{"a":1,"b":2}` {
		t.Fatalf("DisplayValue multi object = %q", got)
	}
	values := []FieldValue{{Field: "z.unknown", Value: "1"}, {Field: "a.unknown", Value: "2"}}
	SortFieldValues(values)
	if values[0].Field != "a.unknown" {
		t.Fatalf("unknown sort = %#v", values)
	}
	keys := sortedObjectKeys("WorldsData", &Value{Kind: valueObject, Object: map[string]*Value{"environmentMinTemps": {}, "lightingConfig": {}}, Order: []string{"environmentMinTemps", "lightingConfig"}})
	if strings.Join(keys, ",") != "lightingConfig,environmentMinTemps" {
		t.Fatalf("sortedObjectKeys known = %#v", keys)
	}
	renderedKnownOrder := string(renderJSON([]byte("{}"), mustParse(t, `{"WorldsData":{"lightingConfig":1},"version":123,"GeneralData":{"disableBaseDamage":false}}`)))
	if strings.Index(renderedKnownOrder, `"GeneralData"`) > strings.Index(renderedKnownOrder, `"WorldsData"`) {
		t.Fatalf("known order render = %s", renderedKnownOrder)
	}
	if findPath(document, "") != document || findPath(document, "a.b.c") != nil {
		t.Fatal("findPath root/missing mismatch")
	}
	walkLeaves("", nil, &[]FieldValue{})
	if NormalizeField(" .a.b. ") != "a.b" {
		t.Fatal("NormalizeField did not trim dots and spaces")
	}
	if displayPath("") != "<root>" || displayPath("a") != "a" {
		t.Fatal("displayPath mismatch")
	}
	if got := removeString([]string{"a", "b", "a"}, "a"); strings.Join(got, ",") != "b" {
		t.Fatalf("removeString = %#v", got)
	}
	if !containsInt([]int{1, 2}, 2) || containsInt([]int{1, 2}, 3) {
		t.Fatal("containsInt mismatch")
	}
	if formatFloat(1.5) != "1.5" || formatInts([]int{1, 2}) != "1, 2" {
		t.Fatal("format helper mismatch")
	}
	if _, ok := specForPath("missing"); ok {
		t.Fatal("specForPath found missing")
	}
	if aliases := strings.Join(aliasPaths("BaseBuildingData.HologramData.disableColdAreaPlacementCheck"), ","); !strings.Contains(aliases, "disableColdAreaBuildingCheck") {
		t.Fatalf("aliasPaths = %q", aliases)
	}
	assertErrorContains(t, deletePath(document, "a.b.c"), "not found")

	decoder := jsonDecoderWithoutUseNumber(`1`)
	_, err := parseValue(decoder, "")
	assertErrorContains(t, err, "unsupported JSON token")
	delimiterDecoder := jsonDecoderWithoutUseNumber(`{}`)
	if _, err := delimiterDecoder.Token(); err != nil {
		t.Fatalf("Token returned error: %v", err)
	}
	_, err = parseValue(delimiterDecoder, "")
	assertErrorContains(t, err, "unexpected delimiter")
	badDecoder := jsonDecoderWithoutUseNumber(`!`)
	_, err = parseValue(badDecoder, "")
	assertErrorContains(t, err, "invalid character")

	var builder strings.Builder
	renderValue(&builder, "", &Value{Kind: valueString, String: "x"}, 0, "\t", "\n")
	renderValue(&builder, "", &Value{Kind: valueNull}, 0, "\t", "\n")
	assertContains(t, builder.String(), `"x"`)
	assertContains(t, builder.String(), `null`)

	crlfUpdated, _, _, err := UpdateField([]byte("{\r\n\t\"version\": 123\r\n}"), "version", []string{"124"}, UpdateFieldOptions{})
	if err != nil {
		t.Fatalf("CRLF update returned error: %v", err)
	}
	assertContains(t, string(crlfUpdated), "\r\n")
	if !updateOptions([]UpdateFieldOptions{{Clear: true}}).Clear {
		t.Fatal("updateOptions did not return provided options")
	}

	if err := readLimitedStatBranch(); err != nil {
		t.Fatal(err)
	}
}

func readLimitedStatBranch() error {
	originalStat := statFile
	originalRead := readFile
	defer func() {
		statFile = originalStat
		readFile = originalRead
	}()
	statFile = func(string) (os.FileInfo, error) { return fakeInfo{}, nil }
	readFile = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	_, _, err := readMutableFile("cfg.json")
	if err == nil || !strings.Contains(err.Error(), "read failed") {
		return fmt.Errorf("readMutableFile read branch err = %v", err)
	}
	return nil
}

func jsonDecoderWithoutUseNumber(data string) *json.Decoder {
	return json.NewDecoder(strings.NewReader(data))
}

type fakeInfo struct{}

func (fakeInfo) Name() string       { return "cfggameplay.json" }
func (fakeInfo) Size() int64        { return 0 }
func (fakeInfo) Mode() fs.FileMode  { return 0o600 }
func (fakeInfo) ModTime() time.Time { return time.Time{} }
func (fakeInfo) IsDir() bool        { return false }
func (fakeInfo) Sys() any           { return nil }

func mustParse(t *testing.T, data string) *Value {
	t.Helper()
	document, err := ParseData([]byte(data))
	if err != nil {
		t.Fatalf("ParseData returned error: %v", err)
	}
	return document
}

func twelveValues() []string {
	return []string{"-3", "-2", "0", "4", "9", "14", "18", "17", "12", "7", "4", "0"}
}

func joinFields(values []FieldValue) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, value.Field+"="+value.Value)
	}
	return strings.Join(parts, "\n")
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "gameplayconfig", name))
}

func writeTempGameplay(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfggameplay.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp gameplay: %v", err)
	}
	return path
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	assertContains(t, string(data), want)
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

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("err = nil, want %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want %q", err.Error(), want)
	}
}
