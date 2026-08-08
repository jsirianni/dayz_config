package gameplay

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
	"dzcli/internal/gameplayconfig"
)

func TestCommandMetadata(t *testing.T) {
	tests := []struct {
		name string
		use  string
		cmd  func(*bytes.Buffer) any
	}{
		{name: "get", use: "gameplay [field]", cmd: func(stdout *bytes.Buffer) any { return NewGetCommand(stdout) }},
		{name: "validate", use: "gameplay <cfggameplay.json>", cmd: func(stdout *bytes.Buffer) any { return NewValidateCommand(stdout) }},
		{name: "update", use: "gameplay <field>", cmd: func(stdout *bytes.Buffer) any { return NewUpdateCommand(strings.NewReader(""), stdout) }},
		{name: "delete", use: "gameplay <field>", cmd: func(stdout *bytes.Buffer) any { return NewDeleteCommand(stdout) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			command := test.cmd(stdout).(interface {
				UseLine() string
				Help() error
			})
			assertContains(t, command.UseLine(), test.use)
			if err := command.Help(); err != nil {
				t.Fatalf("Help returned error: %v", err)
			}
			assertContains(t, stdout.String(), test.use)
		})
	}
}

func TestValidateGameplayCommand(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "valid.json")})
	if err := command.Execute(); err != nil {
		t.Fatalf("validate command returned error: %v", err)
	}
	assertContains(t, stdout.String(), "gameplay")
	assertContains(t, stdout.String(), "ok")

	stdout.Reset()
	err := ValidateGameplay(fixturePath(t, "unknown.json"), &stdout)
	if !errors.Is(err, validation.ErrFailed) {
		t.Fatalf("ValidateGameplay err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), "unknown field")
}

func TestGetGameplayCommand(t *testing.T) {
	var stdout bytes.Buffer
	command := NewGetCommand(&stdout)
	command.SetArgs([]string{"--file", fixturePath(t, "valid.json")})
	if err := command.Execute(); err != nil {
		t.Fatalf("get Execute returned error: %v", err)
	}
	assertContains(t, firstLine(stdout.String()), "FIELD")
	assertContains(t, firstLine(stdout.String()), "VALUE")
	assertContains(t, stdout.String(), "PlayerData.StaminaData.staminaMax")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	command.SetArgs([]string{"PlayerData.StaminaData.staminaMax", "--file", fixturePath(t, "valid.json")})
	if err := command.Execute(); err != nil {
		t.Fatalf("filtered get Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "100.0")
	assertNotContains(t, stdout.String(), "disableBaseDamage")
}

func TestListGameplayErrors(t *testing.T) {
	var stdout bytes.Buffer
	if err := ListGameplay("", "", &stdout); err == nil {
		t.Fatal("ListGameplay missing file err = nil")
	}
	if err := ListGameplay(writeTempGameplay(t, `{`), "", &stdout); err == nil {
		t.Fatal("ListGameplay parse err = nil")
	}
	if err := ListGameplay(writeTempGameplay(t, `{`), "version", &stdout); err == nil {
		t.Fatal("ListGameplay filtered parse err = nil")
	}
	if err := ListGameplay(writeTempGameplay(t, `{}`), "", &stdout); err == nil {
		t.Fatal("ListGameplay empty err = nil")
	}
	if err := ListGameplay(fixturePath(t, "valid.json"), "missing", &stdout); err == nil {
		t.Fatal("ListGameplay missing field err = nil")
	}
}

func TestUpdateGameplayPromptYesNoForceDryRunAndClear(t *testing.T) {
	path := writeTempGameplay(t, `{"version":123,"PlayerData":{"spawnGearPresetFiles":["old"]}}`)

	var stdout bytes.Buffer
	command := NewUpdateCommand(strings.NewReader("n\n"), &stdout)
	command.SetArgs([]string{"version", "--file", path, "--value", "124"})
	if err := command.Execute(); err != nil {
		t.Fatalf("no Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "Overwrite version")
	assertContains(t, stdout.String(), "not modified")
	assertFileContains(t, path, `"version":123`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader("yes"), &stdout)
	command.SetArgs([]string{"version", "--file", path, "--value", "124"})
	if err := command.Execute(); err != nil {
		t.Fatalf("yes Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
	assertFileContains(t, path, `"version": 124`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"version", "--file", path, "--value", "125", "--force"})
	if err := command.Execute(); err != nil {
		t.Fatalf("force Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), "Overwrite")
	assertFileContains(t, path, `"version": 125`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"version", "--file", path, "--value", "126", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("dry-run Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), `"version": 126`)
	assertFileContains(t, path, `"version": 125`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"PlayerData.spawnGearPresetFiles", "--file", path, "--clear", "--force"})
	if err := command.Execute(); err != nil {
		t.Fatalf("clear Execute returned error: %v", err)
	}
	assertFileContains(t, path, `"spawnGearPresetFiles": []`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"UIData.use3DMap", "--file", path, "--value", "true"})
	if err := command.Execute(); err != nil {
		t.Fatalf("insert Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), "Overwrite")
	assertFileContains(t, path, `"use3DMap": true`)
}

func TestUpdateGameplayErrors(t *testing.T) {
	var stdout bytes.Buffer
	command := NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"version", "--value", "1"})
	if err := command.Execute(); err == nil {
		t.Fatal("missing file err = nil")
	}

	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"version", "--file", writeTempGameplay(t, `{`), "--value", "1"})
	if err := command.Execute(); err == nil {
		t.Fatal("mutation err = nil")
	}

	command = NewUpdateCommand(errReader{}, &stdout)
	command.SetArgs([]string{"version", "--file", writeTempGameplay(t, `{"version":1}`), "--value", "2"})
	if err := command.Execute(); err == nil {
		t.Fatal("prompt err = nil")
	}

	if _, err := confirmOverwrite(errReader{}, &stdout, "version", "cfggameplay.json"); err == nil {
		t.Fatal("confirmOverwrite err = nil")
	}
}

func TestDeleteGameplayCommand(t *testing.T) {
	path := writeTempGameplay(t, `{"version":123,"Unknown":{"leaf":true}}`)

	var stdout bytes.Buffer
	command := NewDeleteCommand(&stdout)
	command.SetArgs([]string{"Unknown.leaf", "--file", path, "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("dry-run delete Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), `"leaf"`)
	assertFileContains(t, path, `"leaf"`)

	stdout.Reset()
	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"Unknown.leaf", "--file", path})
	if err := command.Execute(); err != nil {
		t.Fatalf("delete Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
	data, _ := os.ReadFile(path)
	assertNotContains(t, string(data), `"leaf"`)

	stdout.Reset()
	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"missing", "--file", path})
	if err := command.Execute(); err != nil {
		t.Fatalf("missing delete Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
}

func TestDeleteGameplayErrorsAndOutputMutation(t *testing.T) {
	var stdout bytes.Buffer
	command := NewDeleteCommand(&stdout)
	command.SetArgs([]string{"version"})
	if err := command.Execute(); err == nil {
		t.Fatal("missing file err = nil")
	}

	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"version", "--file", writeTempGameplay(t, `{`)})
	if err := command.Execute(); err == nil {
		t.Fatal("mutation err = nil")
	}

	originalWrite := writeFileMutation
	defer func() { writeFileMutation = originalWrite }()
	writeFileMutation = func(string, gameplayconfig.FileMutation) error {
		return errors.New("write failed")
	}
	if err := outputMutation("cfggameplay.json", gameplayconfig.FileMutation{Data: []byte("{}"), Changed: true}, false, &stdout); err == nil {
		t.Fatal("outputMutation err = nil")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "gameplayconfig", name))
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

func firstLine(output string) string {
	line, _, _ := strings.Cut(output, "\n")
	return line
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
