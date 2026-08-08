package validate

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
	"dzcli/internal/economyconfig"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	assertEqual(t, command.Use, "economy <mission-root|cfgeconomycore.xml|economy-file>")
	assertEqual(t, command.Short, "Validate central economy files")
}

func TestNewCommandExecutesAggregateValidation(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "mission")})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "cfgeconomycore.xml ok")
	assertContains(t, stdout.String(), "cfglimitsdefinition.xml ok")
	assertContains(t, stdout.String(), filepath.Join("db", "types.xml")+" ok (2 types)")
}

func TestValidateEconomyReturnsConfigError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomy(fixturePath(t, "mission", "mods", "valid_types.xml"), &stdout)

	if err == nil {
		t.Fatal("err = nil, want config error")
	}
	assertContains(t, err.Error(), "economy: failed")
}

func TestValidateEconomyReturnsValidationError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomy(fixturePath(t, "mission", "cfgeconomycore_bad_ref.xml"), &stdout)

	if err != validation.ErrFailed {
		t.Fatalf("err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
}

func TestValidateEconomyCoreWrapper(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomyCore(fixturePath(t, "mission", "cfgeconomycore.xml"), &stdout)

	if err != nil {
		t.Fatalf("ValidateEconomyCore returned error: %v", err)
	}
	assertContains(t, stdout.String(), "cfgeconomycore.xml ok")
}

func TestValidateEconomyCoreWrapperReturnsConfigError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomyCore(fixturePath(t, "mission", "missing.xml"), &stdout)

	if err == nil {
		t.Fatal("err = nil, want config error")
	}
	assertContains(t, err.Error(), "cfgeconomycore: failed")
}

func TestPrintWarningsWritesEachWarning(t *testing.T) {
	var stdout bytes.Buffer

	printWarnings(&stdout, economyconfig.FileStatus{
		Kind:     "types",
		Path:     "types.xml",
		Warnings: []string{"first", "second"},
	})

	assertContains(t, stdout.String(), "types types.xml warning: first")
	assertContains(t, stdout.String(), "types types.xml warning: second")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	args := append([]string{filepath.Dir(file), "..", "..", "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertEqual[T comparable](t *testing.T, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
