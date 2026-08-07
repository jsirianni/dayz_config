package loadouts

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

	if command.Use != "loadouts <name>" {
		t.Fatalf("Use = %q, want loadouts", command.Use)
	}
	if len(command.Commands()) != 1 {
		t.Fatalf("subcommand count = %d, want 1", len(command.Commands()))
	}
	if err := command.Help(); err != nil {
		t.Fatalf("Help returned error: %v", err)
	}
	assertContains(t, stdout.String(), "--file")
	assertContains(t, stdout.String(), "item")

	var itemOut bytes.Buffer
	itemCommand := findCommand(t, command, "item")
	itemCommand.SetOut(&itemOut)
	if err := itemCommand.Help(); err != nil {
		t.Fatalf("item Help returned error: %v", err)
	}
	assertContains(t, itemOut.String(), "--parent")

	if NewUpdateCommand(&bytes.Buffer{}).Use != "loadouts <name>" {
		t.Fatal("update command use changed")
	}
	if NewDeleteCommandWithInput(strings.NewReader(""), &bytes.Buffer{}).Use != "loadouts <name>" {
		t.Fatal("delete command use changed")
	}
}

func TestCreateUpdateItemAndDeleteCommands(t *testing.T) {
	loadoutPath := copyLoadoutFixture(t, "TestLoadout.json")

	var createOut bytes.Buffer
	create := NewCreateCommand(&createOut)
	create.SetArgs([]string{
		"NewLoadout",
		"--file", filepath.Join(t.TempDir(), "NewLoadout.json"),
		"--dry-run",
		"--class-name", "SurvivorM_Mirek",
		"--chance", "1",
		"--quantity-min", "0",
		"--quantity-max", "1",
		"--health", "=0.7,1",
		"--set-construction-part", "PartA",
		"--add-construction-part", "PartB",
		"--remove-construction-part", "PartA",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("create Execute returned error: %v", err)
	}
	assertContains(t, createOut.String(), `"ClassName": "SurvivorM_Mirek"`)
	assertContains(t, createOut.String(), `"PartB"`)

	var updateOut bytes.Buffer
	update := NewUpdateCommand(&updateOut)
	update.SetArgs([]string{"TestLoadout", "--file", loadoutPath, "--include", "", "--clear-health", "--clear-construction-parts", "--add-construction-part", "PartC"})
	if err := update.Execute(); err != nil {
		t.Fatalf("update Execute returned error: %v", err)
	}
	assertContains(t, updateOut.String(), "loadouts "+loadoutPath+" ok")

	var addOut bytes.Buffer
	add := NewCreateCommand(&addOut)
	add.SetArgs([]string{"item", "TestLoadout", "--file", loadoutPath, "--parent", "root", "--container", "cargo", "--class-name", "Apple"})
	if err := add.Execute(); err != nil {
		t.Fatalf("item add Execute returned error: %v", err)
	}
	assertContains(t, addOut.String(), "ok")

	var itemUpdateOut bytes.Buffer
	itemUpdate := NewUpdateCommand(&itemUpdateOut)
	itemUpdate.SetArgs([]string{"item", "TestLoadout", "root/cargo:1", "--file", loadoutPath, "--class-name", "Pear", "--dry-run"})
	if err := itemUpdate.Execute(); err != nil {
		t.Fatalf("item update Execute returned error: %v", err)
	}
	assertContains(t, itemUpdateOut.String(), `"ClassName": "Pear"`)

	var removeOut bytes.Buffer
	remove := NewDeleteCommandWithInput(strings.NewReader(""), &removeOut)
	remove.SetArgs([]string{"item", "TestLoadout", "root/cargo:1", "--file", loadoutPath})
	if err := remove.Execute(); err != nil {
		t.Fatalf("item remove Execute returned error: %v", err)
	}
	assertContains(t, removeOut.String(), "ok")

	forcePath := copyLoadoutFixture(t, "IncludedLoadout.json")
	var forceOut bytes.Buffer
	forceDelete := NewDeleteCommandWithInput(strings.NewReader(""), &forceOut)
	forceDelete.SetArgs([]string{"IncludedLoadout", "--file", forcePath, "--force"})
	if err := forceDelete.Execute(); err != nil {
		t.Fatalf("force delete Execute returned error: %v", err)
	}
	assertContains(t, forceOut.String(), "ok")
	if _, err := os.Stat(forcePath); !os.IsNotExist(err) {
		t.Fatalf("force delete left file behind: %v", err)
	}
}

func TestDeleteDryRunAndPrompts(t *testing.T) {
	loadoutPath := copyLoadoutFixture(t, "TestLoadout.json")
	patrolsPath := fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")

	var dryRunOut bytes.Buffer
	dryRun := NewDeleteCommandWithInput(strings.NewReader(""), &dryRunOut)
	dryRun.SetArgs([]string{"TestLoadout", "--file", loadoutPath, "--dry-run", "--patrols-file", patrolsPath})
	if err := dryRun.Execute(); err != nil {
		t.Fatalf("dry-run delete Execute returned error: %v", err)
	}
	assertContains(t, dryRunOut.String(), "would delete")
	assertContains(t, dryRunOut.String(), "1 patrol references")

	var noOut bytes.Buffer
	noDelete := NewDeleteCommandWithInput(strings.NewReader("no\n"), &noOut)
	noDelete.SetArgs([]string{"TestLoadout", "--file", loadoutPath, "--patrols-file", patrolsPath})
	if err := noDelete.Execute(); err != nil {
		t.Fatalf("no delete Execute returned error: %v", err)
	}
	assertContains(t, noOut.String(), "referenced by 1 patrols")
	assertContains(t, noOut.String(), "not deleted")

	yesPath := copyLoadoutFixture(t, "IncludedLoadout.json")
	var yesOut bytes.Buffer
	yesDelete := NewDeleteCommandWithInput(strings.NewReader("yes\n"), &yesOut)
	yesDelete.SetArgs([]string{"IncludedLoadout", "--file", yesPath})
	if err := yesDelete.Execute(); err != nil {
		t.Fatalf("yes delete Execute returned error: %v", err)
	}
	assertContains(t, yesOut.String(), "Delete loadout")
	assertContains(t, yesOut.String(), "ok")
}

func TestCommandErrorsAndHelpers(t *testing.T) {
	loadoutPath := copyLoadoutFixture(t, "TestLoadout.json")

	tests := []struct {
		name        string
		constructor func(io.Reader, io.Writer) *cobra.Command
		args        []string
		want        string
	}{
		{name: "create parse", constructor: createConstructor, args: []string{"New", "--file", filepath.Join(t.TempDir(), "New.json"), "--health", "bad"}, want: "health must"},
		{name: "create mutation", constructor: createConstructor, args: []string{"TestLoadout", "--file", loadoutPath}, want: "already exists"},
		{name: "update parse", constructor: updateConstructor, args: []string{"TestLoadout", "--file", loadoutPath, "--health", "=bad,1"}, want: "health min"},
		{name: "update mutation", constructor: updateConstructor, args: []string{"Missing", "--file", filepath.Join(t.TempDir(), "missing.json"), "--class-name", "Missing"}, want: "stat"},
		{name: "item add parent", constructor: createConstructor, args: []string{"item", "TestLoadout", "--file", loadoutPath, "--container", "cargo"}, want: "--parent"},
		{name: "item add container", constructor: createConstructor, args: []string{"item", "TestLoadout", "--file", loadoutPath, "--parent", "root"}, want: "--container"},
		{name: "item add parse", constructor: createConstructor, args: []string{"item", "TestLoadout", "--file", loadoutPath, "--parent", "root", "--container", "cargo", "--health", "bad"}, want: "health must"},
		{name: "item add mutation", constructor: createConstructor, args: []string{"item", "TestLoadout", "--file", loadoutPath, "--parent", "bad", "--container", "cargo"}, want: "path must start"},
		{name: "item update parse", constructor: updateConstructor, args: []string{"item", "TestLoadout", "root", "--file", loadoutPath, "--health", "bad"}, want: "health must"},
		{name: "item update mutation", constructor: updateConstructor, args: []string{"item", "TestLoadout", "bad", "--file", loadoutPath}, want: "path must start"},
		{name: "item remove mutation", constructor: deleteConstructor, args: []string{"item", "TestLoadout", "root", "--file", loadoutPath}, want: "cannot remove root"},
		{name: "delete mutation", constructor: deleteConstructor, args: []string{"Missing", "--file", filepath.Join(t.TempDir(), "missing.json"), "--force"}, want: "stat"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := test.constructor(strings.NewReader(""), &bytes.Buffer{})
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}

	for _, value := range []string{"=1", "=x,1", "=1,x"} {
		if _, _, err := parseHealth(value); err == nil {
			t.Fatalf("parseHealth(%q) err = nil", value)
		}
	}
	if zone, minMax, err := parseHealth("Body=0.1,0.2"); err != nil || zone != "Body" || minMax.Min != 0.1 {
		t.Fatalf("parseHealth valid = %q %#v %v", zone, minMax, err)
	}
	var planOut bytes.Buffer
	printDeletePlan(&planOut, expansion.LoadoutDeletePlan{Path: "Loose.json"})
	assertContains(t, planOut.String(), "would delete")

	_, err := confirmDelete(errorReader{}, &bytes.Buffer{}, expansion.LoadoutDeletePlan{Path: "Loose.json"})
	if err == nil {
		t.Fatal("confirmDelete read error = nil")
	}
	errorPrompt := NewDeleteCommandWithInput(errorReader{}, &bytes.Buffer{})
	errorPrompt.SetArgs([]string{"TestLoadout", "--file", loadoutPath})
	if err := errorPrompt.Execute(); err == nil {
		t.Fatal("delete prompt read error = nil")
	}

	originalWrite := writeFileMutation
	originalDelete := deleteLoadoutFile
	defer func() {
		writeFileMutation = originalWrite
		deleteLoadoutFile = originalDelete
	}()
	writeFileMutation = func(string, expansion.FileMutation) error { return errors.New("write failed") }
	if err := outputMutation("file.json", "loadouts", expansion.FileMutation{Changed: true}, false, &bytes.Buffer{}); err == nil {
		t.Fatal("outputMutation write error = nil")
	}
	deleteLoadoutFile = func(string) error { return errors.New("delete failed") }
	deleteCommand := NewDeleteCommandWithInput(strings.NewReader(""), &bytes.Buffer{})
	deleteCommand.SetArgs([]string{"TestLoadout", "--file", loadoutPath, "--force"})
	if err := deleteCommand.Execute(); err == nil {
		t.Fatal("delete command delete error = nil")
	}
}

func TestResolveLoadoutDiscovery(t *testing.T) {
	root := fixturePath(t, "expansionai", "valid")
	t.Chdir(root)

	resolved, err := resolveLoadoutFile("TestLoadout", "")
	if err != nil {
		t.Fatalf("resolveLoadoutFile returned error: %v", err)
	}
	assertContains(t, resolved, filepath.Join("profiles", "ExpansionMod", "Loadouts", "TestLoadout.json"))

	newPath, err := resolveNewLoadoutFile("NewLoadout", "")
	if err != nil {
		t.Fatalf("resolveNewLoadoutFile returned error: %v", err)
	}
	assertContains(t, newPath, filepath.Join("profiles", "ExpansionMod", "Loadouts", "NewLoadout.json"))
}

func TestResolveLoadoutDiscoveryErrors(t *testing.T) {
	original := getwd
	defer func() { getwd = original }()
	getwd = func() (string, error) { return "", errors.New("cwd failed") }

	if _, err := resolveLoadoutFile("TestLoadout", ""); err == nil {
		t.Fatal("resolveLoadoutFile getwd error = nil")
	}
	if _, err := resolveNewLoadoutFile("TestLoadout", ""); err == nil {
		t.Fatal("resolveNewLoadoutFile getwd error = nil")
	}
	create := NewCreateCommand(&bytes.Buffer{})
	create.SetArgs([]string{"NewLoadout"})
	if err := create.Execute(); err == nil {
		t.Fatal("create getwd error = nil")
	}
	update := NewUpdateCommand(&bytes.Buffer{})
	update.SetArgs([]string{"TestLoadout"})
	if err := update.Execute(); err == nil {
		t.Fatal("update getwd error = nil")
	}
	deleteCommand := NewDeleteCommandWithInput(strings.NewReader(""), &bytes.Buffer{})
	deleteCommand.SetArgs([]string{"TestLoadout", "--force"})
	if err := deleteCommand.Execute(); err == nil {
		t.Fatal("delete getwd error = nil")
	}
	itemAdd := NewCreateCommand(&bytes.Buffer{})
	itemAdd.SetArgs([]string{"item", "TestLoadout", "--parent", "root", "--container", "cargo"})
	if err := itemAdd.Execute(); err == nil {
		t.Fatal("item add getwd error = nil")
	}
	itemUpdate := NewUpdateCommand(&bytes.Buffer{})
	itemUpdate.SetArgs([]string{"item", "TestLoadout", "root"})
	if err := itemUpdate.Execute(); err == nil {
		t.Fatal("item update getwd error = nil")
	}
	itemRemove := NewDeleteCommandWithInput(strings.NewReader(""), &bytes.Buffer{})
	itemRemove.SetArgs([]string{"item", "TestLoadout", "root"})
	if err := itemRemove.Execute(); err == nil {
		t.Fatal("item remove getwd error = nil")
	}

	t.Chdir(t.TempDir())
	getwd = original
	if _, err := resolveLoadoutFile("TestLoadout", ""); err == nil {
		t.Fatal("resolveLoadoutFile missing discovery error = nil")
	}
	if _, err := resolveNewLoadoutFile("TestLoadout", ""); err == nil {
		t.Fatal("resolveNewLoadoutFile missing discovery error = nil")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func copyLoadoutFixture(t *testing.T, name string) string {
	t.Helper()

	source := fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", name)
	target := filepath.Join(t.TempDir(), name)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}
	return target
}

func createConstructor(_ io.Reader, stdout io.Writer) *cobra.Command {
	return NewCreateCommand(stdout)
}

func updateConstructor(_ io.Reader, stdout io.Writer) *cobra.Command {
	return NewUpdateCommand(stdout)
}

func deleteConstructor(stdin io.Reader, stdout io.Writer) *cobra.Command {
	return NewDeleteCommandWithInput(stdin, stdout)
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

func findCommand(t *testing.T, command *cobra.Command, usePrefix string) *cobra.Command {
	t.Helper()
	for _, child := range command.Commands() {
		if strings.HasPrefix(child.Use, usePrefix) {
			return child
		}
	}
	t.Fatalf("command %q not found", usePrefix)
	return nil
}

var _ io.Reader = errorReader{}
