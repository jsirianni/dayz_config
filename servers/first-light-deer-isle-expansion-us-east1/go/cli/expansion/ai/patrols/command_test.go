package patrols

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

func TestNewCommandMetadataAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCreateCommand(&stdout)

	if command.Use != "patrols <name>" {
		t.Fatalf("Use = %q, want patrols", command.Use)
	}
	if len(command.Commands()) != 0 {
		t.Fatalf("subcommand count = %d, want 0", len(command.Commands()))
	}
	if err := command.Help(); err != nil {
		t.Fatalf("Help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "--file")

	if NewUpdateCommand(&bytes.Buffer{}).Use != "patrols [name]" {
		t.Fatal("update command use changed")
	}
	if NewDeleteCommand(&bytes.Buffer{}).Use != "patrols [name]" {
		t.Fatal("delete command use changed")
	}
}

func TestCreateUpdateDeleteCommands(t *testing.T) {
	patrolsPath := copyPatrolFixture(t)

	var createOut bytes.Buffer
	create := NewCreateCommand(&createOut)
	create.SetArgs([]string{
		"Created",
		"--file", patrolsPath,
		"--dry-run",
		"--faction", "West",
		"--loadout", "TestLoadout",
		"--behaviour", "HALT",
		"--speed", "WALK",
		"--under-threat-speed", "SPRINT",
		"--chance", "1",
		"--number-of-ai", "1",
		"--number-of-ai-max", "1",
		"--can-be-looted", "1",
		"--enable-flanking-outside-combat", "-1",
		"--set-unit", "UnitA",
		"--add-unit", "UnitB",
		"--remove-unit", "UnitA",
		"--waypoint", "1,0,1",
		"--set-waypoint", "1=2,0,2",
		"--add-waypoint", "3,0,3",
		"--remove-waypoint", "1",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("create Execute returned error: %v", err)
	}
	assertContains(t, createOut.String(), `"Name": "Created"`)
	assertContains(t, createOut.String(), `"UnitB"`)

	var updateOut bytes.Buffer
	update := NewUpdateCommand(&updateOut)
	update.SetArgs([]string{
		"--file", patrolsPath,
		"--index", "1",
		"--set-name", "Updated",
		"--formation", "Vee",
		"--formation-scale", "-1",
		"--persist", "0",
	})
	if err := update.Execute(); err != nil {
		t.Fatalf("update Execute returned error: %v", err)
	}
	assertContains(t, updateOut.String(), "patrols "+patrolsPath+" ok")

	var nameUpdateOut bytes.Buffer
	nameUpdate := NewUpdateCommand(&nameUpdateOut)
	nameUpdate.SetArgs([]string{"Updated", "--file", patrolsPath, "--formation", "RANDOM"})
	if err := nameUpdate.Execute(); err != nil {
		t.Fatalf("name update Execute returned error: %v", err)
	}
	assertContains(t, nameUpdateOut.String(), "ok")

	var deleteOut bytes.Buffer
	deleteCommand := NewDeleteCommand(&deleteOut)
	deleteCommand.SetArgs([]string{"Updated", "--file", patrolsPath, "--dry-run"})
	if err := deleteCommand.Execute(); err != nil {
		t.Fatalf("delete Execute returned error: %v", err)
	}
	assertContains(t, deleteOut.String(), `"Patrols": []`)
}

func TestCommandErrorsAndHelpers(t *testing.T) {
	patrolsPath := copyPatrolFixture(t)

	tests := []struct {
		name        string
		constructor func(io.Writer) *cobra.Command
		args        []string
		want        string
	}{
		{name: "create parse", constructor: NewCreateCommand, args: []string{"Created", "--file", patrolsPath, "--waypoint", "1,2"}, want: "waypoint must be x,y,z"},
		{name: "create add waypoint parse", constructor: NewCreateCommand, args: []string{"Created", "--file", patrolsPath, "--add-waypoint", "1,bad,3"}, want: "waypoint coordinate"},
		{name: "create mutation", constructor: NewCreateCommand, args: []string{"Created", "--file", filepath.Join(t.TempDir(), "missing.json")}, want: "stat"},
		{name: "update parse", constructor: NewUpdateCommand, args: []string{"Fixture patrol", "--file", patrolsPath, "--set-waypoint", "bad"}, want: "set-waypoint"},
		{name: "update mutation", constructor: NewUpdateCommand, args: []string{"--file", patrolsPath}, want: "--index or --name"},
		{name: "delete mutation", constructor: NewDeleteCommand, args: []string{"--file", patrolsPath}, want: "--index or --name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := test.constructor(&bytes.Buffer{})
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}

	if _, err := parseVector("1,bad,3"); err == nil {
		t.Fatal("parseVector invalid number err = nil")
	}
	if _, err := parseWaypointUpdatesFlag([]string{"0=1,2,3"}); err == nil {
		t.Fatal("parseWaypointUpdatesFlag invalid index err = nil")
	}
	if _, err := parseWaypointUpdatesFlag([]string{"1=1,bad,3"}); err == nil {
		t.Fatal("parseWaypointUpdatesFlag invalid vector err = nil")
	}
	if _, err := parseVectorsFlag(nil); err != nil {
		t.Fatalf("parseVectorsFlag nil returned error: %v", err)
	}
	if updates, err := parseWaypointUpdatesFlag(nil); err != nil || len(updates) != 0 {
		t.Fatalf("parseWaypointUpdatesFlag nil = %#v, %v", updates, err)
	}

	originalWrite := writeFileMutation
	defer func() { writeFileMutation = originalWrite }()
	writeFileMutation = func(string, expansion.FileMutation) error { return errors.New("write failed") }
	if err := outputMutation("file.json", "patrols", expansion.FileMutation{Changed: true}, false, &bytes.Buffer{}); err == nil {
		t.Fatal("outputMutation write error = nil")
	}
}

func TestResolvePatrolsFileDiscovery(t *testing.T) {
	patrolsPath := copyPatrolFixture(t)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(patrolsPath))))
	t.Chdir(root)

	resolved, err := resolvePatrolsFile("")

	if err != nil {
		t.Fatalf("resolvePatrolsFile returned error: %v", err)
	}
	if resolved != patrolsPath {
		t.Fatalf("resolved = %q, want %q", resolved, patrolsPath)
	}
}

func TestResolvePatrolsFileErrors(t *testing.T) {
	original := getwd
	defer func() { getwd = original }()
	getwd = func() (string, error) { return "", errors.New("cwd failed") }

	if _, err := resolvePatrolsFile(""); err == nil {
		t.Fatal("resolvePatrolsFile getwd error = nil")
	}
	create := NewCreateCommand(&bytes.Buffer{})
	create.SetArgs([]string{"Created"})
	if err := create.Execute(); err == nil {
		t.Fatal("create getwd error = nil")
	}
	update := NewUpdateCommand(&bytes.Buffer{})
	update.SetArgs([]string{"Fixture patrol"})
	if err := update.Execute(); err == nil {
		t.Fatal("update getwd error = nil")
	}
	deleteCommand := NewDeleteCommand(&bytes.Buffer{})
	deleteCommand.SetArgs([]string{"Fixture patrol"})
	if err := deleteCommand.Execute(); err == nil {
		t.Fatal("delete getwd error = nil")
	}

	t.Chdir(t.TempDir())
	getwd = original
	if _, err := resolvePatrolsFile(""); err == nil {
		t.Fatal("resolvePatrolsFile missing discovery error = nil")
	}
}

func copyPatrolFixture(t *testing.T) string {
	t.Helper()

	source := fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")
	target := filepath.Join(t.TempDir(), "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("make temp dir: %v", err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}
	return target
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "..", "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}
