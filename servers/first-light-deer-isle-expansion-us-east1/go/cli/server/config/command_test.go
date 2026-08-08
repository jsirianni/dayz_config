package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
	"dzcli/internal/serverconfig"
)

func TestCommandMetadata(t *testing.T) {
	tests := []struct {
		name string
		use  string
		cmd  func(*bytes.Buffer) any
	}{
		{name: "get", use: "server [field]", cmd: func(stdout *bytes.Buffer) any { return NewGetCommand(stdout) }},
		{name: "validate", use: "server <serverDZ.cfg>", cmd: func(stdout *bytes.Buffer) any { return NewValidateCommand(stdout) }},
		{name: "update", use: "server <field>", cmd: func(stdout *bytes.Buffer) any { return NewUpdateCommand(strings.NewReader(""), stdout) }},
		{name: "delete", use: "server <field>", cmd: func(stdout *bytes.Buffer) any { return NewDeleteCommand(stdout) }},
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

func TestValidateConfig(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "valid.cfg")})
	if err := command.Execute(); err != nil {
		t.Fatalf("validate command returned error: %v", err)
	}
	assertContains(t, stdout.String(), "server ")
	assertContains(t, stdout.String(), "ok")

	stdout.Reset()
	err := ValidateConfig(fixturePath(t, "valid.cfg"), &stdout)
	if err != nil {
		t.Fatalf("ValidateConfig returned error: %v", err)
	}
	assertContains(t, stdout.String(), "server ")
	assertContains(t, stdout.String(), "ok")

	stdout.Reset()
	err = ValidateConfig(writeTempConfig(t, "custom = 1;"), &stdout)
	if !errors.Is(err, validation.ErrFailed) {
		t.Fatalf("ValidateConfig err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
	assertContains(t, stdout.String(), `unknown field "custom"`)

	stdout.Reset()
	err = ValidateConfig(writeTempConfig(t, "custom = 1; // mod option\n"), &stdout)
	if err != nil {
		t.Fatalf("ValidateConfig annotated custom returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
}

func TestGetConfigCommand(t *testing.T) {
	var stdout bytes.Buffer
	command := NewGetCommand(&stdout)
	command.SetArgs([]string{"--file", fixturePath(t, "valid.cfg")})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, firstLine(stdout.String()), "FIELD")
	assertContains(t, firstLine(stdout.String()), "VALUE")
	assertContains(t, stdout.String(), "hostname")
	assertContains(t, stdout.String(), "Fixture Server")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	command.SetArgs([]string{"motd", "--file", fixturePath(t, "valid.cfg")})
	if err := command.Execute(); err != nil {
		t.Fatalf("filtered Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "line1,line2")
	assertNotContains(t, stdout.String(), "hostname")
}

func TestListConfigErrors(t *testing.T) {
	var stdout bytes.Buffer
	if err := ListConfig("", "", &stdout); err == nil {
		t.Fatal("ListConfig missing file err = nil")
	}
	if err := ListConfig(writeTempConfig(t, "{"), "", &stdout); err == nil {
		t.Fatal("ListConfig parse err = nil")
	}
	if err := ListConfig(writeTempConfig(t, ""), "", &stdout); err == nil {
		t.Fatal("ListConfig empty err = nil")
	}
	if err := ListConfig(fixturePath(t, "valid.cfg"), "missing", &stdout); err == nil {
		t.Fatal("ListConfig missing field err = nil")
	}
}

func TestUpdateConfigPromptYesNoForceAndDryRun(t *testing.T) {
	path := writeTempConfig(t, `hostname = "Old";`)

	var stdout bytes.Buffer
	command := NewUpdateCommand(strings.NewReader("n\n"), &stdout)
	command.SetArgs([]string{"hostname", "--file", path, "--value", "Nope"})
	if err := command.Execute(); err != nil {
		t.Fatalf("no Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "Overwrite hostname")
	assertContains(t, stdout.String(), "not modified")
	assertFileContains(t, path, `hostname = "Old";`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader("yes"), &stdout)
	command.SetArgs([]string{"hostname", "--file", path, "--value", "Yes"})
	if err := command.Execute(); err != nil {
		t.Fatalf("yes Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
	assertFileContains(t, path, `hostname = "Yes";`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"hostname", "--file", path, "--value", "Forced", "--force"})
	if err := command.Execute(); err != nil {
		t.Fatalf("force Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), "Overwrite")
	assertFileContains(t, path, `hostname = "Forced";`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"hostname", "--file", path, "--value", "Dry", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("dry-run Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), `hostname = "Dry";`)
	assertFileContains(t, path, `hostname = "Forced";`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"description", "--file", path, "--value", "Created"})
	if err := command.Execute(); err != nil {
		t.Fatalf("new field Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), "Overwrite")
	assertFileContains(t, path, `description = "Created";`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"vppDisablePassword", "--file", path, "--value", "1", "--allow-unknown"})
	if err := command.Execute(); err != nil {
		t.Fatalf("unknown insert Execute returned error: %v", err)
	}
	assertFileContains(t, path, `vppDisablePassword = 1; // #dzcli:allow-unknown`)

	stdout.Reset()
	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"vppDisablePassword", "--file", path, "--value", "0", "--force"})
	if err := command.Execute(); err != nil {
		t.Fatalf("existing unknown Execute returned error: %v", err)
	}
	assertFileContains(t, path, `vppDisablePassword = 0; // #dzcli:allow-unknown`)
}

func TestUpdateConfigErrors(t *testing.T) {
	var stdout bytes.Buffer
	command := NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"hostname", "--value", "x"})
	if err := command.Execute(); err == nil {
		t.Fatal("missing file err = nil")
	}

	command = NewUpdateCommand(strings.NewReader(""), &stdout)
	command.SetArgs([]string{"verifySignatures", "--file", writeTempConfig(t, ""), "--value", "1"})
	if err := command.Execute(); err == nil {
		t.Fatal("invalid value err = nil")
	}

	command = NewUpdateCommand(errReader{}, &stdout)
	command.SetArgs([]string{"hostname", "--file", writeTempConfig(t, `hostname = "Old";`), "--value", "New"})
	if err := command.Execute(); err == nil {
		t.Fatal("prompt err = nil")
	}

	if _, err := confirmOverwrite(errReader{}, &stdout, "hostname", "serverDZ.cfg"); err == nil {
		t.Fatal("confirmOverwrite err = nil")
	}
}

func TestDeleteConfigCommand(t *testing.T) {
	path := writeTempConfig(t, "hostname = \"Server\";\nvppDisablePassword = 1;\n")

	var stdout bytes.Buffer
	command := NewDeleteCommand(&stdout)
	command.SetArgs([]string{"vppDisablePassword", "--file", path, "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("dry-run Execute returned error: %v", err)
	}
	assertNotContains(t, stdout.String(), "vppDisablePassword")
	assertFileContains(t, path, "vppDisablePassword")

	stdout.Reset()
	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"vppDisablePassword", "--file", path})
	if err := command.Execute(); err != nil {
		t.Fatalf("delete Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
	data, _ := os.ReadFile(path)
	assertNotContains(t, string(data), "vppDisablePassword")

	stdout.Reset()
	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"missing", "--file", path})
	if err := command.Execute(); err != nil {
		t.Fatalf("missing delete Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ok")
}

func TestDeleteConfigErrorsAndOutputMutation(t *testing.T) {
	var stdout bytes.Buffer
	command := NewDeleteCommand(&stdout)
	command.SetArgs([]string{"hostname"})
	if err := command.Execute(); err == nil {
		t.Fatal("missing file err = nil")
	}

	command = NewDeleteCommand(&stdout)
	command.SetArgs([]string{"hostname", "--file", writeTempConfig(t, "{")})
	if err := command.Execute(); err == nil {
		t.Fatal("parse err = nil")
	}

	originalWrite := writeFileMutation
	defer func() { writeFileMutation = originalWrite }()
	writeFileMutation = func(string, serverconfig.FileMutation) error {
		return errors.New("write failed")
	}
	if err := outputMutation("serverDZ.cfg", serverconfig.FileMutation{Data: []byte("x"), Changed: true}, false, &stdout); err == nil {
		t.Fatal("outputMutation err = nil")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "..", "testdata", "serverconfig"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serverDZ.cfg")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
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
