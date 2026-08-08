package weatherconfig

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateFixtures(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "valid-modern"},
		{name: "valid-legacy"},
		{name: "valid-partial"},
		{name: "valid-mixed"},
		{name: "malformed", want: "XML syntax error"},
		{name: "wrong-root", want: "expected <weather> root"},
		{name: "unknown-element", want: "unknown element <weather.clouds>"},
		{name: "unknown-attribute", want: "unknown attribute weather.storm@surprise"},
		{name: "invalid-bool", want: "weather.reset must be a boolean"},
		{name: "invalid-float", want: "weather.storm.density must be a number"},
		{name: "invalid-range", want: "weather.storm.density must be less than or equal to 1"},
		{name: "duplicate-field", want: "duplicate field weather.rain.limits.min as attribute and element"},
		{name: "min-greater-than-max", want: "weather.windDirection.limits min must be less than or equal to max"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateFile(fixturePath(t, test.name, "cfgweather.xml"))
			if test.want == "" {
				if err != nil {
					t.Fatalf("ValidateFile returned error: %v", err)
				}
				return
			}
			assertErrorContains(t, err, test.want)
		})
	}
}

func TestValidateDataEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "empty", data: "", want: "missing <weather> root"},
		{name: "text outside root", data: "text<weather />", want: "unexpected text outside root"},
		{name: "multiple roots", data: "<weather /><weather />", want: "multiple root elements"},
		{name: "root unknown attribute", data: `<weather extra="1" />`, want: "unknown attribute weather@extra"},
		{name: "container text", data: `<weather><storm>loud</storm></weather>`, want: "unexpected text in <weather.storm>"},
		{name: "scalar attribute", data: `<weather><storm><density unit="ratio">0.1</density></storm></weather>`, want: "unknown attribute weather.storm.density@unit"},
		{name: "scalar nested element", data: `<weather><storm><density><value>0.1</value></density></storm></weather>`, want: "unexpected element <weather.storm.density.value>"},
		{name: "below minimum", data: `<weather><storm density="-0.1" /></weather>`, want: "weather.storm.density must be greater than or equal to 0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateData([]byte(test.data), "inline.xml")
			assertErrorContains(t, err, test.want)
		})
	}
}

func TestValidateDataParseErrorPaths(t *testing.T) {
	for _, data := range []string{
		`<`,
		`<weather/><extra>`,
		`<notweather>`,
		`<weather><storm><density>`,
		`<weather><unknown>`,
		`<weather><storm><density><value></density></storm></weather>`,
	} {
		err := ValidateData([]byte(data), "inline.xml")
		assertErrorContains(t, err, "parse inline.xml")
	}
}

func TestValidateFileReadError(t *testing.T) {
	original := readFile
	defer func() { readFile = original }()
	readFile = func(string) ([]byte, error) {
		return nil, errors.New("nope")
	}

	err := ValidateFile("missing.xml")

	assertErrorContains(t, err, "read missing.xml: nope")
}

func TestValidationErrorsError(t *testing.T) {
	errs := ValidationErrors{"one", "two"}

	if got := errs.Error(); got != "one; two" {
		t.Fatalf("ValidationErrors.Error() = %q, want one; two", got)
	}
}

func TestBoolValues(t *testing.T) {
	for _, value := range []string{"0", "1", "true", "false", "yes", "no", " TRUE "} {
		if !isBool(value) {
			t.Fatalf("isBool(%q) = false, want true", value)
		}
	}
	if isBool("maybe") {
		t.Fatal("isBool(maybe) = true, want false")
	}
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "testdata", "weatherconfig"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

func TestValidateDataFromFileBytes(t *testing.T) {
	data, err := os.ReadFile(fixturePath(t, "valid-modern", "cfgweather.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := ValidateData(data, "fixture.xml"); err != nil {
		t.Fatalf("ValidateData returned error: %v", err)
	}
}
