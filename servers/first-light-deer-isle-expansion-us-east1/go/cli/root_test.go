package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMainExitsOnFailure(t *testing.T) {
	var exitCode int
	Main([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}, func(code int) {
		exitCode = code
	})

	if exitCode != FailureExitCode {
		t.Fatalf("exit code = %d, want %d", exitCode, FailureExitCode)
	}
}

func TestRunWithoutCommandShowsRootHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "Tools for DayZ server configuration")
	assertContains(t, stdout.String(), "Available Commands:")
	assertContains(t, stdout.String(), "economy")
	assertContains(t, stdout.String(), "xml")
}

func TestRunReturnsUnknownCommandFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"unknown"}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "unknown command")
}

func TestRunReturnsMissingValidateArgumentFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"economy", "validate"}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "accepts 1 arg(s), received 0")
}

func TestRunReportsEconomyCoreParseFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"economy", "validate", fixturePath(t, "mission", "mods", "valid_types.xml")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "expected <economycore> root")
}

func TestRunReportsEachTypesFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"economy", "validate", fixturePath(t, "mission", "cfgeconomycore.xml")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "cfgeconomycore")
	assertContains(t, stdout.String(), "cfgeconomycore.xml ok")
	assertContains(t, stdout.String(), "base-types")
	assertContains(t, stdout.String(), filepath.Join("db", "types.xml")+" ok (2 types)")
	assertContains(t, stdout.String(), filepath.Join("mods", "valid_types.xml")+" ok (1 types)")
	assertNotContains(t, stdout.String(), "spawnabletypes")
}

func TestRunReturnsFailureWhenAReferencedTypesFileFails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"economy", "validate", fixturePath(t, "mission", "cfgeconomycore_bad_ref.xml")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), filepath.Join("badmods", "invalid_types.badxml")+" failed")
	assertContains(t, stdout.String(), "XML syntax error")
}

func TestRunReportsEachGenericXMLFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"xml", "validate", fixturePath(t, "xml", "valid")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "xml "+filepath.Join(fixturePath(t, "xml", "valid"), "nested", "second.XML")+" ok")
	assertContains(t, stdout.String(), "xml "+filepath.Join(fixturePath(t, "xml", "valid"), "root.xml")+" ok")
	assertNotContains(t, stdout.String(), "ignored.txt")
}

func TestRunXMLValidateReturnsFailureForInvalidXML(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := mixedXMLDir(t)

	code := Run([]string{"xml", "validate", dir}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "bad.xml failed")
	assertContains(t, stdout.String(), "good.xml ok")
}

func TestNewRootCommandMetadata(t *testing.T) {
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})

	assertEqual(t, command.Use, "dzcli")
	if len(command.Commands()) != 2 {
		t.Fatalf("root command count = %d, want 2", len(command.Commands()))
	}
	uses := []string{command.Commands()[0].Use, command.Commands()[1].Use}
	assertContains(t, strings.Join(uses, ","), "economy")
	assertContains(t, strings.Join(uses, ","), "xml")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	args := append([]string{filepath.Dir(file), "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func mixedXMLDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "bad.xml"), `<?xml version="1.0" encoding="UTF-8"?><bad>`)
	writeTestFile(t, filepath.Join(dir, "good.xml"), `<?xml version="1.0" encoding="UTF-8"?><good />`)
	return dir
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := osWriteFile(path, []byte(content)); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
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
