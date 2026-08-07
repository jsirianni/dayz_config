package limits

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dzcli/internal/economy"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "limits" {
		t.Fatalf("Use = %q, want limits", command.Use)
	}
	if len(command.Commands()) != 3 {
		t.Fatalf("subcommand count = %d, want 3", len(command.Commands()))
	}
	uses := []string{command.Commands()[0].Use, command.Commands()[1].Use, command.Commands()[2].Use}
	assertContains(t, strings.Join(uses, ","), "add <category|tag|usage|value> <name>")
	assertContains(t, strings.Join(uses, ","), "remove <category|tag|usage|value> <name>")
	assertContains(t, strings.Join(uses, ","), "group")
}

func TestParentCommandsShowHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	if err := command.RunE(command, nil); err != nil {
		t.Fatalf("limits help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "group")

	stdout.Reset()
	group := newGroupCommand(&stdout)
	if err := group.RunE(group, nil); err != nil {
		t.Fatalf("group help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "member")

	stdout.Reset()
	member := newMemberCommand(&stdout)
	if err := member.RunE(member, nil); err != nil {
		t.Fatalf("member help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "remove")
}

func TestBaseAddDryRunPrintsModifiedXML(t *testing.T) {
	path := writeLimitsFile(t)
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"add", "tag", "fishing", "--file", path, "--dry-run"})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), `<tag name="fishing" />`)
	assertNotContains(t, readFile(t, path), "fishing")
}

func TestBaseRemoveWritesInPlace(t *testing.T) {
	path := writeLimitsFile(t)
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"remove", "tag", "floor", "--file", path})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "limits "+path+" ok")
	assertNotContains(t, readFile(t, path), `<tag name="floor" />`)
}

func TestBaseCommandRequiresFile(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"add", "tag", "fishing"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want missing file error")
	}
	assertContains(t, err.Error(), "--file is required")
}

func TestBaseCommandReturnsMutationError(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"add", "tag", "fishing", "--file", filepath.Join(t.TempDir(), "missing.xml")})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want mutation error")
	}
	assertContains(t, err.Error(), "stat")
}

func TestGroupCommandsMutateUserLimits(t *testing.T) {
	path := writeUserLimitsFile(t)

	runLimitsCommand(t, []string{"group", "add", "usage", "Tenement", "--file", path, "--member", "Town", "--member", "Village"})
	assertContains(t, readFile(t, path), `<user name="Tenement">`)
	assertContains(t, readFile(t, path), `<usage name="Village" />`)

	runLimitsCommand(t, []string{"group", "member", "add", "usage", "Tenement", "Coast", "--file", path})
	assertContains(t, readFile(t, path), `<usage name="Coast" />`)

	runLimitsCommand(t, []string{"group", "member", "remove", "usage", "Tenement", "Coast", "--file", path})
	assertNotContains(t, readFile(t, path), `<usage name="Coast" />`)

	runLimitsCommand(t, []string{"group", "remove", "usage", "Tenement", "--file", path})
	assertNotContains(t, readFile(t, path), `<user name="Tenement">`)
}

func TestGroupAddDryRunPrintsModifiedXML(t *testing.T) {
	path := writeUserLimitsFile(t)
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{"group", "add", "value", "TierGroup", "--file", path, "--member", "Tier1", "--dry-run"})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), `<user name="TierGroup">`)
	assertNotContains(t, readFile(t, path), "TierGroup")
}

func TestGroupCommandRequiresFile(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"group", "add", "usage", "Tenement"})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want missing file error")
	}
	assertContains(t, err.Error(), "--file is required")
}

func TestGroupCommandReturnsMutationError(t *testing.T) {
	path := writeUserLimitsFile(t)
	command := NewCommand(&bytes.Buffer{})
	command.SetArgs([]string{"group", "member", "add", "usage", "Missing", "Town", "--file", path})

	err := command.Execute()

	if err == nil {
		t.Fatal("err = nil, want mutation error")
	}
	assertContains(t, err.Error(), "not found")
}

func TestOutputMutationReturnsWriteError(t *testing.T) {
	original := writeFileMutation
	writeFileMutation = func(string, economy.FileMutation) error {
		return errors.New("write failed")
	}
	defer func() { writeFileMutation = original }()

	err := outputMutation("file.xml", "limits", economy.FileMutation{Changed: true}, false, &bytes.Buffer{})

	if err == nil {
		t.Fatal("err = nil, want write error")
	}
	assertContains(t, err.Error(), "write failed")
}

func runLimitsCommand(t *testing.T, args []string) {
	t.Helper()
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute(%v) returned error: %v", args, err)
	}
	assertContains(t, stdout.String(), "ok")
}

func writeLimitsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfglimitsdefinition.xml")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<lists>
  <categories>
    <category name="tools" />
  </categories>
  <tags>
    <tag name="floor" />
  </tags>
  <usageflags>
    <usage name="Town" />
  </usageflags>
  <valueflags>
    <value name="Tier1" />
  </valueflags>
</lists>`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write limits file: %v", err)
	}
	return path
}

func writeUserLimitsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfglimitsdefinitionuser.xml")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<user_lists>
  <usageflags>
    <user name="TownVillage">
      <usage name="Town" />
    </user>
  </usageflags>
  <valueflags>
  </valueflags>
</user_lists>`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write user limits file: %v", err)
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

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%q contains %q", haystack, needle)
	}
}
