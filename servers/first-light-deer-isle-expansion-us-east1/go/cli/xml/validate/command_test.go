package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	assertEqual(t, command.Use, "validate [path]")
	assertEqual(t, command.Short, "Validate XML files recursively")
}

func TestNewCommandExecutesValidationWithExplicitPath(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "xml", "valid")})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "root.xml ok")
}

func TestRunXMLValidateDefaultsToCurrentDirectory(t *testing.T) {
	t.Chdir(fixturePath(t, "xml", "valid"))
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "xml "+filepath.Join("nested", "second.XML")+" ok")
	assertContains(t, stdout.String(), "xml root.xml ok")
}

func TestNewCommandRejectsTooManyArguments(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"one", "two"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want arg error")
	}
	assertContains(t, err.Error(), "accepts at most 1 arg(s), received 2")
}

func TestValidateXMLPathReturnsNilWhenAllFilesParse(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateXMLPath(fixturePath(t, "xml", "valid"), &stdout)

	if err != nil {
		t.Fatalf("ValidateXMLPath returned error: %v", err)
	}
	assertContains(t, stdout.String(), "root.xml ok")
}

func TestValidateXMLPathReturnsDiscoveryError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateXMLPath(fixturePath(t, "xml", "missing"), &stdout)

	if err == nil {
		t.Fatal("err = nil, want discovery error")
	}
	assertContains(t, err.Error(), "xml: failed")
}

func TestValidateXMLPathReturnsValidationError(t *testing.T) {
	var stdout bytes.Buffer
	dir := mixedXMLDir(t)

	err := ValidateXMLPath(dir, &stdout)

	if err != validation.ErrFailed {
		t.Fatalf("err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
}

func TestInspectXMLPathFindsAndParsesGenericXMLFiles(t *testing.T) {
	statuses, err := InspectXMLPath(fixturePath(t, "xml", "valid"))
	if err != nil {
		t.Fatalf("InspectXMLPath returned error: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("status count = %d, want 2", len(statuses))
	}
	assertEqual(t, filepath.Base(statuses[0].Path), "second.XML")
	assertEqual(t, statuses[0].Kind, "xml")
	if statuses[0].Err != nil {
		t.Fatalf("first XML status err = %v, want nil", statuses[0].Err)
	}
	assertEqual(t, filepath.Base(statuses[1].Path), "root.xml")
	assertEqual(t, statuses[1].Kind, "xml")
	if statuses[1].Err != nil {
		t.Fatalf("second XML status err = %v, want nil", statuses[1].Err)
	}
}

func TestInspectXMLPathRecordsInvalidXML(t *testing.T) {
	statuses, err := InspectXMLPath(mixedXMLDir(t))
	if err != nil {
		t.Fatalf("InspectXMLPath returned error: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("status count = %d, want 2", len(statuses))
	}
	if statuses[0].Err == nil {
		t.Fatal("invalid XML status err = nil, want error")
	}
	assertContains(t, statuses[0].Err.Error(), "parse")
	if statuses[1].Err != nil {
		t.Fatalf("valid XML status err = %v, want nil", statuses[1].Err)
	}
}

func TestFindXMLFilesReturnsEmptyWhenDirectoryHasNoXML(t *testing.T) {
	files, err := FindXMLFiles(fixturePath(t, "xml", "empty"))
	if err != nil {
		t.Fatalf("FindXMLFiles returned error: %v", err)
	}
	assertEqual(t, len(files), 0)
}

func TestFindXMLFilesReturnsReadDirectoryError(t *testing.T) {
	_, err := FindXMLFiles(fixturePath(t, "xml", "missing"))
	if err == nil {
		t.Fatal("err = nil, want read directory error")
	}
	assertContains(t, err.Error(), "read directory")
}

func TestParseGenericXMLFileReportsReadFailure(t *testing.T) {
	err := ParseGenericXMLFile(fixturePath(t, "xml", "missing.xml"))
	if err == nil {
		t.Fatal("err = nil, want read error")
	}
	assertContains(t, err.Error(), "read")
}

func TestParseGenericXMLFileRejectsInvalidXML(t *testing.T) {
	dir := mixedXMLDir(t)

	err := ParseGenericXMLFile(filepath.Join(dir, "bad.xml"))
	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
	assertContains(t, err.Error(), "parse")
}

func TestParseGenericXMLFileAcceptsValidXML(t *testing.T) {
	err := ParseGenericXMLFile(fixturePath(t, "xml", "valid", "root.xml"))
	if err != nil {
		t.Fatalf("ParseGenericXMLFile returned error: %v", err)
	}
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

func mixedXMLDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "bad.xml"), `<?xml version="1.0" encoding="UTF-8"?><bad>`)
	writeTestFile(t, filepath.Join(dir, "good.xml"), `<?xml version="1.0" encoding="UTF-8"?><good />`)
	return dir
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
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
