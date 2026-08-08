package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "validate [path]" {
		t.Fatalf("Use = %q, want validate [path]", command.Use)
	}
	if command.Short != "Validate DayZ Expansion AI patrol and loadout files" {
		t.Fatalf("Short = %q", command.Short)
	}
}

func TestNewCommandExecutesValidationWithExplicitPath(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "expansionai", "valid")})

	err := command.Execute()

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	assertContains(t, stdout.String(), "expansion-ai-patrols")
	assertContains(t, stdout.String(), "AIPatrolSettings.json ok (1 patrols)")
	assertContains(t, stdout.String(), "TestLoadout.json ok (4 items)")
}

func TestNewCommandExecutesValidationWithDefaultPath(t *testing.T) {
	t.Chdir(fixturePath(t, "expansionai", "valid"))
	var stdout bytes.Buffer
	command := NewCommand(&stdout)

	err := command.Execute()

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	assertContains(t, stdout.String(), "AISettings.json ok")
}

func TestNewCommandRejectsTooManyArguments(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"one", "two"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want error")
	}
	assertContains(t, err.Error(), "accepts at most 1 arg")
}

func TestValidateAIPathReportsDiscoveryFailure(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateAIPath(filepath.Join(t.TempDir(), "missing"), &stdout)

	if err == nil {
		t.Fatal("err = nil, want error")
	}
	assertContains(t, err.Error(), "expansion ai: failed")
}

func TestValidateAIPathReportsFileFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AISettings.json")
	if err := writeFile(path, `{"m_Version":20,"Unknown":true}`); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var stdout bytes.Buffer

	err := ValidateAIPath(path, &stdout)

	if err == nil {
		t.Fatal("err = nil, want validation failure")
	}
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "unknown field")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	args := append([]string{filepath.Dir(file), "..", "..", "..", "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func writeFile(path string, content string) error {
	return osWriteFile(path, []byte(content))
}

var osWriteFile = func(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}
