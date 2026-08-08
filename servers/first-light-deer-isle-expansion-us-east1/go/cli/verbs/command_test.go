package verbs

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

func TestVerbCommandMetadataAndHelp(t *testing.T) {
	tests := []struct {
		name    string
		command *cobra.Command
		want    string
	}{
		{name: "get", command: NewGetCommand(&bytes.Buffer{}), want: "economy"},
		{name: "create", command: NewCreateCommand(&bytes.Buffer{}), want: "expansion"},
		{name: "update", command: NewUpdateCommand(strings.NewReader(""), &bytes.Buffer{}), want: "economy"},
		{name: "delete", command: NewDeleteCommand(strings.NewReader(""), &bytes.Buffer{}), want: "expansion"},
		{name: "validate", command: NewValidateCommand(&bytes.Buffer{}), want: "init"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			test.command.SetOut(&stdout)

			if err := test.command.RunE(test.command, nil); err != nil {
				t.Fatalf("RunE returned error: %v", err)
			}
			assertContains(t, stdout.String(), test.want)
		})
	}
}

func TestValidateCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "economy", args: []string{"economy", fixturePath(t, "mission", "cfgeconomycore.xml")}, want: "cfgeconomycore"},
		{name: "expansion ai", args: []string{"expansion", "ai", fixturePath(t, "expansionai", "valid")}, want: "expansion-ai-patrols"},
		{name: "init", args: []string{"init", fixturePath(t, "dayzinit", "valid", "init.c")}, want: "init"},
		{name: "server", args: []string{"server", fixturePath(t, "serverconfig", "valid.cfg")}, want: "server"},
		{name: "gameplay", args: []string{"gameplay", fixturePath(t, "gameplayconfig", "valid.json")}, want: "gameplay"},
		{name: "weather", args: []string{"weather", fixturePath(t, "weatherconfig", "valid-modern", "cfgweather.xml")}, want: "weather"},
		{name: "xml", args: []string{"xml", fixturePath(t, "xml", "valid")}, want: "root.xml ok"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			command := NewValidateCommand(&stdout)
			command.SetArgs(test.args)

			err := command.Execute()

			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			assertContains(t, stdout.String(), test.want)
		})
	}
}

func TestGetEconomyTypes(t *testing.T) {
	var stdout bytes.Buffer
	command := NewGetCommand(&stdout)
	typesFile := fixturePath(t, "mission", "db", "types.xml")
	command.SetArgs([]string{"economy", "types", "--file", typesFile})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertNotContains(t, firstOutputLine(stdout.String()), "KIND")
	assertContains(t, firstOutputLine(stdout.String()), "NAME")
	assertNotContains(t, firstOutputLine(stdout.String()), "OCCURRENCE")
	assertContains(t, stdout.String(), "Apple")
	assertContains(t, stdout.String(), "BandageDressing")
	assertContains(t, stdout.String(), "types.xml")
	assertNotContains(t, stdout.String(), typesFile)

	stdout.Reset()
	command = NewGetCommand(&stdout)
	command.SetArgs([]string{"economy", "types", "ModdedItem", "--cfgeconomycore", fixturePath(t, "mission", "cfgeconomycore.xml")})
	if err := command.Execute(); err != nil {
		t.Fatalf("filtered Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ModdedItem")
	assertNotContains(t, stdout.String(), "Apple")
}

func TestGetEconomyLimitsAndGroups(t *testing.T) {
	var stdout bytes.Buffer
	command := NewGetCommand(&stdout)
	limitsFile := fixturePath(t, "mission", "cfglimitsdefinition.xml")
	command.SetArgs([]string{"economy", "limits", "category", "--file", limitsFile})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertNotContains(t, firstOutputLine(stdout.String()), "KIND")
	assertContains(t, firstOutputLine(stdout.String()), "NAME")
	assertNotContains(t, firstOutputLine(stdout.String()), "FILE")
	assertContains(t, stdout.String(), "tools")
	assertContains(t, stdout.String(), "weapons")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	userLimitsFile := fixturePath(t, "mission", "cfglimitsdefinitionuser.xml")
	command.SetArgs([]string{"economy", "limits", "group", "usage", "CustomUsageGroup", "--file", userLimitsFile})
	if err := command.Execute(); err != nil {
		t.Fatalf("group Execute returned error: %v", err)
	}
	assertNotContains(t, firstOutputLine(stdout.String()), "KIND")
	assertContains(t, firstOutputLine(stdout.String()), "MEMBERS")
	assertNotContains(t, firstOutputLine(stdout.String()), "FILE")
	assertContains(t, stdout.String(), "Military")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	command.SetArgs([]string{"economy", "limits", "tag", "floor", "--file", fixturePath(t, "mission", "cfglimitsdefinition.xml")})
	if err := command.Execute(); err != nil {
		t.Fatalf("filtered limits Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "floor")
	assertNotContains(t, stdout.String(), "Military")
}

func TestGetExpansionAI(t *testing.T) {
	var stdout bytes.Buffer
	command := NewGetCommand(&stdout)
	patrolsFile := fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")
	command.SetArgs([]string{"expansion", "ai", "patrols", "Fixture patrol", "--file", patrolsFile})

	err := command.Execute()

	if err != nil {
		t.Fatalf("patrols Execute returned error: %v", err)
	}
	assertNotContains(t, firstOutputLine(stdout.String()), "KIND")
	assertContains(t, firstOutputLine(stdout.String()), "INDEX")
	assertContains(t, firstOutputLine(stdout.String()), "COORDINATES")
	assertNotContains(t, firstOutputLine(stdout.String()), "FILE")
	assertNotContains(t, firstOutputLine(stdout.String()), "OCCURRENCE")
	assertContains(t, stdout.String(), "Fixture patrol")
	assertContains(t, stdout.String(), "TestLoadout")
	assertContains(t, stdout.String(), "100,0,200")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	command.SetArgs([]string{"expansion", "ai", "loadouts", "--path", fixturePath(t, "expansionai", "valid")})
	if err := command.Execute(); err != nil {
		t.Fatalf("loadouts Execute returned error: %v", err)
	}
	assertNotContains(t, firstOutputLine(stdout.String()), "KIND")
	assertContains(t, firstOutputLine(stdout.String()), "ITEMS")
	assertNotContains(t, firstOutputLine(stdout.String()), "FILE")
	assertContains(t, stdout.String(), "IncludedLoadout")
	assertContains(t, stdout.String(), "TestLoadout")

	stdout.Reset()
	command = NewGetCommand(&stdout)
	loadoutFile := fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", "TestLoadout.json")
	command.SetArgs([]string{"expansion", "ai", "loadouts", "TestLoadout", "--file", loadoutFile})
	if err := command.Execute(); err != nil {
		t.Fatalf("loadout file Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "4")
	assertNotContains(t, stdout.String(), "IncludedLoadout")
	assertNotContains(t, stdout.String(), "TestLoadout.json")
	assertNotContains(t, stdout.String(), loadoutFile)
}

func TestGetExpansionAIPatrolsDropsEmptyNameColumn(t *testing.T) {
	patrolsFile := writeTempFile(t, `{
  "Patrols": [
    {"Name": "", "Loadout": "WestLoadout", "Faction": "West", "Behaviour": "ROAMING_LOCAL", "Waypoints": [[10,0,10]]},
    {"Name": "", "Loadout": "MedicLoadout", "Faction": "Raiders", "Behaviour": "LOOP", "Waypoints": [[2.5,0,3.75]]}
  ]
}`)

	var stdout bytes.Buffer
	if err := listExpansionAIPatrols(patrolsFile, "", &stdout); err != nil {
		t.Fatalf("listExpansionAIPatrols returned error: %v", err)
	}

	header := firstOutputLine(stdout.String())
	assertNotContains(t, header, "NAME")
	assertNotContains(t, header, "FILE")
	assertNotContains(t, header, "OCCURRENCE")
	assertContains(t, header, "INDEX")
	assertContains(t, header, "COORDINATES")
	assertContains(t, stdout.String(), "WestLoadout")
	assertContains(t, stdout.String(), "10,0,10")
	assertContains(t, stdout.String(), "2.5,0,3.75")
}

func TestGetDiscoveryFromWorkingDirectory(t *testing.T) {
	root := fixturePath(t, "expansionai", "valid")
	t.Chdir(root)

	var stdout bytes.Buffer
	if err := listExpansionAIPatrols("", "Fixture patrol", &stdout); err != nil {
		t.Fatalf("listExpansionAIPatrols returned error: %v", err)
	}
	assertContains(t, stdout.String(), "Fixture patrol")

	stdout.Reset()
	if err := listExpansionAILoadouts("", "", "TestLoadout", &stdout); err != nil {
		t.Fatalf("listExpansionAILoadouts returned error: %v", err)
	}
	assertContains(t, stdout.String(), "TestLoadout")
}

func TestGetErrorsAndHelpers(t *testing.T) {
	var stdout bytes.Buffer
	if err := listEconomyTypes("", "", "", &stdout); err == nil {
		t.Fatal("listEconomyTypes missing source err = nil")
	}
	if err := listEconomyTypes("types.xml", "cfgeconomycore.xml", "", &stdout); err == nil {
		t.Fatal("listEconomyTypes both source err = nil")
	}
	if err := listEconomyTypes(fixturePath(t, "mission", "db", "types.xml"), "", "Missing", &stdout); err == nil {
		t.Fatal("listEconomyTypes missing name err = nil")
	}
	if err := listEconomyTypes(writeTempFile(t, `<types>`), "", "", &stdout); err == nil {
		t.Fatal("listEconomyTypes parse err = nil")
	}
	if _, err := economyTypeFiles("", writeTempFile(t, `<notxml>`)); err == nil {
		t.Fatal("economyTypeFiles parse err = nil")
	}
	if err := listEconomyLimits("", "tag", "", &stdout); err == nil {
		t.Fatal("listEconomyLimits missing file err = nil")
	}
	if err := listEconomyLimits(fixturePath(t, "mission", "cfglimitsdefinition.xml"), "bad", "", &stdout); err == nil {
		t.Fatal("listEconomyLimits bad kind err = nil")
	}
	if err := listEconomyLimitGroups("", "usage", "", &stdout); err == nil {
		t.Fatal("listEconomyLimitGroups missing file err = nil")
	}
	if err := listEconomyLimitGroups(fixturePath(t, "mission", "cfglimitsdefinitionuser.xml"), "tag", "", &stdout); err == nil {
		t.Fatal("listEconomyLimitGroups bad kind err = nil")
	}
	if err := listExpansionAIPatrols(fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", "TestLoadout.json"), "", &stdout); err == nil {
		t.Fatal("listExpansionAIPatrols parse err = nil")
	}
	if err := listExpansionAIPatrols(fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json"), "Missing", &stdout); err == nil {
		t.Fatal("listExpansionAIPatrols missing err = nil")
	}
	if err := listExpansionAILoadouts(writeTempFile(t, `{"ClassName":`), "", "", &stdout); err == nil {
		t.Fatal("listExpansionAILoadouts parse err = nil")
	}
	if err := listExpansionAILoadouts("", fixturePath(t, "expansionai", "valid"), "Missing", &stdout); err == nil {
		t.Fatal("listExpansionAILoadouts missing err = nil")
	}
	if err := printMatchedRows(io.Discard, []string{"KIND"}, nil, "thing", "name"); err == nil {
		t.Fatal("printMatchedRows filtered err = nil")
	}
	if err := printMatchedRows(io.Discard, []string{"KIND"}, nil, "thing", ""); err == nil {
		t.Fatal("printMatchedRows empty err = nil")
	}

	rows := [][]string{{"kind", "same", "b"}, {"kind", "same", "a"}}
	sortRows(rows, 1)
	if rows[0][2] != "a" {
		t.Fatalf("sortRows fallback first row = %#v", rows[0])
	}
	assertContains(t, strings.Join(filterAIFilePaths([]expansion.AIFile{{Path: "a", Kind: expansion.KindAILoadout}, {Path: "b", Kind: expansion.KindAIPatrols}}, expansion.KindAILoadout), ","), "a")
}

func TestDiscoveryErrors(t *testing.T) {
	original := getwd
	defer func() { getwd = original }()
	getwd = func() (string, error) { return "", errors.New("cwd failed") }

	if _, err := loadoutFiles("", ""); err == nil {
		t.Fatal("loadoutFiles getwd err = nil")
	}
	if _, err := resolvePatrolsFile(""); err == nil {
		t.Fatal("resolvePatrolsFile getwd err = nil")
	}
	if err := listExpansionAIPatrols("", "", io.Discard); err == nil {
		t.Fatal("listExpansionAIPatrols discovery err = nil")
	}
	if err := listExpansionAILoadouts("", "", "", io.Discard); err == nil {
		t.Fatal("listExpansionAILoadouts discovery err = nil")
	}

	getwd = original
	t.Chdir(t.TempDir())
	if _, err := loadoutFiles("", ""); err == nil {
		t.Fatal("loadoutFiles missing discovery err = nil")
	}
	if _, err := resolvePatrolsFile(""); err == nil {
		t.Fatal("resolvePatrolsFile missing discovery err = nil")
	}
}

func TestLoadoutFilesPathErrors(t *testing.T) {
	if _, err := loadoutFiles("", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("loadoutFiles path err = nil")
	}
	if paths, err := loadoutFiles(fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", "TestLoadout.json"), "ignored"); err != nil || len(paths) != 1 {
		t.Fatalf("loadoutFiles explicit = %#v, %v", paths, err)
	}
}

func TestDropEmptyNameColumnIgnoresTablesWithoutName(t *testing.T) {
	headers, rows := dropEmptyNameColumn([]string{"INDEX"}, [][]string{{"1"}})

	if headers[0] != "INDEX" {
		t.Fatalf("header = %q, want INDEX", headers[0])
	}
	if rows[0][0] != "1" {
		t.Fatalf("row = %q, want 1", rows[0][0])
	}
}

func TestSortNamelessIndexRowsUsesNumericOrder(t *testing.T) {
	rows := [][]string{
		{"10", "ten"},
		{"2", "two"},
		{"1", "one"},
	}

	sortNamelessIndexRows([]string{"INDEX", "LOADOUT"}, rows)

	if got := strings.Join([]string{rows[0][0], rows[1][0], rows[2][0]}, ","); got != "1,2,10" {
		t.Fatalf("index order = %s, want 1,2,10", got)
	}

	namedRows := [][]string{{"B", "2"}, {"A", "1"}}
	sortNamelessIndexRows([]string{"NAME", "INDEX"}, namedRows)
	if namedRows[0][0] != "B" {
		t.Fatalf("named rows changed: %#v", namedRows)
	}

	unindexedRows := [][]string{{"ten"}, {"two"}}
	sortNamelessIndexRows([]string{"LOADOUT"}, unindexedRows)
	if unindexedRows[0][0] != "ten" {
		t.Fatalf("unindexed rows changed: %#v", unindexedRows)
	}
}

func TestFormatPatrolCoordinates(t *testing.T) {
	if got := formatPatrolCoordinates(expansion.AIPatrol{}); got != "" {
		t.Fatalf("empty coordinates = %q, want blank", got)
	}

	patrol := expansion.AIPatrol{
		Waypoints: []expansion.Vector{{1.25, 0, 3}},
	}
	if got := formatPatrolCoordinates(patrol); got != "1.25,0,3" {
		t.Fatalf("coordinates = %q, want 1.25,0,3", got)
	}
}

func TestValidateExpansionParentHelp(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)
	command.SetArgs([]string{"expansion"})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "ai")
}

func TestValidateTopLevelHelpListsPromotedResources(t *testing.T) {
	var stdout bytes.Buffer
	command := NewValidateCommand(&stdout)
	command.SetArgs([]string{})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "server")
	assertContains(t, stdout.String(), "gameplay")
	assertContains(t, stdout.String(), "weather")
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
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

func firstOutputLine(output string) string {
	line, _, _ := strings.Cut(output, "\n")
	return line
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.xml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
