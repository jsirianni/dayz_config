package expansion

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPatrolFileMutationsCreateUpdateDelete(t *testing.T) {
	root := makeExpansionAITree(t)
	patrolsPath := filepath.Join(root, "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")

	setUnits := []string{"UnitA"}
	setWaypoints := []Vector{{1, 0, 1}, {2, 0, 2}}
	create, err := CreatePatrolFile(patrolsPath, PatrolPatch{
		Strings:         map[string]string{"Name": "Created", "Faction": "East", "Loadout": "HumanLoadout", "Behaviour": "HALT", "Speed": "WALK", "UnderThreatSpeed": "SPRINT"},
		Floats:          map[string]float64{"Chance": 0.5, "MinSpreadRadius": 1, "MaxSpreadRadius": 2},
		Ints:            map[string]int{"NumberOfAI": 2, "NumberOfAIMax": 3, "UnlimitedReload": 2},
		BoolInts:        map[string]int{"Persist": 1, "CanBeLooted": 0},
		InheritBools:    map[string]int{"EnableFlankingOutsideCombat": 0},
		SetUnits:        &setUnits,
		AddUnits:        []string{"UnitB"},
		RemoveUnits:     []string{"UnitA"},
		SetWaypoints:    &setWaypoints,
		UpdateWaypoints: map[int]Vector{2: {3, 0, 3}},
		RemoveWaypoints: []int{1, 1},
		AddWaypoints:    []Vector{{4, 0, 4}},
	})
	if err != nil {
		t.Fatalf("CreatePatrolFile returned error: %v", err)
	}
	if !create.Changed || !strings.Contains(string(create.Data), `"Name": "Created"`) {
		t.Fatalf("create mutation was not applied: changed=%v data=%s", create.Changed, string(create.Data))
	}
	if err := WriteFileMutation(patrolsPath, create); err != nil {
		t.Fatalf("write create mutation: %v", err)
	}

	update, err := UpdatePatrolFile(patrolsPath, PatrolUpdateOptions{
		Selector: PatrolSelector{Name: "Duplicate", Occurrence: 2},
		Patch: PatrolPatch{
			Strings:        map[string]string{"Name": "Renamed", "Faction": "West", "Loadout": "HumanLoadout", "Behaviour": "LOOP", "Speed": "JOG", "UnderThreatSpeed": "SPRINT"},
			Floats:         map[string]float64{"Chance": 1, "MinSpreadRadius": 0, "MaxSpreadRadius": 0},
			BoolInts:       map[string]int{"UseRandomWaypointAsStartPoint": 1},
			InheritBools:   map[string]int{"EnableFlankingOutsideCombat": -1},
			ClearUnits:     true,
			ClearWaypoints: true,
			AddWaypoints:   []Vector{{8, 0, 8}},
		},
	})
	if err != nil {
		t.Fatalf("UpdatePatrolFile returned error: %v", err)
	}
	if err := WriteFileMutation(patrolsPath, update); err != nil {
		t.Fatalf("write update mutation: %v", err)
	}

	noChange, err := UpdatePatrolFile(patrolsPath, PatrolUpdateOptions{Selector: PatrolSelector{Index: 1}})
	if err != nil {
		t.Fatalf("no-change update returned error: %v", err)
	}
	if noChange.Changed {
		t.Fatal("no-change update changed the file")
	}

	deleted, err := DeletePatrolFile(patrolsPath, PatrolSelector{Name: "Duplicate"})
	if err != nil {
		t.Fatalf("DeletePatrolFile returned error: %v", err)
	}
	if !deleted.Changed {
		t.Fatal("delete mutation did not change the file")
	}
}

func TestPatrolMutationErrors(t *testing.T) {
	root := makeExpansionAITree(t)
	patrolsPath := filepath.Join(root, "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")
	badJSONPath := writeTestFile(t, filepath.Join(root, "empty.deerisle", "expansion", "settings", "bad.json"), `{"m_Version":32`)

	if _, err := CreatePatrolFile(filepath.Join(root, "missing.json"), PatrolPatch{}); err == nil {
		t.Fatal("CreatePatrolFile missing path err = nil")
	}
	if _, err := CreatePatrolFile(badJSONPath, PatrolPatch{}); err == nil {
		t.Fatal("CreatePatrolFile malformed JSON err = nil")
	}
	if _, err := CreatePatrolFile(patrolsPath, PatrolPatch{BoolInts: map[string]int{"Persist": 2}}); err == nil {
		t.Fatal("CreatePatrolFile invalid patch err = nil")
	}
	if _, err := UpdatePatrolFile(filepath.Join(root, "missing.json"), PatrolUpdateOptions{}); err == nil {
		t.Fatal("UpdatePatrolFile missing path err = nil")
	}
	if _, err := UpdatePatrolFile(badJSONPath, PatrolUpdateOptions{}); err == nil {
		t.Fatal("UpdatePatrolFile malformed JSON err = nil")
	}
	invalidExistingPath := filepath.Join(root, "empty.deerisle", "expansion", "settings", "invalid_existing.json")
	invalidExisting := validMutationPatrolSettings()
	invalidExisting.Patrols[0].Faction = "Bad"
	writeJSONForMutationTest(t, invalidExistingPath, invalidExisting)
	if _, err := UpdatePatrolFile(invalidExistingPath, PatrolUpdateOptions{Selector: PatrolSelector{Index: 1}}); err == nil {
		t.Fatal("UpdatePatrolFile invalid existing settings err = nil")
	}
	if _, err := UpdatePatrolFile(patrolsPath, PatrolUpdateOptions{Selector: PatrolSelector{Index: 1}, Patch: PatrolPatch{BoolInts: map[string]int{"Persist": 2}}}); err == nil {
		t.Fatal("UpdatePatrolFile invalid patch err = nil")
	}
	if _, err := DeletePatrolFile(filepath.Join(root, "missing.json"), PatrolSelector{}); err == nil {
		t.Fatal("DeletePatrolFile missing path err = nil")
	}
	if _, err := DeletePatrolFile(badJSONPath, PatrolSelector{}); err == nil {
		t.Fatal("DeletePatrolFile malformed JSON err = nil")
	}
	if _, err := UpdatePatrolFile(patrolsPath, PatrolUpdateOptions{Selector: PatrolSelector{Name: "Alpha"}, Patch: PatrolPatch{Floats: map[string]float64{"Chance": 2}}}); err == nil {
		t.Fatal("UpdatePatrolFile invalid output err = nil")
	}
	if _, err := UpdatePatrolFile(patrolsPath, PatrolUpdateOptions{Selector: PatrolSelector{Name: "Duplicate"}}); err == nil {
		t.Fatal("duplicate name err = nil")
	}
	if _, err := DeletePatrolFile(patrolsPath, PatrolSelector{Index: 1, Name: "Alpha"}); err == nil {
		t.Fatal("mixed selector err = nil")
	}
	if _, err := DeletePatrolFile(patrolsPath, PatrolSelector{}); err == nil {
		t.Fatal("missing selector err = nil")
	}
	if _, err := DeletePatrolFile(patrolsPath, PatrolSelector{Index: 99}); err == nil {
		t.Fatal("missing index err = nil")
	}
	if _, err := DeletePatrolFile(patrolsPath, PatrolSelector{Name: "Missing"}); err == nil {
		t.Fatal("missing name err = nil")
	}
	if _, err := DeletePatrolFile(patrolsPath, PatrolSelector{Name: "Duplicate", Occurrence: 3}); err == nil {
		t.Fatal("missing occurrence err = nil")
	}

	patrol := DefaultPatrol()
	for _, patch := range []PatrolPatch{
		{Strings: map[string]string{"Chance": "bad"}},
		{Floats: map[string]float64{"Faction": 1}},
		{Ints: map[string]int{"Faction": 1}},
		{BoolInts: map[string]int{"Persist": 2}},
		{BoolInts: map[string]int{"Faction": 1}},
		{InheritBools: map[string]int{"EnableFlankingOutsideCombat": 2}},
		{InheritBools: map[string]int{"Faction": -1}},
		{SetWaypoints: &[]Vector{{1, 2, 3}}, UpdateWaypoints: map[int]Vector{2: {1, 2, 3}}},
		{SetWaypoints: &[]Vector{{1, 2, 3}}, RemoveWaypoints: []int{2}},
	} {
		if err := ApplyPatrolPatch(&patrol, patch); err == nil {
			t.Fatalf("ApplyPatrolPatch(%#v) err = nil", patch)
		}
	}
}

func TestLoadoutFileMutationsCreateUpdateItemsAndDelete(t *testing.T) {
	root := makeExpansionAITree(t)
	loadoutPath := filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "NewLoadout.json")
	quantityMin := 0.25
	quantityMax := 0.75
	parts := []string{"PartA"}

	create, err := CreateLoadoutFile(loadoutPath, PrefabObjectPatch{
		Strings:           map[string]string{"ClassName": "SurvivorM_Mirek"},
		Floats:            map[string]float64{"Chance": 1},
		QuantityMin:       &quantityMin,
		QuantityMax:       &quantityMax,
		SetHealth:         map[string]MinMax{"": {Min: 0.7, Max: 1}},
		ConstructionParts: StringListPatch{Set: &parts, Add: []string{"PartB"}, Remove: []string{"PartA"}},
	})
	if err != nil {
		t.Fatalf("CreateLoadoutFile returned error: %v", err)
	}
	if err := WriteFileMutation(loadoutPath, create); err != nil {
		t.Fatalf("write loadout: %v", err)
	}

	rootUpdate, err := UpdateLoadoutFile(loadoutPath, PrefabObjectPatch{
		SetHealth:         map[string]MinMax{"": {Min: 0.8, Max: 1}},
		RemoveHealth:      []string{""},
		ClearHealth:       true,
		ConstructionParts: StringListPatch{Clear: true, Add: []string{"PartC"}},
	})
	if err != nil {
		t.Fatalf("UpdateLoadoutFile returned error: %v", err)
	}
	if err := WriteFileMutation(loadoutPath, rootUpdate); err != nil {
		t.Fatalf("write root update: %v", err)
	}

	addAttachment, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{
		ParentPath: "root",
		Container:  "attachment",
		Slot:       "Body",
		Patch:      PrefabObjectPatch{Strings: map[string]string{"ClassName": "TShirt_Black"}, Floats: map[string]float64{"Chance": 1}},
	})
	if err != nil {
		t.Fatalf("AddLoadoutItemFile attachment returned error: %v", err)
	}
	if err := WriteFileMutation(loadoutPath, addAttachment); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	for _, options := range []LoadoutItemAddOptions{
		{ParentPath: "root", Container: "cargo", Patch: PrefabObjectPatch{Strings: map[string]string{"ClassName": "Apple"}, Floats: map[string]float64{"Chance": 1}}},
		{ParentPath: "root", Container: "set", Patch: PrefabObjectPatch{Strings: map[string]string{"ClassName": "WEAPON"}, Floats: map[string]float64{"Chance": 1}}},
		{ParentPath: "root/attachments:Body:1", Container: "attachment", Slot: "", Patch: PrefabObjectPatch{Strings: map[string]string{"ClassName": "Battery9V"}, Floats: map[string]float64{"Chance": 1}}},
	} {
		mutation, err := AddLoadoutItemFile(loadoutPath, options)
		if err != nil {
			t.Fatalf("AddLoadoutItemFile(%#v) returned error: %v", options, err)
		}
		if err := WriteFileMutation(loadoutPath, mutation); err != nil {
			t.Fatalf("write nested item: %v", err)
		}
	}

	if _, err := UpdateLoadoutItemFile(loadoutPath, "root/attachments:Body:1/attachments::1", PrefabObjectPatch{Strings: map[string]string{"ClassName": "Battery9V"}, Floats: map[string]float64{"Chance": 0.5}}); err != nil {
		t.Fatalf("UpdateLoadoutItemFile returned error: %v", err)
	}
	for _, path := range []string{"root/cargo:1", "root/sets:1", "root/attachments:Body:1/attachments::1"} {
		mutation, err := RemoveLoadoutItemFile(loadoutPath, path)
		if err != nil {
			t.Fatalf("RemoveLoadoutItemFile(%s) returned error: %v", path, err)
		}
		if err := WriteFileMutation(loadoutPath, mutation); err != nil {
			t.Fatalf("write remove: %v", err)
		}
	}

	plan, err := PlanLoadoutDelete(loadoutPath, filepath.Join(root, "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json"))
	if err != nil {
		t.Fatalf("PlanLoadoutDelete returned error: %v", err)
	}
	assertEqual(t, plan.Exists, true)
	assertEqual(t, plan.References, 0)
	if err := DeleteLoadoutFile(loadoutPath); err != nil {
		t.Fatalf("DeleteLoadoutFile returned error: %v", err)
	}
}

func TestLoadoutMutationErrors(t *testing.T) {
	root := makeExpansionAITree(t)
	loadoutPath := filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "HumanLoadout.json")
	badPath := writeTestFile(t, filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Bad.json"), `{"Chance":2}`)

	if _, err := CreateLoadoutFile(loadoutPath, PrefabObjectPatch{}); err == nil {
		t.Fatal("CreateLoadoutFile existing path err = nil")
	}
	originalStat := statMutable
	statMutable = func(string) (os.FileInfo, error) { return nil, errors.New("stat failed") }
	if _, err := CreateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Stat.json"), PrefabObjectPatch{}); err == nil {
		t.Fatal("CreateLoadoutFile stat error = nil")
	}
	statMutable = originalStat
	if _, err := CreateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Patch.json"), PrefabObjectPatch{Strings: map[string]string{"Chance": "bad"}}); err == nil {
		t.Fatal("CreateLoadoutFile invalid patch err = nil")
	}
	if _, err := CreateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Invalid.json"), PrefabObjectPatch{Strings: map[string]string{"Include": "Invalid"}}); err == nil {
		t.Fatal("CreateLoadoutFile self include err = nil")
	}
	if _, err := CreateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Invalid.json"), PrefabObjectPatch{Floats: map[string]float64{"Chance": 2}}); err == nil {
		t.Fatal("CreateLoadoutFile invalid output err = nil")
	}
	if _, err := UpdateLoadoutFile(filepath.Join(root, "missing.json"), PrefabObjectPatch{}); err == nil {
		t.Fatal("UpdateLoadoutFile missing path err = nil")
	}
	if _, err := UpdateLoadoutFile(writeTestFile(t, filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "Malformed.json"), `{"ClassName":`), PrefabObjectPatch{}); err == nil {
		t.Fatal("UpdateLoadoutFile malformed JSON err = nil")
	}
	if _, err := UpdateLoadoutFile(badPath, PrefabObjectPatch{}); err == nil {
		t.Fatal("UpdateLoadoutFile invalid existing loadout err = nil")
	}
	if _, err := UpdateLoadoutFile(loadoutPath, PrefabObjectPatch{Strings: map[string]string{"Chance": "bad"}}); err == nil {
		t.Fatal("UpdateLoadoutFile invalid field err = nil")
	}
	if _, err := UpdateLoadoutFile(loadoutPath, PrefabObjectPatch{Floats: map[string]float64{"ClassName": 1}}); err == nil {
		t.Fatal("UpdateLoadoutFile invalid float field err = nil")
	}
	min := 2.0
	max := 1.0
	if _, err := UpdateLoadoutFile(loadoutPath, PrefabObjectPatch{QuantityMin: &min, QuantityMax: &max}); err == nil {
		t.Fatal("UpdateLoadoutFile invalid output err = nil")
	}
	noChange, err := UpdateLoadoutFile(loadoutPath, PrefabObjectPatch{})
	if err != nil {
		t.Fatalf("UpdateLoadoutFile no-change returned error: %v", err)
	}
	if noChange.Changed {
		t.Fatal("UpdateLoadoutFile no-change changed data")
	}
	if _, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{ParentPath: "bad", Container: "cargo"}); err == nil {
		t.Fatal("AddLoadoutItemFile invalid parent err = nil")
	}
	if _, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{ParentPath: "root", Container: "cargo", Slot: "Body"}); err == nil {
		t.Fatal("AddLoadoutItemFile cargo slot err = nil")
	}
	if _, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{ParentPath: "root", Container: "bad"}); err == nil {
		t.Fatal("AddLoadoutItemFile invalid container err = nil")
	}
	if _, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{ParentPath: "root", Container: "set", Slot: "Body"}); err == nil {
		t.Fatal("AddLoadoutItemFile set slot err = nil")
	}
	if _, err := AddLoadoutItemFile(loadoutPath, LoadoutItemAddOptions{ParentPath: "root", Container: "cargo", Patch: PrefabObjectPatch{Strings: map[string]string{"Chance": "bad"}}}); err == nil {
		t.Fatal("AddLoadoutItemFile invalid patch err = nil")
	}
	if _, err := UpdateLoadoutItemFile(loadoutPath, "root/cargo:9", PrefabObjectPatch{}); err == nil {
		t.Fatal("UpdateLoadoutItemFile missing cargo err = nil")
	}
	for _, path := range []string{"root", "bad", "root/cargo:x", "root/cargo:9", "root/cargo:9/cargo:1", "root/sets:x", "root/sets:9", "root/attachments:Body:x", "root/attachments:Body:9", "root/bad:1"} {
		if _, err := RemoveLoadoutItemFile(loadoutPath, path); err == nil {
			t.Fatalf("RemoveLoadoutItemFile(%s) err = nil", path)
		}
	}
	if _, err := PlanLoadoutDelete(filepath.Join(root, "missing.json"), ""); err == nil {
		t.Fatal("PlanLoadoutDelete missing loadout err = nil")
	}
	if _, err := PlanLoadoutDelete(loadoutPath, badPath); err == nil {
		t.Fatal("PlanLoadoutDelete bad patrols file err = nil")
	}
}

func TestMutationHelpersAndErrorStubs(t *testing.T) {
	boolData, err := boolInt(true).MarshalJSON()
	if err != nil || string(boolData) != "1" {
		t.Fatalf("boolInt true marshal = %q, %v", string(boolData), err)
	}
	boolData, err = boolInt(false).MarshalJSON()
	if err != nil || string(boolData) != "0" {
		t.Fatalf("boolInt false marshal = %q, %v", string(boolData), err)
	}
	inheritData, err := inheritBool(-1).MarshalJSON()
	if err != nil || string(inheritData) != "-1" {
		t.Fatalf("inheritBool marshal = %q, %v", string(inheritData), err)
	}
	if _, err := renderJSON(nil, func() {}); err == nil {
		t.Fatal("renderJSON unsupported value err = nil")
	}
	if detectJSONIndent([]byte("{\r\n\t\"x\": true\r\n}")) != "\t" {
		t.Fatal("detectJSONIndent did not detect tab indentation")
	}
	if detectJSONLineEnding([]byte("{\r\n}")) != "\r\n" {
		t.Fatal("detectJSONLineEnding did not detect CRLF")
	}
	renderedCRLF, err := renderJSON([]byte("{\r\n    \"x\": true\r\n}"), DefaultPrefabObject())
	if err != nil {
		t.Fatalf("renderJSON CRLF returned error: %v", err)
	}
	if !strings.Contains(string(renderedCRLF), "\r\n") {
		t.Fatal("renderJSON did not preserve CRLF")
	}
	if !reflect.DeepEqual(sortedDescendingUniqueInts([]int{2, 1, 2}), []int{2, 1}) {
		t.Fatal("sortedDescendingUniqueInts did not sort and de-duplicate")
	}
	loosePath := writeTestFile(t, filepath.Join(t.TempDir(), "Loose.json"), `{"ClassName":"","Include":"","Chance":1,"Quantity":{"Min":0,"Max":0},"Health":[],"InventoryAttachments":[],"InventoryCargo":[],"ConstructionPartsBuilt":[],"Sets":[]}`)
	if _, err := PlanLoadoutDelete(loosePath, ""); err != nil {
		t.Fatalf("PlanLoadoutDelete without nearby context returned error: %v", err)
	}
	if validationContextNear(filepath.Join(t.TempDir(), "missing.json")).loadoutNames != nil {
		t.Fatal("validationContextNear without config returned non-empty context")
	}
	context := validationContextFromFiles([]AIFile{{Path: "A.json", Kind: KindAILoadout}, {Path: "Drop.json", Kind: KindAILootDrop}})
	if _, ok := context.lootDrops["Drop.json"]; !ok {
		t.Fatal("validationContextFromFiles did not record loot drop")
	}
	onlyLoadouts := t.TempDir()
	writeJSONForMutationTest(t, filepath.Join(onlyLoadouts, "profiles", "ExpansionMod", "Loadouts", "Only.json"), DefaultPrefabObject())
	if discoverPatrolsFileNear(filepath.Join(onlyLoadouts, "profiles", "ExpansionMod", "Loadouts", "Only.json")) != "" {
		t.Fatal("discoverPatrolsFileNear found patrols where none exist")
	}
	root := makeExpansionAITree(t)
	references, err := countLoadoutReferences(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "HumanLoadout.json"), "")
	if err != nil {
		t.Fatalf("countLoadoutReferences returned error: %v", err)
	}
	assertEqual(t, references, 3)
	values := []string{"keep", "remove"}
	if got := removeStringValues(values, []string{"remove"}); !reflect.DeepEqual(got, []string{"keep"}) {
		t.Fatalf("removeStringValues = %#v", got)
	}
	var nilStrings []string
	applyStringListPatch(&nilStrings, StringListPatch{})
	if nilStrings == nil {
		t.Fatal("applyStringListPatch did not normalize nil slice")
	}
	patrol := AIPatrol{}
	if err := ApplyPatrolPatch(&patrol, PatrolPatch{}); err != nil {
		t.Fatalf("ApplyPatrolPatch empty returned error: %v", err)
	}
	if patrol.Waypoints == nil {
		t.Fatal("ApplyPatrolPatch did not normalize nil waypoints")
	}
	pathRoot := DefaultPrefabObject()
	pathRoot.InventoryCargo = []PrefabObject{{ClassName: "Cargo", Chance: 1}}
	pathRoot.Sets = []PrefabObject{{ClassName: "Set", Chance: 1}}
	pathRoot.InventoryAttachments = []PrefabSlot{{SlotName: "Body", Items: []PrefabObject{{ClassName: "Body", Chance: 1}}}}
	for _, path := range []string{"root/cargo:1", "root/sets:1", "root/attachments:Body:1"} {
		if _, err := ResolvePrefabObjectPath(&pathRoot, path); err != nil {
			t.Fatalf("ResolvePrefabObjectPath(%s) returned error: %v", path, err)
		}
	}
	for _, path := range []string{"root/cargo:x", "root/sets:x", "root/sets:9", "root/attachments:Body:x", "root/attachments:Missing:1", "root/bad:1"} {
		if _, err := ResolvePrefabObjectPath(&pathRoot, path); err == nil {
			t.Fatalf("ResolvePrefabObjectPath(%s) err = nil", path)
		}
	}
	object := PrefabObject{Health: []Health{{Zone: "Body", Min: 0.1, Max: 0.2}}}
	_ = ApplyPrefabObjectPatch(&object, PrefabObjectPatch{SetHealth: map[string]MinMax{"Body": {Min: 0.3, Max: 0.4}}, RemoveHealth: []string{"Other"}})
	if object.Health[0].Min != 0.3 {
		t.Fatal("ApplyPrefabObjectPatch did not update existing health")
	}
	settings := AIPatrolSettings{Patrols: []AIPatrol{{}}}
	NormalizeAIPatrolSettings(&settings)
	if settings.LoadBalancingCategories == nil || settings.Patrols[0].Units == nil || settings.Patrols[0].Waypoints == nil {
		t.Fatal("NormalizeAIPatrolSettings did not initialize nil collections")
	}
	emptySettings := AIPatrolSettings{}
	NormalizeAIPatrolSettings(&emptySettings)
	if emptySettings.Patrols == nil {
		t.Fatal("NormalizeAIPatrolSettings did not initialize nil patrols")
	}
	normalizeObject := PrefabObject{InventoryAttachments: []PrefabSlot{{}, {Items: []PrefabObject{{}}}}, InventoryCargo: []PrefabObject{{}}, Sets: []PrefabObject{{}}}
	NormalizePrefabObject(&normalizeObject)
	if normalizeObject.Health == nil || normalizeObject.InventoryAttachments[0].Items == nil || normalizeObject.InventoryAttachments[1].Items[0].Health == nil || normalizeObject.InventoryCargo[0].Health == nil || normalizeObject.Sets[0].Health == nil {
		t.Fatal("NormalizePrefabObject did not initialize nested collections")
	}

	originalRead := readMutable
	originalWrite := writeMutable
	originalRemove := removeMutable
	originalMakeDir := makeDirAllMutable
	originalRender := renderJSONForMutation
	defer func() {
		readMutable = originalRead
		writeMutable = originalWrite
		removeMutable = originalRemove
		makeDirAllMutable = originalMakeDir
		renderJSONForMutation = originalRender
	}()

	path := writeTestFile(t, filepath.Join(t.TempDir(), "file.json"), "{}")
	readMutable = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	if _, _, err := readMutableFile(path); err == nil {
		t.Fatal("readMutableFile read error = nil")
	}
	readMutable = originalRead

	makeDirAllMutable = func(string, os.FileMode) error { return errors.New("mkdir failed") }
	if err := WriteFileMutation(filepath.Join(t.TempDir(), "x", "file.json"), FileMutation{Data: []byte("{}"), Mode: 0o600, Changed: true}); err == nil {
		t.Fatal("WriteFileMutation mkdir error = nil")
	}
	makeDirAllMutable = originalMakeDir

	writeMutable = func(string, []byte, os.FileMode) error { return errors.New("write failed") }
	if err := WriteFileMutation(path, FileMutation{Data: []byte("{}"), Mode: 0o600, Changed: true}); err == nil {
		t.Fatal("WriteFileMutation write error = nil")
	}
	writeMutable = originalWrite
	if err := WriteFileMutation(path, FileMutation{Changed: false}); err != nil {
		t.Fatalf("WriteFileMutation no-change returned error: %v", err)
	}

	removeMutable = func(string) error { return errors.New("remove failed") }
	if err := DeleteLoadoutFile(path); err == nil {
		t.Fatal("DeleteLoadoutFile remove error = nil")
	}
	removeMutable = originalRemove

	renderJSONForMutation = func([]byte, any) ([]byte, error) { return nil, errors.New("render failed") }
	if _, err := CreateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "RenderCreate.json"), PrefabObjectPatch{}); err == nil {
		t.Fatal("CreateLoadoutFile render error = nil")
	}
	if _, err := UpdatePatrolFile(filepath.Join(root, "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json"), PatrolUpdateOptions{Selector: PatrolSelector{Index: 1}, Patch: PatrolPatch{Strings: map[string]string{"Name": "Render"}}}); err == nil {
		t.Fatal("UpdatePatrolFile render error = nil")
	}
	if _, err := UpdateLoadoutFile(filepath.Join(root, "profiles", "ExpansionMod", "Loadouts", "HumanLoadout.json"), PrefabObjectPatch{Strings: map[string]string{"ClassName": "Render"}}); err == nil {
		t.Fatal("UpdateLoadoutFile render error = nil")
	}
}

func makeExpansionAITree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	patrolsPath := filepath.Join(root, "empty.deerisle", "expansion", "settings", "AIPatrolSettings.json")
	loadoutDir := filepath.Join(root, "profiles", "ExpansionMod", "Loadouts")
	writeJSONForMutationTest(t, filepath.Join(loadoutDir, "HumanLoadout.json"), DefaultPrefabObject())
	writeJSONForMutationTest(t, patrolsPath, validMutationPatrolSettings())
	return root
}

func validMutationPatrolSettings() AIPatrolSettings {
	alpha := DefaultPatrol()
	alpha.Name = "Alpha"
	alpha.LoadBalancingCategory = "Patrol"
	alpha.Waypoints = []Vector{{1, 0, 1}}
	duplicateOne := DefaultPatrol()
	duplicateOne.Name = "Duplicate"
	duplicateOne.LoadBalancingCategory = "Patrol"
	duplicateOne.Waypoints = []Vector{{2, 0, 2}}
	duplicateTwo := DefaultPatrol()
	duplicateTwo.Name = "Duplicate"
	duplicateTwo.LoadBalancingCategory = "Patrol"
	duplicateTwo.Waypoints = []Vector{{3, 0, 3}}
	return AIPatrolSettings{
		Version:                         32,
		Enabled:                         boolInt(true),
		FormationScale:                  -1,
		DespawnTime:                     600,
		RespawnTime:                     600,
		MinDistRadius:                   400,
		MaxDistRadius:                   1000,
		DespawnRadius:                   1100,
		AccuracyMin:                     -1,
		AccuracyMax:                     -1,
		ThreatDistanceLimit:             -1,
		NoiseInvestigationDistanceLimit: -1,
		MaxFlankingDistance:             -1,
		EnableFlankingOutsideCombat:     inheritBool(-1),
		DamageMultiplier:                -1,
		DamageReceivedMultiplier:        -1,
		ShoryukenChance:                 0,
		ShoryukenDamageMultiplier:       0,
		LoadBalancingCategories: map[string][]LoadBalancingRule{
			"Patrol": {{MinPlayers: 0, MaxPlayers: 255, MaxPatrols: 5}},
		},
		Patrols: []AIPatrol{alpha, duplicateOne, duplicateTwo},
	}
}

func writeJSONForMutationTest(t *testing.T, path string, value any) {
	t.Helper()

	data, err := renderJSON(nil, value)
	if err != nil {
		t.Fatalf("render test JSON: %v", err)
	}
	writeTestFile(t, path, string(data))
}
