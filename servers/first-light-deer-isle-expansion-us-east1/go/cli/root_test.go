package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	assertContains(t, stdout.String(), "create")
	assertContains(t, stdout.String(), "delete")
	assertContains(t, stdout.String(), "get")
	assertContains(t, stdout.String(), "update")
	assertContains(t, stdout.String(), "validate")
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

	code := Run([]string{"validate", "economy"}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "accepts 1 arg(s), received 0")
}

func TestRunReportsEconomyCoreParseFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "economy", fixturePath(t, "mission", "mods", "valid_types.xml")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "expected <economycore> root")
}

func TestRunReportsEachTypesFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "economy", fixturePath(t, "mission", "cfgeconomycore.xml")}, &stdout, &stderr)

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

	code := Run([]string{"validate", "economy", fixturePath(t, "mission", "cfgeconomycore_bad_ref.xml")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), filepath.Join("badmods", "invalid_types.badxml")+" failed")
	assertContains(t, stdout.String(), "XML syntax error")
}

func TestRunReportsEachGenericXMLFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "xml", fixturePath(t, "xml", "valid")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "xml "+filepath.Join(fixturePath(t, "xml", "valid"), "nested", "second.XML")+" ok")
	assertContains(t, stdout.String(), "xml "+filepath.Join(fixturePath(t, "xml", "valid"), "root.xml")+" ok")
	assertNotContains(t, stdout.String(), "ignored.txt")
}

func TestRunReportsInitValidation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "init", fixturePath(t, "dayzinit", "valid", "init.c")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "init ")
	assertContains(t, stdout.String(), " ok")

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"validate", "init", fixturePath(t, "dayzinit", "invalid", "init.c")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "DZI")
}

func TestRunReportsGameplayValidation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "gameplay", fixturePath(t, "gameplayconfig", "valid.json")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "gameplay")
	assertContains(t, stdout.String(), " ok")

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"validate", "gameplay", fixturePath(t, "gameplayconfig", "unknown.json")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "unknown field")
}

func TestRunReportsWeatherValidation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate", "weather", fixturePath(t, "weatherconfig", "valid-modern", "cfgweather.xml")}, &stdout, &stderr)

	assertEqual(t, code, SuccessExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "weather")
	assertContains(t, stdout.String(), " ok")

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"validate", "weather", fixturePath(t, "weatherconfig", "invalid-range", "cfgweather.xml")}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "less than or equal")
}

func TestRunXMLValidateReturnsFailureForInvalidXML(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := mixedXMLDir(t)

	code := Run([]string{"validate", "xml", dir}, &stdout, &stderr)

	assertEqual(t, code, FailureExitCode)
	assertEqual(t, stderr.String(), "")
	assertContains(t, stdout.String(), "bad.xml failed")
	assertContains(t, stdout.String(), "good.xml ok")
}

func TestNewRootCommandMetadata(t *testing.T) {
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})

	assertEqual(t, command.Use, "dzcli")
	if len(command.Commands()) != 5 {
		t.Fatalf("root command count = %d, want 5", len(command.Commands()))
	}
	uses := commandUses(command.Commands())
	assertContains(t, uses, "create")
	assertContains(t, uses, "delete")
	assertContains(t, uses, "get")
	assertContains(t, uses, "update")
	assertContains(t, uses, "validate")
}

func TestOldNounFirstCommandsFail(t *testing.T) {
	for _, args := range [][]string{
		{"economy", "types", "update"},
		{"expansion", "ai", "validate"},
		{"xml", "validate"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		code := Run(args, &stdout, &stderr)

		assertEqual(t, code, FailureExitCode)
		assertEqual(t, stdout.String(), "")
		assertContains(t, stderr.String(), "unknown command")
	}
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

func commandUses(commands []*cobra.Command) string {
	uses := make([]string, 0, len(commands))
	for _, command := range commands {
		uses = append(uses, command.Use)
	}
	return strings.Join(uses, ",")
}
