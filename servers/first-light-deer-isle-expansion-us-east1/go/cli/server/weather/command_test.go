package weather

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
)

func TestNewValidateCommandMetadataAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)

	if command.Use != "weather <cfgweather.xml>" {
		t.Fatalf("Use = %q, want weather <cfgweather.xml>", command.Use)
	}
	if command.Short != "Validate cfgweather.xml" {
		t.Fatalf("Short = %q, want Validate cfgweather.xml", command.Short)
	}
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "Validate cfgweather.xml")
}

func TestValidateWeather(t *testing.T) {
	var stdout bytes.Buffer
	valid := fixturePath(t, "valid-modern", "cfgweather.xml")

	err := ValidateWeather(valid, &stdout)

	if err != nil {
		t.Fatalf("ValidateWeather returned error: %v", err)
	}
	assertContains(t, stdout.String(), "weather "+valid+" ok")

	stdout.Reset()
	invalid := fixturePath(t, "invalid-range", "cfgweather.xml")
	err = ValidateWeather(invalid, &stdout)

	if !errors.Is(err, validation.ErrFailed) {
		t.Fatalf("ValidateWeather error = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "weather "+invalid+" failed")
	assertContains(t, stdout.String(), "less than or equal")
}

func TestValidateCommandExecute(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "valid-legacy", "cfgweather.xml")})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
}

func TestValidateCommandMissingArgument(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)

	if err := command.Execute(); err == nil {
		t.Fatal("Execute missing argument error = nil")
	}
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "..", "testdata", "weatherconfig"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}
