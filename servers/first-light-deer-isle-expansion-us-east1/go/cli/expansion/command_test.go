package expansion

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "expansion" {
		t.Fatalf("Use = %q, want expansion", command.Use)
	}
	if len(command.Commands()) != 1 {
		t.Fatalf("subcommand count = %d, want 1", len(command.Commands()))
	}
	if command.Commands()[0].Use != "ai" {
		t.Fatalf("ai Use = %q", command.Commands()[0].Use)
	}
}

func TestNewCommandWithoutSubcommandShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)

	err := command.RunE(command, nil)

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "ai") {
		t.Fatalf("%q does not contain ai", stdout.String())
	}
}
