package types

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	if command.Use != "types" {
		t.Fatalf("Use = %q, want types", command.Use)
	}
	if len(command.Commands()) != 1 {
		t.Fatalf("subcommand count = %d, want 1", len(command.Commands()))
	}
	if command.Commands()[0].Use != "update <types.xml> <type-name>" {
		t.Fatalf("update Use = %q", command.Commands()[0].Use)
	}
}

func TestNewCommandWithoutSubcommandShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)

	err := command.RunE(command, nil)

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "update") {
		t.Fatalf("%q does not contain update", stdout.String())
	}
}
