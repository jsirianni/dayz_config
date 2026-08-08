package expansion

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInspectAIPathValidServerRoot(t *testing.T) {
	statuses, err := InspectAIPath(fixturePath(t, "expansionai", "valid"))

	if err != nil {
		t.Fatalf("InspectAIPath returned error: %v", err)
	}
	if len(statuses) != 6 {
		t.Fatalf("status count = %d, want 6", len(statuses))
	}
	assertStatus(t, statuses, KindAISettings, "AISettings.json", "")
	assertStatus(t, statuses, KindAIPatrols, "AIPatrolSettings.json", "1 patrols")
	assertStatus(t, statuses, KindAILocations, "AILocationSettings.json", "1 locations")
	assertStatus(t, statuses, KindAILootDrop, "Drop.json", "1 entries")
	assertStatus(t, statuses, KindAILoadout, "IncludedLoadout.json", "1 items")
	assertStatus(t, statuses, KindAILoadout, "TestLoadout.json", "4 items")
	for _, status := range statuses {
		if status.Err != nil {
			t.Fatalf("%s returned error: %v", status.Path, status.Err)
		}
	}
}

func TestDiscoverAIConfigFilesSupportsMissionAndDirectPaths(t *testing.T) {
	missionRoot := fixturePath(t, "expansionai", "valid", "empty.deerisle")
	files, err := DiscoverAIConfigFiles(missionRoot)

	if err != nil {
		t.Fatalf("DiscoverAIConfigFiles returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}

	directLoadout := fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", "TestLoadout.json")
	files, err = DiscoverAIConfigFiles(directLoadout)
	if err != nil {
		t.Fatalf("direct DiscoverAIConfigFiles returned error: %v", err)
	}
	assertEqual(t, files[0].Kind, KindAILoadout)
	assertEqual(t, files[0].Path, directLoadout)

	directFiles := map[string]string{
		fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Settings", "AISettings.json"):            KindAISettings,
		fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json"):   KindAIPatrols,
		fixturePath(t, "expansionai", "valid", "empty.deerisle", "expansion", "settings", "AILocationSettings.json"): KindAILocations,
		fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "AI", "LootDrops", "Drop.json"):           KindAILootDrop,
		fixturePath(t, "expansionai", "valid", "profiles", "ExpansionMod", "Loadouts", "IncludedLoadout.json"):       KindAILoadout,
	}
	for path, kind := range directFiles {
		t.Run(filepath.Base(path), func(t *testing.T) {
			files, err := DiscoverAIConfigFiles(path)
			if err != nil {
				t.Fatalf("DiscoverAIConfigFiles returned error: %v", err)
			}
			assertEqual(t, files[0].Kind, kind)
		})
	}
}

func TestDiscoverAIConfigFilesErrors(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "note.txt"), "not json")

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "missing", path: filepath.Join(dir, "missing"), want: "stat"},
		{name: "empty directory", path: dir, want: "no Expansion AI config files"},
		{name: "unsupported extension", path: filepath.Join(dir, "note.txt"), want: "not a supported"},
		{name: "unsupported json", path: writeTestFile(t, filepath.Join(dir, "Other.json"), "{}"), want: "not a supported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DiscoverAIConfigFiles(test.path)
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestDiscoverAIConfigFilesNearAndNamedHelpers(t *testing.T) {
	root := fixturePath(t, "expansionai", "valid")
	files, err := DiscoverAIConfigFilesNear(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "TestLoadout.json"))
	if err != nil {
		t.Fatalf("DiscoverAIConfigFilesNear returned error: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("DiscoverAIConfigFilesNear returned no files")
	}

	patrolsFile, err := FindAIPatrolsFileNear(root)
	if err != nil {
		t.Fatalf("FindAIPatrolsFileNear returned error: %v", err)
	}
	assertContains(t, patrolsFile, "AIPatrolSettings.json")

	loadoutsDir, err := FindAILoadoutsDirNear(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "NewLoadout.json"))
	if err != nil {
		t.Fatalf("FindAILoadoutsDirNear returned error: %v", err)
	}
	assertContains(t, loadoutsDir, filepath.Join("profiles", "ExpansionMod", "Loadouts"))

	loadoutFile, err := FindAILoadoutFileNear("TestLoadout.json", root)
	if err != nil {
		t.Fatalf("FindAILoadoutFileNear returned error: %v", err)
	}
	assertEqual(t, LoadoutName(loadoutFile), "TestLoadout")
	assertEqual(t, NormalizeLoadoutName("TestLoadout.json"), "TestLoadout")
	assertEqual(t, NormalizeLoadoutName("TestLoadout"), "TestLoadout")
	assertEqual(t, CountPrefabItems(PrefabObject{Chance: 1, InventoryCargo: []PrefabObject{{Chance: 1}}}), 2)
}

func TestDiscoverAIConfigFilesNearErrors(t *testing.T) {
	if _, err := DiscoverAIConfigFilesNear(t.TempDir()); err == nil {
		t.Fatal("DiscoverAIConfigFilesNear missing err = nil")
	}
	if _, err := FindAILoadoutsDirNear(t.TempDir()); err == nil {
		t.Fatal("FindAILoadoutsDirNear missing err = nil")
	}
	if _, err := FindAIPatrolsFileNear(t.TempDir()); err == nil {
		t.Fatal("FindAIPatrolsFileNear discovery err = nil")
	}
	if _, err := FindAILoadoutFileNear("Missing", t.TempDir()); err == nil {
		t.Fatal("FindAILoadoutFileNear discovery err = nil")
	}

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Only.json"), `{}`)
	if _, err := FindAIPatrolsFileNear(root); err == nil {
		t.Fatal("FindAIPatrolsFileNear missing err = nil")
	}
	if _, err := FindAILoadoutFileNear("Missing", root); err == nil {
		t.Fatal("FindAILoadoutFileNear missing err = nil")
	}

	writeTestFile(t, filepath.Join(root, "first", "expansion", "settings", "AIPatrolSettings.json"), `{}`)
	writeTestFile(t, filepath.Join(root, "second", "expansion", "settings", "AIPatrolSettings.json"), `{}`)
	if _, err := FindAIPatrolsFileNear(root); err == nil {
		t.Fatal("FindAIPatrolsFileNear ambiguous err = nil")
	}
}

func TestDiscoverySmallHelpers(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, filepath.Join(dir, "file.json"), `{}`)
	assertEqual(t, nearestExistingDir(file), dir)
	assertEqual(t, nearestExistingDir(filepath.Join(dir, "missing.json")), dir)
	assertEqual(t, nearestExistingDir(filepath.Join(dir, "missing")), filepath.Join(dir, "missing"))

	path, err := requireSinglePath([]string{"b", "a"}, "resource", "--file")
	if err == nil || path != "" {
		t.Fatal("requireSinglePath many should return ambiguity")
	}
	path, err = requireSinglePath([]string{"only"}, "resource", "--file")
	if err != nil || path != "only" {
		t.Fatalf("requireSinglePath one = %q, %v", path, err)
	}
	_, err = requireSinglePath(nil, "resource", "--file")
	if err == nil {
		t.Fatal("requireSinglePath none err = nil")
	}
}

func TestInspectAIPathReturnsDiscoveryError(t *testing.T) {
	_, err := InspectAIPath(filepath.Join(t.TempDir(), "missing"))

	if err == nil {
		t.Fatal("err = nil, want error")
	}
	assertContains(t, err.Error(), "stat")
}

func TestDiscoverAIConfigFilesReturnsReadDirError(t *testing.T) {
	originalReadDir := readDir
	defer func() { readDir = originalReadDir }()

	readDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("read failed")
	}

	_, err := DiscoverAIConfigFiles(t.TempDir())

	if err == nil {
		t.Fatal("err = nil, want error")
	}
	assertContains(t, err.Error(), "read failed")
}

func TestParseJSONFilesReturnReadAndParseErrors(t *testing.T) {
	_, err := ParseAISettingsFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("err = nil, want read error")
	}
	assertContains(t, err.Error(), "read")

	path := writeTestFile(t, filepath.Join(t.TempDir(), "AISettings.json"), `{"m_Version":20}{"extra":true}`)
	_, err = ParseAISettingsFile(path)
	if err == nil {
		t.Fatal("err = nil, want trailing JSON error")
	}
	assertContains(t, err.Error(), "expected one JSON document")

	path = writeTestFile(t, filepath.Join(t.TempDir(), "AISettings.json"), `{"m_Version":20,"Unknown":true}`)
	_, err = ParseAISettingsFile(path)
	if err == nil {
		t.Fatal("err = nil, want unknown field error")
	}
	assertContains(t, err.Error(), "unknown field")
}

func TestCustomJSONTypeErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "bool",
			content: `{"m_Version":20,"AccuracyMin":0,"AccuracyMax":0,"ThreatDistanceLimit":0,"NoiseInvestigationDistanceLimit":0,"MaxFlankingDistance":0,"EnableFlankingOutsideCombat":2,"DamageMultiplier":0,"DamageReceivedMultiplier":0,"ShoryukenChance":0,"ShoryukenDamageMultiplier":0,"Admins":[],"Vaulting":0,"SniperProneDistanceThreshold":0,"AggressionTimeout":0,"GuardAggressionTimeout":0,"Manners":0,"MemeLevel":0,"CanRecruitFriendly":0,"CanRecruitGuards":0,"MaxRecruitableAI":0,"PreventClimb":[],"FormationScale":0,"PlayerFactions":[],"LogAIHitBy":0,"LogAIKilled":0,"EnableZombieVehicleAttackHandler":0,"EnableZombieVehicleAttackPhysics":0,"OverrideClientWeaponFiring":0,"RecreateWeaponNetworkRepresentation":0,"LightingConfigMinNightVisibilityMeters":{}}`,
			want:    "expected boolean or 0/1",
		},
		{
			name:    "inherit bool type",
			content: `{"m_Version":32,"Enabled":1,"FormationScale":0,"DespawnTime":0,"RespawnTime":0,"MinDistRadius":0,"MaxDistRadius":0,"DespawnRadius":0,"AccuracyMin":0,"AccuracyMax":0,"ThreatDistanceLimit":0,"NoiseInvestigationDistanceLimit":0,"MaxFlankingDistance":0,"EnableFlankingOutsideCombat":true,"DamageMultiplier":0,"DamageReceivedMultiplier":0,"ShoryukenChance":0,"ShoryukenDamageMultiplier":0,"LoadBalancingCategories":{},"Patrols":[]}`,
			want:    "cannot unmarshal bool",
		},
		{
			name:    "inherit bool value",
			content: `{"m_Version":32,"Enabled":1,"FormationScale":0,"DespawnTime":0,"RespawnTime":0,"MinDistRadius":0,"MaxDistRadius":0,"DespawnRadius":0,"AccuracyMin":0,"AccuracyMax":0,"ThreatDistanceLimit":0,"NoiseInvestigationDistanceLimit":0,"MaxFlankingDistance":0,"EnableFlankingOutsideCombat":2,"DamageMultiplier":0,"DamageReceivedMultiplier":0,"ShoryukenChance":0,"ShoryukenDamageMultiplier":0,"LoadBalancingCategories":{},"Patrols":[]}`,
			want:    "expected -1, 0, or 1",
		},
		{
			name:    "vector length",
			content: `{"m_Version":4,"RoamingLocations":[{"Name":"A","Position":[1,2],"Radius":1,"Type":"","Enabled":1}],"ExcludedRoamingBuildings":[],"NoGoAreas":[]}`,
			want:    "vector must contain exactly 3 numbers",
		},
		{
			name:    "vector type",
			content: `{"m_Version":4,"RoamingLocations":[{"Name":"A","Position":"1 2 3","Radius":1,"Type":"","Enabled":1}],"ExcludedRoamingBuildings":[],"NoGoAreas":[]}`,
			want:    "cannot unmarshal string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, filepath.Join(t.TempDir(), "settings.json"), test.content)
			var err error
			if strings.Contains(test.name, "vector") {
				_, err = ParseAILocationSettingsFile(path)
			} else if strings.Contains(test.name, "inherit") {
				_, err = ParseAIPatrolSettingsFile(path)
			} else {
				_, err = ParseAISettingsFile(path)
			}
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestValidateAISettingsReportsDocumentedRangeErrors(t *testing.T) {
	settings := AISettings{
		AccuracyMin:                      2,
		AccuracyMax:                      1,
		ThreatDistanceLimit:              -1,
		NoiseInvestigationDistanceLimit:  -1,
		MaxFlankingDistance:              -1,
		DamageMultiplier:                 -1,
		DamageReceivedMultiplier:         -1,
		ShoryukenChance:                  2,
		ShoryukenDamageMultiplier:        -1,
		SniperProneDistanceThreshold:     -1,
		AggressionTimeout:                -1,
		GuardAggressionTimeout:           -1,
		MaxRecruitableAI:                 300,
		FormationScale:                   -1,
		LightingConfigMinNightVisibility: map[int]float64{0: 0, 1: 1001},
	}

	err := ValidateAISettings(settings)

	if err == nil {
		t.Fatal("err = nil, want validation errors")
	}
	assertContains(t, err.Error(), "AccuracyMin must be between")
	assertContains(t, err.Error(), "MaxRecruitableAI must be between 0 and 255")
	assertContains(t, err.Error(), "LightingConfigMinNightVisibilityMeters[0]")
}

func TestValidateAIPatrolSettingsReportsDocumentedErrors(t *testing.T) {
	settings := AIPatrolSettings{
		FormationScale:                  -2,
		DespawnTime:                     -2,
		RespawnTime:                     -2,
		MinDistRadius:                   -2,
		MaxDistRadius:                   -2,
		DespawnRadius:                   -1,
		AccuracyMin:                     2,
		AccuracyMax:                     1,
		ThreatDistanceLimit:             -2,
		NoiseInvestigationDistanceLimit: -2,
		MaxFlankingDistance:             -2,
		EnableFlankingOutsideCombat:     -1,
		DamageMultiplier:                -2,
		DamageReceivedMultiplier:        -2,
		ShoryukenChance:                 2,
		ShoryukenDamageMultiplier:       -1,
		LoadBalancingCategories: map[string][]LoadBalancingRule{
			"empty": {},
			"bad":   {{MinPlayers: -1, MaxPlayers: -2, MaxPatrols: -2}},
		},
		Patrols: []AIPatrol{
			{
				Faction:                         "Bad",
				Formation:                       "Bad",
				FormationScale:                  -2,
				FormationLooseness:              -1,
				Loadout:                         "Missing.json",
				NumberOfAI:                      3,
				NumberOfAIMax:                   2,
				Behaviour:                       "Bad",
				LootingBehaviour:                "DEFAULT | Bad",
				Speed:                           "Bad",
				UnderThreatSpeed:                "Bad",
				DefaultStance:                   "Bad",
				DefaultLookAngle:                181,
				UnlimitedReload:                 3,
				SniperProneDistanceThreshold:    -1,
				AccuracyMin:                     2,
				AccuracyMax:                     1,
				ThreatDistanceLimit:             -2,
				NoiseInvestigationDistanceLimit: -2,
				MaxFlankingDistance:             -2,
				DamageMultiplier:                -2,
				DamageReceivedMultiplier:        -2,
				HeadshotResistance:              -1,
				ShoryukenChance:                 2,
				ShoryukenDamageMultiplier:       -2,
				MinDistRadius:                   -2,
				MaxDistRadius:                   -2,
				DespawnRadius:                   -2,
				MinSpreadRadius:                 2,
				MaxSpreadRadius:                 1,
				Chance:                          2,
				DespawnTime:                     -2,
				RespawnTime:                     -3,
				LoadBalancingCategory:           "Missing",
				LootDropOnDeath:                 "Missing.json",
				WaypointInterpolation:           "Bad",
			},
			{
				Faction:               "West",
				Formation:             "",
				Loadout:               "Missing",
				Behaviour:             "HALT",
				LootingBehaviour:      "",
				Speed:                 "WALK",
				UnderThreatSpeed:      "SPRINT",
				DefaultStance:         "",
				RespawnTime:           0,
				WaypointInterpolation: "",
			},
		},
	}

	err := ValidateAIPatrolSettings(settings, validateContext{
		loadoutNames: map[string]string{"Existing": "Existing.json"},
		lootDrops:    map[string]string{"Existing.json": "Existing.json"},
	})

	if err == nil {
		t.Fatal("err = nil, want validation errors")
	}
	assertContains(t, err.Error(), `Patrols[0].Faction has invalid value "Bad"`)
	assertContains(t, err.Error(), "UnlimitedReload")
	assertContains(t, err.Error(), "LoadBalancingCategories")
	assertContains(t, err.Error(), `Patrols[1].Loadout references missing loadout "Missing"`)
}

func TestValidateAIPatrolSettingsAllowsEmptyLoadoutWithoutContext(t *testing.T) {
	err := ValidateAIPatrolSettings(AIPatrolSettings{
		Patrols: []AIPatrol{{
			Faction:          "West",
			Behaviour:        "HALT",
			Speed:            "WALK",
			UnderThreatSpeed: "SPRINT",
		}},
	}, validateContext{})

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestValidateAILocationSettingsReportsDocumentedErrors(t *testing.T) {
	err := ValidateAILocationSettings(AILocationSettings{
		RoamingLocations: []RoamingLocation{{Radius: -1}},
		NoGoAreas:        []NoGoArea{{Radius: -1, Height: -1}},
	})

	if err == nil {
		t.Fatal("err = nil, want validation errors")
	}
	assertContains(t, err.Error(), "RoamingLocations[0].Radius")
	assertContains(t, err.Error(), "NoGoAreas[0].Height")
}

func TestValidateLoadoutAndLootDropReportDocumentedErrors(t *testing.T) {
	loadout := PrefabObject{
		Include:  "Self",
		Chance:   2,
		Quantity: MinMax{Min: 2, Max: 1},
		Health:   []Health{{Min: -1, Max: 2}},
		InventoryCargo: []PrefabObject{
			{Include: "Missing", Chance: 1},
		},
	}

	err := ValidateLoadout(loadout, "Self", validateContext{loadoutNames: map[string]string{"Other": "Other.json"}})

	if err == nil {
		t.Fatal("err = nil, want validation errors")
	}
	assertContains(t, err.Error(), "root.Include cannot include its own loadout")
	assertContains(t, err.Error(), "root.Quantity.Min must be less than")
	assertContains(t, err.Error(), `root.InventoryCargo[0].Include references missing loadout "Missing"`)

	err = ValidateLootDrop([]PrefabObject{{Chance: 2}}, validateContext{})
	if err == nil {
		t.Fatal("err = nil, want loot drop validation error")
	}
	assertContains(t, err.Error(), "entries[0].Chance")
}

func TestValidateLoadoutAllowsEmptyContext(t *testing.T) {
	err := ValidateLoadout(PrefabObject{Include: "Missing", Chance: 1}, "", validateContext{})

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestValidateAIFileReturnsUnsupportedKindError(t *testing.T) {
	_, err := validateAIFile(AIFile{Path: "x", Kind: "unknown"}, validateContext{})

	if err == nil {
		t.Fatal("err = nil, want error")
	}
	assertContains(t, err.Error(), "unsupported")
}

func TestValidateAIFileReturnsParseErrorsForEachKind(t *testing.T) {
	for _, kind := range []string{KindAISettings, KindAIPatrols, KindAILocations, KindAILoadout, KindAILootDrop} {
		t.Run(kind, func(t *testing.T) {
			_, err := validateAIFile(AIFile{Path: filepath.Join(t.TempDir(), "missing.json"), Kind: kind}, validateContext{})
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), "read")
		})
	}
}

func assertStatus(t *testing.T, statuses []AIFileStatus, kind string, base string, summary string) {
	t.Helper()
	for _, status := range statuses {
		if status.Kind == kind && filepath.Base(status.Path) == base {
			assertEqual(t, status.Summary, summary)
			return
		}
	}
	t.Fatalf("status %s %s not found", kind, base)
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

func writeTestFile(t *testing.T, path string, content string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
	return path
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertEqual[T comparable](t *testing.T, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
