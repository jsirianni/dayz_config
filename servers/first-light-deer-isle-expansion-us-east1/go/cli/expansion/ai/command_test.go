package ai

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "ai" {
		t.Fatalf("Use = %q, want ai", command.Use)
	}
	if len(command.Commands()) != 3 {
		t.Fatalf("subcommand count = %d, want 3", len(command.Commands()))
	}
	uses := strings.Join([]string{command.Commands()[0].Use, command.Commands()[1].Use, command.Commands()[2].Use}, ",")
	for _, want := range []string{"loadouts", "patrols", "validate [path]"} {
		if !strings.Contains(uses, want) {
			t.Fatalf("subcommands %q does not contain %q", uses, want)
		}
	}
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
	if !strings.Contains(stdout.String(), "patrols") {
		t.Fatalf("%q does not contain patrols", stdout.String())
	}
	if !strings.Contains(stdout.String(), "loadouts") {
		t.Fatalf("%q does not contain loadouts", stdout.String())
	}
}
