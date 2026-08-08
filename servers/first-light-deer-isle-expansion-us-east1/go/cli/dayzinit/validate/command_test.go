package validate

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
)

func TestNewCommandMetadataAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)

	if command.Use != "init <init.c>" {
		t.Fatalf("Use = %q, want init <init.c>", command.Use)
	}
	if command.Short != "Validate DayZ mission init.c" {
		t.Fatalf("Short = %q", command.Short)
	}
	if err := command.Help(); err != nil {
		t.Fatalf("Help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "init <init.c>")
}

func TestNewCommandExecutesValidation(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "valid", "init.c")})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "init ")
	assertContains(t, stdout.String(), " ok")

	stdout.Reset()
	command = NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "invalid", "init.c")})
	err := command.Execute()
	if !errors.Is(err, validation.ErrFailed) {
		t.Fatalf("Execute err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "DZI")
}

func TestNewCommandRejectsMissingArgument(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs(nil)

	if err := command.Execute(); err == nil {
		t.Fatal("Execute missing argument err = nil")
	}
}

func TestValidateInit(t *testing.T) {
	var stdout bytes.Buffer
	if err := ValidateInit(fixturePath(t, "valid", "init.c"), &stdout); err != nil {
		t.Fatalf("ValidateInit returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")

	stdout.Reset()
	err := ValidateInit(fixturePath(t, "invalid", "init.c"), &stdout)
	if !errors.Is(err, validation.ErrFailed) {
		t.Fatalf("ValidateInit err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "DZI")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	args := append([]string{filepath.Dir(file), "..", "..", "..", "testdata", "dayzinit"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}
