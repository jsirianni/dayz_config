package economy

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "economy" {
		t.Fatalf("Use = %q, want economy", command.Use)
	}
	if len(command.Commands()) != 3 {
		t.Fatalf("subcommand count = %d, want 3", len(command.Commands()))
	}
	uses := []string{command.Commands()[0].Use, command.Commands()[1].Use, command.Commands()[2].Use}
	assertContains(t, strings.Join(uses, ","), "limits")
	assertContains(t, strings.Join(uses, ","), "types")
	assertContains(t, strings.Join(uses, ","), "validate <cfgeconomycore.xml>")
}

func TestNewCommandWithoutSubcommandShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)

	err := command.RunE(command, nil)

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "validate") {
		t.Fatalf("%q does not contain validate", stdout.String())
	}
	if !strings.Contains(stdout.String(), "types") {
		t.Fatalf("%q does not contain types", stdout.String())
	}
	if !strings.Contains(stdout.String(), "limits") {
		t.Fatalf("%q does not contain limits", stdout.String())
	}
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}
