package update

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/internal/economy"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "types <type-name>" {
		t.Fatalf("Use = %q, want update usage", command.Use)
	}
	if command.Short == "" {
		t.Fatal("Short is empty")
	}
}

func TestNewCommandDryRunPrintsModifiedXML(t *testing.T) {
	path := writeTypesFile(t, `<types><type name="ACOGOptic"><nominal>1</nominal><flags count_in_map="0" /><category name="old" /><tag name="floor" /><usage name="Tenement" /><value name="Tier1" /></type></types>`)
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{
		"ACOGOptic",
		"--file", path,
		"--dry-run",
		"--rename", "ACOGOpticFixed",
		"--occurrence", "1",
		"--nominal", "2",
		"--lifetime", "3",
		"--restock", "4",
		"--min", "1",
		"--quantmin", "5",
		"--quantmax", "90",
		"--cost", "6",
		"--flag", "count_in_map=1",
		"--remove-flag", "crafted",
		"--remove-flags",
		"--set-category", "tools",
		"--add-category", "weapons",
		"--remove-category", "old",
		"--clear-tags",
		"--add-tag", "fishing",
		"--remove-usage", "Tenement",
		"--add-usage", "Town",
		"--set-value", "Tier2",
	})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	output := stdout.String()
	assertContains(t, output, `<type name="ACOGOpticFixed">`)
	assertContains(t, output, `<nominal>2</nominal>`)
	assertContains(t, output, `<category name="tools" />`)
	assertContains(t, output, `<category name="weapons" />`)
	assertContains(t, output, `<tag name="fishing" />`)
	assertContains(t, output, `<usage name="Town" />`)
	assertContains(t, output, `<value name="Tier2" />`)
	assertContains(t, readFile(t, path), `<type name="ACOGOptic">`)
}

func TestNewCommandWritesInPlace(t *testing.T) {
	path := writeTypesFile(t, `<types><type name="Apple"><nominal>1</nominal></type></types>`)
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"Apple", "--file", path, "--nominal", "2"})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "types "+path+" ok")
	assertContains(t, readFile(t, path), `<nominal>2</nominal>`)
}

func TestNewCommandReturnsOptionError(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"Apple", "--file", writeTypesFile(t, `<types><type name="Apple" /></types>`), "--flag", "broken"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want flag expression error")
	}
	assertContains(t, err.Error(), "name=0|1")
}

func TestNewCommandReturnsMutationError(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"Apple", "--file", filepath.Join(t.TempDir(), "missing.xml")})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want mutation error")
	}
	assertContains(t, err.Error(), "stat")
}

func TestNewCommandReturnsWriteError(t *testing.T) {
	original := writeFileMutation
	path := writeTypesFile(t, `<types><type name="Apple"><nominal>1</nominal></type></types>`)
	writeFileMutation = func(string, economy.FileMutation) error {
		return errors.New("write failed")
	}
	defer func() { writeFileMutation = original }()

	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"Apple", "--file", path, "--nominal", "2"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want write error")
	}
	assertContains(t, err.Error(), "write failed")
}

func TestSplitFlagExpressionRejectsMissingValue(t *testing.T) {
	_, _, err := splitFlagExpression("count_in_map=")
	if err == nil {
		t.Fatal("err = nil, want missing value error")
	}
}

func TestNewCommandDiscoversTypesFileFromEconomyCore(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"ModdedItem", "--cfgeconomycore", fixturePath(t, "mission", "cfgeconomycore.xml"), "--nominal", "2", "--dry-run"})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), `<type name="ModdedItem">`)
	assertContains(t, stdout.String(), `<nominal>2</nominal>`)
}

func TestResolveTypesFileErrors(t *testing.T) {
	if _, err := resolveTypesFile("types.xml", "cfgeconomycore.xml", "Apple"); err == nil {
		t.Fatal("resolveTypesFile both flags err = nil")
	}
	if _, err := resolveTypesFile("", "", "Apple"); err == nil {
		t.Fatal("resolveTypesFile missing flags err = nil")
	}

	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"Apple"})
	if err := command.Execute(); err == nil {
		t.Fatal("command missing file err = nil")
	}
}

func writeTypesFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "types.xml")
	if err := os.WriteFile(path, []byte(`<?xml version="1.0" encoding="UTF-8"?>`+body), 0o600); err != nil {
		t.Fatalf("write types file: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
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
