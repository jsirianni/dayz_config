package economyconfig

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInspectEconomyCoreFindsBaseAndReferencedTypesFiles(t *testing.T) {
	statuses, err := InspectEconomyCore(fixturePath(t, "mission", "cfgeconomycore.xml"))
	if err != nil {
		t.Fatalf("InspectEconomyCore returned error: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("status count = %d, want 3", len(statuses))
	}
	assertEqual(t, statuses[0].Kind, "cfgeconomycore")
	assertEqual(t, filepath.Base(statuses[0].Path), "cfgeconomycore.xml")
	if statuses[0].Err != nil {
		t.Fatalf("cfgeconomycore status err = %v, want nil", statuses[0].Err)
	}
	assertEqual(t, statuses[1].Kind, "base-types")
	assertEqual(t, filepath.Base(statuses[1].Path), "types.xml")
	assertEqual(t, statuses[1].TypeCount, 2)
	if statuses[1].Err != nil {
		t.Fatalf("base status err = %v, want nil", statuses[1].Err)
	}
	assertEqual(t, statuses[2].Kind, "types")
	assertEqual(t, filepath.Base(statuses[2].Path), "valid_types.xml")
	assertEqual(t, statuses[2].TypeCount, 1)
	if statuses[2].Err != nil {
		t.Fatalf("mod status err = %v, want nil", statuses[2].Err)
	}
}

func TestInspectEconomyCoreRecordsMissingTypesFile(t *testing.T) {
	statuses, err := InspectEconomyCore(fixturePath(t, "mission", "cfgeconomycore_missing_ref.xml"))
	if err != nil {
		t.Fatalf("InspectEconomyCore returned error: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("status count = %d, want 3", len(statuses))
	}
	if statuses[2].Err == nil {
		t.Fatal("missing referenced file err = nil, want error")
	}
	assertContains(t, statuses[2].Err.Error(), "read")
}

func TestInspectEconomyCoreReportsMissingLimitsDefinition(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	writeTestFile(t, filepath.Join(dir, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore />`)
	writeTestFile(t, filepath.Join(dir, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types />`)

	_, err := InspectEconomyCore(filepath.Join(dir, "cfgeconomycore.xml"))

	if err == nil {
		t.Fatal("err = nil, want missing limits error")
	}
	assertContains(t, err.Error(), "cfglimitsdefinition.xml")
}

func TestParseEconomyCoreFileRejectsInvalidXML(t *testing.T) {
	_, err := ParseEconomyCoreFile(fixturePath(t, "mission", "bad_cfgeconomycore.badxml"))
	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
	assertContains(t, err.Error(), "parse")
}

func TestParseEconomyCoreFileReportsReadFailure(t *testing.T) {
	_, err := ParseEconomyCoreFile(fixturePath(t, "mission", "does-not-exist.xml"))
	if err == nil {
		t.Fatal("err = nil, want read error")
	}
	assertContains(t, err.Error(), "read")
}

func TestEconomyCoreTypeFileRefsFiltersNonTypesFiles(t *testing.T) {
	core := EconomyCore{
		CEs: []CEBlock{
			{
				Folder: "folder-one",
				Files: []CEFileRef{
					{Name: "loot.xml", Type: "types"},
					{Name: "attachments.xml", Type: "spawnabletypes"},
				},
			},
			{
				Folder: "folder-two",
				Files: []CEFileRef{
					{Name: "other.xml", Type: "types"},
				},
			},
		},
	}

	refs := core.TypeFileRefs()

	if len(refs) != 2 {
		t.Fatalf("ref count = %d, want 2", len(refs))
	}
	assertEqual(t, refs[0], TypeFileRef{Folder: "folder-one", Name: "loot.xml"})
	assertEqual(t, refs[1], TypeFileRef{Folder: "folder-two", Name: "other.xml"})
}

func TestParseTypesFileReportsReadFailure(t *testing.T) {
	_, err := ParseTypesFile(fixturePath(t, "mission", "db", "missing.xml"))
	if err == nil {
		t.Fatal("err = nil, want read error")
	}
	assertContains(t, err.Error(), "read")
}

func TestParseTypesFileRejectsInvalidXML(t *testing.T) {
	_, err := ParseTypesFile(fixturePath(t, "mission", "badmods", "invalid_types.badxml"))
	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
	assertContains(t, err.Error(), "parse")
}

func TestParseTypesFileRejectsWrongRoot(t *testing.T) {
	_, err := ParseTypesFile(fixturePath(t, "mission", "cfgeconomycore.xml"))
	if err == nil {
		t.Fatal("err = nil, want root error")
	}
	assertContains(t, err.Error(), "expected <types> root")
}

func TestParseTypesFileRejectsUnnamedType(t *testing.T) {
	_, err := ParseTypesFile(fixturePath(t, "mission", "badmods", "unnamed_type.xml"))
	if err == nil {
		t.Fatal("err = nil, want missing name error")
	}
	assertContains(t, err.Error(), "missing name")
}

func TestParseTypesFileAcceptsNamedTypes(t *testing.T) {
	types, err := ParseTypesFile(fixturePath(t, "mission", "db", "types.xml"))
	if err != nil {
		t.Fatalf("ParseTypesFile returned error: %v", err)
	}

	assertEqual(t, len(types.Types), 2)
	assertEqual(t, types.Types[0].Name, "Apple")
	assertEqual(t, types.Types[1].Name, "BandageDressing")
}

func TestParseTypesFileAcceptsStandardTypeFields(t *testing.T) {
	types, err := ParseTypesFile(writeTypesFixture(t, `
<types>
  <type name="CompleteType">
    <nominal>10</nominal>
    <lifetime>7200</lifetime>
    <restock>1800</restock>
    <min>5</min>
    <quantmin>-1</quantmin>
    <quantmax>-1</quantmax>
    <cost>100</cost>
    <flags count_in_cargo="0" count_in_hoarder="0" count_in_map="1" count_in_player="0" crafted="0" deloot="0" />
    <category name="weapons" />
    <category name="tools" />
    <tag name="floor" />
    <usage name="Military" />
    <value name="Tier1" />
    <value name="CustomTier" />
  </type>
</types>`))
	if err != nil {
		t.Fatalf("ParseTypesFile returned error: %v", err)
	}

	entry := types.Types[0]
	assertEqual(t, entry.Name, "CompleteType")
	assertEqual(t, entry.Nominal, 10)
	assertEqual(t, entry.Lifetime, 7200)
	assertEqual(t, entry.Restock, 1800)
	assertEqual(t, entry.Min, 5)
	assertEqual(t, entry.QuantMin, -1)
	assertEqual(t, entry.QuantMax, -1)
	assertEqual(t, entry.Cost, 100)
	assertEqual(t, len(entry.Flags), 6)
	assertEqual(t, entry.Flags[0], FlagSetting{Name: FlagCountInCargo, Value: false})
	assertEqual(t, len(entry.Categories), 2)
	assertEqual(t, entry.Categories[0], Category("weapons"))
	assertEqual(t, len(entry.Tags), 1)
	assertEqual(t, entry.Tags[0].Name, "floor")
	assertEqual(t, len(entry.Usages), 1)
	assertEqual(t, entry.Usages[0].Name, "Military")
	assertEqual(t, len(entry.Values), 2)
	assertEqual(t, entry.Values[1].Name, "CustomTier")
}

func TestParseTypesFileRejectsUnknownTypeAttribute(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type typo="Nope" /></types>`))
	if err == nil {
		t.Fatal("err = nil, want unknown attribute error")
	}
	assertContains(t, err.Error(), "unknown attribute")
}

func TestParseTypesFileRejectsUnknownTypeField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><nomial>1</nomial></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want unknown field error")
	}
	assertContains(t, err.Error(), "unknown field <nomial>")
}

func TestParseTypesFileRejectsScalarAttributes(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><nominal typo="1">1</nominal></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want scalar attribute error")
	}
	assertContains(t, err.Error(), "<nominal> has unknown attribute")
}

func TestParseTypesFileRejectsDuplicateSingletonField(t *testing.T) {
	fields := []string{"nominal", "lifetime", "restock", "min", "quantmin", "quantmax", "cost"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><`+field+`>1</`+field+`><`+field+`>2</`+field+`></type></types>`))
			if err == nil {
				t.Fatal("err = nil, want duplicate field error")
			}
			assertContains(t, err.Error(), "duplicate <"+field+">")
		})
	}
}

func TestParseTypesFileRejectsDuplicateFlagsField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_map="1" /><flags count_in_player="0" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want duplicate flags error")
	}
	assertContains(t, err.Error(), "duplicate <flags>")
}

func TestParseTypesFileRejectsEmptyIntegerField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><nominal> </nominal></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want empty integer error")
	}
	assertContains(t, err.Error(), "expected integer value")
}

func TestParseTypesFileRejectsDecimalIntegerField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><lifetime>1.5</lifetime></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want decimal integer error")
	}
	assertContains(t, err.Error(), `got "1.5"`)
}

func TestParseTypesFileReportsMalformedIntegerFieldXML(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><lifetime></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want malformed integer field error")
	}
	assertContains(t, err.Error(), "XML syntax error")
}

func TestParseTypesFileRejectsIntegerFieldChildElement(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><nominal><child /></nominal></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want integer child element error")
	}
	assertContains(t, err.Error(), "got child <child>")
}

func TestParseTypesFileRejectsNegativeNonQuantField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><restock>-1</restock></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want range error")
	}
	assertContains(t, err.Error(), "outside allowed range")
}

func TestParseTypesFileRejectsQuantBelowAllowedRange(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><quantmin>-2</quantmin></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want quant range error")
	}
	assertContains(t, err.Error(), "outside allowed range")
}

func TestParseTypesFileRejectsQuantAboveAllowedRange(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><quantmax>101</quantmax></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want quant range error")
	}
	assertContains(t, err.Error(), "outside allowed range")
}

func TestParseTypesFileRejectsUnknownFlag(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_bag="1" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want unknown flag error")
	}
	assertContains(t, err.Error(), "unknown flag")
}

func TestParseTypesFileRejectsNonBooleanFlagValue(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_map="yes" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want flag value error")
	}
	assertContains(t, err.Error(), "expected 0 or 1")
}

func TestParseTypesFileRejectsNonEmptyFlags(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_map="1">text</flags></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want empty flags error")
	}
	assertContains(t, err.Error(), "expected empty element")
}

func TestParseTypesFileRejectsFlagsChildElement(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_map="1"><child /></flags></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want flags child element error")
	}
	assertContains(t, err.Error(), "got child <child>")
}

func TestParseTypesFileReportsMalformedEmptyElementXML(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><flags count_in_map="1"></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want malformed empty element error")
	}
	assertContains(t, err.Error(), "XML syntax error")
}

func TestParseTypesFileRejectsCategoryWithoutName(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><category /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want missing category name error")
	}
	assertContains(t, err.Error(), "expected name attribute")
}

func TestParseTypesFileRejectsCategoryUnknownAttribute(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><category typo="weapons" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want category attribute error")
	}
	assertContains(t, err.Error(), "unknown attribute")
}

func TestParseTypesFileRejectsNonEmptyNamedField(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><tag name="floor">text</tag></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want non-empty tag error")
	}
	assertContains(t, err.Error(), "expected empty element")
}

func TestParseTypesFileRejectsNamedFieldUnknownAttribute(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><usage typo="Military" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want usage attribute error")
	}
	assertContains(t, err.Error(), "unknown attribute")
}

func TestParseTypesFileRejectsValueWithoutName(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><value /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want missing value name error")
	}
	assertContains(t, err.Error(), "expected name attribute")
}

func TestParseTypesFileRejectsValueUnknownAttribute(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><value typo="Tier1" /></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want value attribute error")
	}
	assertContains(t, err.Error(), "unknown attribute")
}

func TestParseTypesFileRejectsNonEmptyValue(t *testing.T) {
	_, err := ParseTypesFile(writeTypesFixture(t, `<types><type name="Bad"><value name="Tier1">text</value></type></types>`))
	if err == nil {
		t.Fatal("err = nil, want value empty element error")
	}
	assertContains(t, err.Error(), "expected empty element")
}

func TestValidateTypesFileWarnsForLimitReferencesAndRelationships(t *testing.T) {
	path := writeTypesFixture(t, `
<types>
    <type name="Odd">
      <nominal>0</nominal>
    <restock>0</restock>
    <min>1</min>
    <quantmin>80</quantmin>
    <quantmax>30</quantmax>
    <category name="mystery" />
    <tag name="ceiling" />
    <usage name="Anywhere" />
    <value name="CustomValue" />
  </type>
</types>`)

	_, warnings, err := ValidateTypesFile(path, minimalLimits())

	if err != nil {
		t.Fatalf("ValidateTypesFile returned error: %v", err)
	}
	assertContains(t, strings.Join(warnings, "\n"), `category "mystery"`)
	assertContains(t, strings.Join(warnings, "\n"), `tag "ceiling"`)
	assertContains(t, strings.Join(warnings, "\n"), `usage "Anywhere"`)
	assertContains(t, strings.Join(warnings, "\n"), `value "CustomValue"`)
	assertContains(t, strings.Join(warnings, "\n"), "min greater than nominal")
	assertContains(t, strings.Join(warnings, "\n"), "quantmin greater than quantmax")
}

func TestValidateTypesFileReturnsParseError(t *testing.T) {
	_, _, err := ValidateTypesFile(fixturePath(t, "mission", "badmods", "invalid_types.badxml"), minimalLimits())
	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
	assertContains(t, err.Error(), "parse")
}

func TestLoadLimitsDefinitionsIncludesBaseAndUserDefinitions(t *testing.T) {
	limits, err := LoadLimitsDefinitions(fixturePath(t, "mission"))
	if err != nil {
		t.Fatalf("LoadLimitsDefinitions returned error: %v", err)
	}

	assertEqual(t, limits.Categories["tools"], true)
	assertEqual(t, limits.Tags["floor"], true)
	assertEqual(t, limits.Usages["Military"], true)
	assertEqual(t, limits.Usages["CustomUsageGroup"], true)
	assertEqual(t, limits.Values["Tier1"], true)
	assertEqual(t, limits.Values["CustomValueGroup"], true)
}

func TestLoadLimitsDefinitionsIgnoresMissingUserDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinition.xml"), validLimitsXML())

	limits, err := LoadLimitsDefinitions(dir)

	if err != nil {
		t.Fatalf("LoadLimitsDefinitions returned error: %v", err)
	}
	assertEqual(t, limits.Categories["tools"], true)
}

func TestLoadLimitsDefinitionsReportsMissingBaseDefinitions(t *testing.T) {
	_, err := LoadLimitsDefinitions(t.TempDir())
	if err == nil {
		t.Fatal("err = nil, want missing base definitions error")
	}
	assertContains(t, err.Error(), "read")
}

func TestLoadLimitsDefinitionsReportsInvalidUserDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinition.xml"), validLimitsXML())
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinitionuser.xml"), `<?xml version="1.0" encoding="UTF-8"?><not-user-lists />`)

	_, err := LoadLimitsDefinitions(dir)

	if err == nil {
		t.Fatal("err = nil, want invalid user definitions error")
	}
	assertContains(t, err.Error(), "expected <user_lists> root")
}

func TestParseLimitsDefinitionFileRejectsInvalidXML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinition.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><lists>`)

	_, err := ParseLimitsDefinitionFile(path)

	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
	assertContains(t, err.Error(), "XML syntax error")
}

func TestParseLimitsDefinitionFileRejectsWrongRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinition.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><nope />`)

	_, err := ParseLimitsDefinitionFile(path)

	if err == nil {
		t.Fatal("err = nil, want root error")
	}
	assertContains(t, err.Error(), "expected <lists> root")
}

func TestAppendUserLimitsDefinitionFileRejectsWrongRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinitionuser.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><nope />`)
	limits := minimalLimits()

	err := AppendUserLimitsDefinitionFile(path, &limits)

	if err == nil {
		t.Fatal("err = nil, want root error")
	}
	assertContains(t, err.Error(), "expected <user_lists> root")
}

func TestAppendUserLimitsDefinitionFileRejectsUnnamedUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinitionuser.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user /></usageflags></user_lists>`)
	limits := minimalLimits()

	err := AppendUserLimitsDefinitionFile(path, &limits)

	if err == nil {
		t.Fatal("err = nil, want unnamed user error")
	}
	assertContains(t, err.Error(), "expected name attribute")
}

func TestInspectEconomyCoreAddsDuplicateTypeWarnings(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinition.xml"), validLimitsXML())
	if err := os.Mkdir(filepath.Join(dir, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "mods"), 0o700); err != nil {
		t.Fatalf("mkdir mods: %v", err)
	}
	writeTestFile(t, filepath.Join(dir, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore><ce folder="mods"><file name="types.xml" type="types" /></ce></economycore>`)
	writeTestFile(t, filepath.Join(dir, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Duplicate"><nominal>1</nominal></type></types>`)
	writeTestFile(t, filepath.Join(dir, "mods", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Duplicate"><nominal>1</nominal></type></types>`)

	statuses, err := InspectEconomyCore(filepath.Join(dir, "cfgeconomycore.xml"))

	if err != nil {
		t.Fatalf("InspectEconomyCore returned error: %v", err)
	}
	assertContains(t, strings.Join(statuses[2].Warnings, "\n"), "duplicates a type")
}

func TestInspectEconomyCoreReportsSameFileDuplicateTypeWarnings(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinition.xml"), validLimitsXML())
	if err := os.Mkdir(filepath.Join(dir, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	writeTestFile(t, filepath.Join(dir, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore />`)
	writeTestFile(t, filepath.Join(dir, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Duplicate"><nominal>1</nominal></type><type name="Duplicate"><nominal>2</nominal></type></types>`)

	statuses, err := InspectEconomyCore(filepath.Join(dir, "cfgeconomycore.xml"))

	if err != nil {
		t.Fatalf("InspectEconomyCore returned error: %v", err)
	}
	assertContains(t, strings.Join(statuses[1].Warnings, "\n"), `type "Duplicate" duplicates a type already loaded from`)
}

func TestInspectEconomyValidatesAggregateMissionAndWarns(t *testing.T) {
	root := fixturePath(t, "economyconfig", "full")

	statuses, err := InspectEconomy(root)

	if err != nil {
		t.Fatalf("InspectEconomy returned error: %v", err)
	}
	kinds := statusKinds(statuses)
	for _, kind := range []string{
		"cfgeconomycore",
		"base-types",
		"types",
		"cfglimitsdefinition",
		"cfglimitsdefinitionuser",
		"events",
		"globals",
		"messages",
		"cfgeventspawns",
		"cfgrandompresets",
		"cfgspawnabletypes",
		"cfgplayerspawnpoints",
		"cfgenvironment",
		"cfgEffectArea",
		"cfgIgnoreList",
	} {
		assertContains(t, kinds, kind)
	}
	assertContains(t, allWarnings(statuses), `fixed event "MissingSpawnEvent"`)
	assertContains(t, allWarnings(statuses), `random preset "missingPreset"`)
	assertContains(t, allWarnings(statuses), `missing territory file "env/missing_territories.xml"`)
	assertContains(t, allWarnings(statuses), `usable file "unregistered_territories"`)
}

func TestInspectEconomyErrorPaths(t *testing.T) {
	if _, err := InspectEconomy(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("InspectEconomy missing path err = nil, want error")
	}
	if _, err := InspectEconomy(fixturePath(t, "economyconfig", "full", "mods", "mod_types.xml")); err == nil {
		t.Fatal("InspectEconomy non-core file err = nil, want error")
	}
	if statuses := inspectExistingFile(filepath.Join(t.TempDir(), "missing.xml"), "missing", ValidateMessagesFile); statuses != nil {
		t.Fatalf("missing optional statuses = %#v, want nil", statuses)
	}
}

func TestResolveMissionPathHandlesSupportedInputs(t *testing.T) {
	root := fixturePath(t, "economyconfig", "full")
	tests := []string{
		root,
		filepath.Join(root, "cfgeconomycore.xml"),
		filepath.Join(root, "cfgIgnoreList.xml"),
		filepath.Join(root, "db", "globals.xml"),
		filepath.Join(root, "env", "wolf_territories.xml"),
		filepath.Join(root, "mods", "mod_types.xml"),
	}
	for _, path := range tests {
		resolved, err := ResolveMissionPath(path)
		if err != nil {
			t.Fatalf("ResolveMissionPath(%s) returned error: %v", path, err)
		}
		if strings.TrimSpace(resolved.Root) == "" || strings.TrimSpace(resolved.CorePath) == "" {
			t.Fatalf("ResolveMissionPath(%s) returned empty paths: %#v", path, resolved)
		}
	}

	_, err := ResolveMissionPath(filepath.Join(root, "missing.xml"))
	if err == nil {
		t.Fatal("ResolveMissionPath missing path err = nil, want error")
	}
}

func TestAggregateWarningHelpersIgnoreMissingInputs(t *testing.T) {
	statuses := []FileStatus{{Kind: "events"}, {Kind: "cfgspawnabletypes"}, {Kind: "cfgenvironment"}}
	root := t.TempDir()

	addEventSpawnWarnings(statuses, root)
	addRandomPresetWarnings(statuses, root)
	addEnvironmentWarnings(statuses, root)

	assertEqual(t, len(statuses[0].Warnings), 0)

	eventsOnly := t.TempDir()
	writeTestFile(t, filepath.Join(eventsOnly, "db", "events.xml"), `<events />`)
	addEventSpawnWarnings(statuses, eventsOnly)

	presetsOnly := t.TempDir()
	writeTestFile(t, filepath.Join(presetsOnly, "cfgrandompresets.xml"), `<randompresets />`)
	addRandomPresetWarnings(statuses, presetsOnly)
}

func TestAggregateValidatorsReportReadFailures(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	validators := []statusValidator{
		ValidateGlobalsFile,
		ValidateEventsFile,
		ValidateMessagesFile,
		ValidateEventSpawnsFile,
		ValidateRandomPresetsFile,
		ValidateSpawnableTypesFile,
		ValidatePlayerSpawnPointsFile,
		ValidateEnvironmentFile,
		ValidateEffectAreaFile,
		ValidateIgnoreListFile,
	}
	for _, validator := range validators {
		if _, err := validator(missing); err == nil {
			t.Fatal("validator missing file err = nil, want error")
		}
	}
}

func TestValidateGlobalsData(t *testing.T) {
	assertContains(t, ValidationErrors{"one", "two"}.Error(), "one; two")

	valid := `<?xml version="1.0"?><variables><var name="AnimalMaxCount" type="0" value="1" /><var name="LootDamageMin" type="1" value="0.5" /><var name="TimeLogin" type="0" value="1" /></variables>`
	if err := ValidateGlobalsData([]byte(valid), "globals.xml"); err != nil {
		t.Fatalf("valid globals returned error: %v", err)
	}

	tests := map[string]string{
		"wrong root":   `<?xml version="1.0"?><bad />`,
		"malformed":    `<?xml version="1.0"?><variables>`,
		"missing name": `<?xml version="1.0"?><variables><var type="0" value="1" /></variables>`,
		"unknown":      `<?xml version="1.0"?><variables><var name="Custom" type="0" value="1" /></variables>`,
		"bad type":     `<?xml version="1.0"?><variables><var name="AnimalMaxCount" type="1" value="1" /></variables>`,
		"bad int":      `<?xml version="1.0"?><variables><var name="AnimalMaxCount" type="0" value="x" /></variables>`,
		"int max":      `<?xml version="1.0"?><variables><var name="TimeLogin" type="0" value="65537" /></variables>`,
		"bad float":    `<?xml version="1.0"?><variables><var name="LootDamageMin" type="1" value="x" /></variables>`,
		"float min":    `<?xml version="1.0"?><variables><var name="LootDamageMin" type="1" value="-0.1" /></variables>`,
		"float max":    `<?xml version="1.0"?><variables><var name="LootDamageMax" type="1" value="1.1" /></variables>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateGlobalsData([]byte(data), "globals.xml"); err == nil {
				t.Fatal("err = nil, want validation error")
			}
		})
	}
}

func TestParseEventsDataValidation(t *testing.T) {
	valid := `<?xml version="1.0"?><events><event name="A"><nominal>1</nominal><active>1</active><position>uniform</position><limit>parent</limit><flags deletable="1" init_random="0" remove_damaged="0" /><children><child type="Thing" min="1" max="2" lootmin="0" lootmax="1" /></children><ignored /></event><ignored /></events>`
	positions, err := ParseEventsData([]byte(valid), "events.xml")
	if err != nil {
		t.Fatalf("valid events returned error: %v", err)
	}
	assertEqual(t, positions["A"], "uniform")

	tests := map[string]string{
		"bad root":       `<bad />`,
		"bad scalar xml": `<events><event name="A"><nominal><child /></nominal></event></events>`,
		"missing name":   `<events><event><position>player</position></event></events>`,
		"bad int":        `<events><event name="A"><nominal>x</nominal></event></events>`,
		"bad active":     `<events><event name="A"><active>yes</active></event></events>`,
		"bad position":   `<events><event name="A"><position>everywhere</position></event></events>`,
		"bad limit":      `<events><event name="A"><limit>none</limit></event></events>`,
		"bad flag":       `<events><event name="A"><flags deletable="yes" /></event></events>`,
		"bad child":      `<events><event name="A"><children><child min="x" /></children></event></events>`,
		"active child":   `<events><event name="A"><active><child /></active></event></events>`,
		"position child": `<events><event name="A"><position><child /></position></event></events>`,
		"limit child":    `<events><event name="A"><limit><child /></limit></event></events>`,
		"unknown eof":    `<events><unknown>`,
		"root eof":       `<events>`,
		"event eof":      `<events><event name="A">`,
		"children eof":   `<events><event name="A"><children>`,
		"child eof":      `<events><event name="A"><children><child>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseEventsData([]byte(data), "events.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestValidateMessagesData(t *testing.T) {
	valid := `<messages><message deadline="1" shutdown="0" text="Hi"><deadline>2</deadline><shutdown>1</shutdown><text>Bye</text><unknown>ignored</unknown></message><other><child /></other></messages>`
	if err := ValidateMessagesData([]byte(valid), "messages.xml"); err != nil {
		t.Fatalf("valid messages returned error: %v", err)
	}

	tests := map[string]string{
		"wrong root":    `<bad />`,
		"missing root":  ``,
		"text outside":  `text<messages />`,
		"multiple root": `<messages /><messages />`,
		"bad deadline":  `<messages><message deadline="x" /></messages>`,
		"bad shutdown":  `<messages><message><shutdown>yes</shutdown></message></messages>`,
		"bad child":     `<messages><message><deadline><child /></deadline></message></messages>`,
		"parse outside": `<`,
		"root eof":      `<messages>`,
		"unknown eof":   `<messages><other>`,
		"eof":           `<messages><message>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateMessagesData([]byte(data), "messages.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestParseEventSpawnsDataValidation(t *testing.T) {
	valid := `<eventposdef><event name="A"><pos x="1" z="2" a="360" /><pos x="1" z="2" a="1.5" /><pos x="1" z="2" a="-177.003967" /><pos x="1" z="2" /><unknown><child /></unknown></event><unknown /></eventposdef>`
	names, err := ParseEventSpawnsData([]byte(valid), "cfgeventspawns.xml")
	if err != nil {
		t.Fatalf("valid event spawns returned error: %v", err)
	}
	assertEqual(t, names["A"], true)

	tests := map[string]string{
		"wrong root":   `<bad />`,
		"missing name": `<eventposdef><event><pos x="1" z="2" a="0" /></event></eventposdef>`,
		"missing x":    `<eventposdef><event name="A"><pos z="2" a="0" /></event></eventposdef>`,
		"bad z":        `<eventposdef><event name="A"><pos x="1" z="bad" a="0" /></event></eventposdef>`,
		"bad a":        `<eventposdef><event name="A"><pos x="1" z="2" a="bad" /></event></eventposdef>`,
		"root eof":     `<eventposdef>`,
		"unknown eof":  `<eventposdef><unknown>`,
		"pos eof":      `<eventposdef><event name="A"><pos x="1" z="2" a="0">`,
		"eof":          `<eventposdef><event name="A">`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseEventSpawnsData([]byte(data), "cfgeventspawns.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestParseRandomPresetsDataValidation(t *testing.T) {
	valid := "\ufeff" + `<randompresets><cargo chance="0" name="food"><item name="Apple" chance="2" /></cargo><attachments name="parts"><item name="Battery" /></attachments><unknown /></randompresets>`
	presets, err := ParseRandomPresetsData([]byte(valid), "cfgrandompresets.xml")
	if err != nil {
		t.Fatalf("valid random presets returned error: %v", err)
	}
	assertEqual(t, presets["food"], true)

	tests := map[string]string{
		"wrong root":      `<bad />`,
		"missing name":    `<randompresets><cargo /></randompresets>`,
		"bad chance":      `<randompresets><cargo name="x" chance="bad" /></randompresets>`,
		"chance range":    `<randompresets><cargo name="x" chance="2" /></randompresets>`,
		"item name":       `<randompresets><cargo name="x"><item /></cargo></randompresets>`,
		"item chance":     `<randompresets><cargo name="x"><item name="Apple" chance="bad" /></cargo></randompresets>`,
		"root eof":        `<randompresets>`,
		"unknown eof":     `<randompresets><unknown>`,
		"preset eof":      `<randompresets><cargo name="x">`,
		"malformed child": `<randompresets><cargo name="x"><item>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRandomPresetsData([]byte(data), "cfgrandompresets.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestParseSpawnableTypesDataValidation(t *testing.T) {
	valid := `<spawnabletypes><type name="Bag"><cargo preset="food" /><attachments chance="1"><item name="Apple" chance="2" /></attachments><hoarder><item name="Apple" count_in_hoarder="0" /></hoarder><unknown /></type><unknown /></spawnabletypes>`
	refs, err := ParseSpawnableTypesData([]byte(valid), "cfgspawnabletypes.xml")
	if err != nil {
		t.Fatalf("valid spawnable types returned error: %v", err)
	}
	assertEqual(t, refs["food"], true)

	tests := map[string]string{
		"wrong root":        `<bad />`,
		"missing type":      `<spawnabletypes><type /></spawnabletypes>`,
		"missing config":    `<spawnabletypes><type name="Bag"><cargo /></type></spawnabletypes>`,
		"bad chance":        `<spawnabletypes><type name="Bag"><attachments chance="bad" /></type></spawnabletypes>`,
		"chance range":      `<spawnabletypes><type name="Bag"><attachments chance="2" /></type></spawnabletypes>`,
		"missing item":      `<spawnabletypes><type name="Bag"><attachments chance="1"><item /></attachments></type></spawnabletypes>`,
		"bad item chance":   `<spawnabletypes><type name="Bag"><attachments chance="1"><item name="Apple" chance="bad" /></attachments></type></spawnabletypes>`,
		"hoarder item":      `<spawnabletypes><type name="Bag"><hoarder><item count_in_hoarder="yes" /></hoarder></type></spawnabletypes>`,
		"root eof":          `<spawnabletypes>`,
		"unknown eof":       `<spawnabletypes><unknown>`,
		"type unknown eof":  `<spawnabletypes><type name="Bag"><unknown>`,
		"malformed type":    `<spawnabletypes><type name="Bag">`,
		"cargo eof":         `<spawnabletypes><type name="Bag"><cargo preset="x">`,
		"malformed cargo":   `<spawnabletypes><type name="Bag"><cargo preset="x"><item>`,
		"hoarder eof":       `<spawnabletypes><type name="Bag"><hoarder>`,
		"malformed hoarder": `<spawnabletypes><type name="Bag"><hoarder><item>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSpawnableTypesData([]byte(data), "cfgspawnabletypes.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestValidatePlayerSpawnPointsData(t *testing.T) {
	valid := `<playerspawnpoints><fresh><generator type="grid"><spawn id="1"><pos x="1" z="2" /></spawn></generator></fresh></playerspawnpoints>`
	if err := ValidatePlayerSpawnPointsData([]byte(valid), "cfgplayerspawnpoints.xml"); err != nil {
		t.Fatalf("valid player spawn points returned error: %v", err)
	}

	tests := map[string]string{
		"wrong root":     `<bad />`,
		"bad generator":  `<playerspawnpoints><generator /></playerspawnpoints>`,
		"bad spawn id":   `<playerspawnpoints><spawn id="x" /></playerspawnpoints>`,
		"missing pos x":  `<playerspawnpoints><pos z="2" /></playerspawnpoints>`,
		"bad pos z":      `<playerspawnpoints><pos x="1" z="bad" /></playerspawnpoints>`,
		"malformed root": `<playerspawnpoints><fresh>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePlayerSpawnPointsData([]byte(data), "cfgplayerspawnpoints.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestParseEnvironmentDataValidation(t *testing.T) {
	valid := `<env><territory type="Herd" x="1" z="2" width="3" height="4"><file path="env/wolf.xml" /><file usable="wolf" /></territory></env>`
	paths, usables, err := ParseEnvironmentData([]byte(valid), "cfgenvironment.xml")
	if err != nil {
		t.Fatalf("valid environment returned error: %v", err)
	}
	assertEqual(t, paths[0], "env/wolf.xml")
	assertEqual(t, usables["wolf"], true)

	tests := map[string]string{
		"wrong root":    `<bad />`,
		"blank type":    `<env><territory type=" " /></env>`,
		"bad territory": `<env><territory x="bad" /></env>`,
		"bad file":      `<env><file /></env>`,
		"malformed":     `<env><territory>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseEnvironmentData([]byte(data), "cfgenvironment.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestValidateEffectAreaData(t *testing.T) {
	valid := `{"Areas":[{"AreaName":"A","Type":"ContaminatedArea_Dynamic","TriggerType":"ContaminatedTrigger","Data":{"Pos":[1,2,3],"Radius":1,"PosHeight":1,"NegHeight":1,"VerticalOffset":0,"InnerRingRatio":0,"OuterRingRatio":1},"PlayerData":{"AroundPartName":"a","TinyPartName":"t","PPERequesterType":"p"}}],"Other":{"nested":[true,null]}}`
	if err := ValidateEffectAreaData([]byte(valid), "cfgEffectArea.json"); err != nil {
		t.Fatalf("valid effect area returned error: %v", err)
	}

	tests := map[string]string{
		"malformed":       `{"Areas":`,
		"multiple":        `{} {}`,
		"array root":      `[]`,
		"duplicate":       `{"Areas":[],"Areas":[]}`,
		"areas type":      `{"Areas":{}}`,
		"area type":       `{"Areas":[1]}`,
		"name type":       `{"Areas":[{"AreaName":1}]}`,
		"type type":       `{"Areas":[{"Type":1}]}`,
		"type invalid":    `{"Areas":[{"Type":"Bad"}]}`,
		"trigger invalid": `{"Areas":[{"TriggerType":"Bad"}]}`,
		"data type":       `{"Areas":[{"Data":1}]}`,
		"pos type":        `{"Areas":[{"Data":{"Pos":1}}]}`,
		"pos len":         `{"Areas":[{"Data":{"Pos":[1,2]}}]}`,
		"pos value":       `{"Areas":[{"Data":{"Pos":[1,"x",3]}}]}`,
		"radius type":     `{"Areas":[{"Data":{"Radius":"x"}}]}`,
		"ratio type":      `{"Areas":[{"Data":{"InnerRingRatio":"x"}}]}`,
		"ratio overflow":  `{"Areas":[{"Data":{"InnerRingRatio":1e9999}}]}`,
		"ratio min":       `{"Areas":[{"Data":{"InnerRingRatio":-0.1}}]}`,
		"ratio max":       `{"Areas":[{"Data":{"OuterRingRatio":1.1}}]}`,
		"player type":     `{"Areas":[{"PlayerData":1}]}`,
		"player field":    `{"Areas":[{"PlayerData":{"AroundPartName":1}}]}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateEffectAreaData([]byte(data), "cfgEffectArea.json"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestValidateIgnoreListData(t *testing.T) {
	if err := ValidateIgnoreListData([]byte(`<ignore><type name="Bandage" /><unknown /></ignore>`), "cfgIgnoreList.xml"); err != nil {
		t.Fatalf("valid ignore list returned error: %v", err)
	}
	tests := map[string]string{
		"wrong root": `<bad />`,
		"missing":    `<ignore><type /></ignore>`,
		"root eof":   `<ignore>`,
		"malformed":  `<ignore><type>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateIgnoreListData([]byte(data), "cfgIgnoreList.xml"); err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestJSONParserDefensiveErrors(t *testing.T) {
	if err := walkXMLRoot(jsonlessXMLDecoder(`<root />`), "root", "root.xml", func(xml.StartElement) error { return io.EOF }); err == nil {
		t.Fatal("walkXMLRoot handler EOF err = nil, want error")
	}
	decoderXML := jsonlessXMLDecoder(`<field>`)
	if _, err := decoderXML.Token(); err != nil {
		t.Fatalf("read field start: %v", err)
	}
	if _, err := readTextElement(decoderXML, "field"); err == nil {
		t.Fatal("readTextElement EOF err = nil, want error")
	}
	if _, err := parseStrictJSON([]byte(`{} x`)); err == nil {
		t.Fatal("parseStrictJSON trailing parse err = nil, want error")
	}
	if _, err := parseJSONValue(json.NewDecoder(strings.NewReader(`1`))); err == nil {
		t.Fatal("parseJSONValue unexpected token err = nil, want error")
	}
	decoder := jsonDecoder(`[`)
	if _, err := parseJSONArray(decoder); err == nil {
		t.Fatal("parseJSONArray read err = nil, want error")
	}
	decoder = jsonDecoder(`{"a"`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read object start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject key read err = nil, want error")
	}
	decoder = jsonDecoder(`{"`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read object start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject unterminated key err = nil, want error")
	}
	decoder = jsonDecoder(`{"a":`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read object start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject value read err = nil, want error")
	}
	decoder = jsonDecoder(`{`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read object start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject end read err = nil, want error")
	}
	decoder = jsonDecoder(`[1]`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read array start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject key type err = nil, want error")
	}
	decoder = jsonDecoder(`[]`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read array start: %v", err)
	}
	if _, err := parseJSONObject(decoder); err == nil {
		t.Fatal("parseJSONObject wrong end err = nil, want error")
	}
	decoder = jsonDecoder(`{}`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read object start: %v", err)
	}
	if _, err := parseJSONArray(decoder); err == nil {
		t.Fatal("parseJSONArray wrong end err = nil, want error")
	}
	decoder = jsonDecoder(`[]`)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read array start: %v", err)
	}
	if _, err := parseJSONValue(decoder); err == nil {
		t.Fatal("parseJSONValue closing delimiter err = nil, want error")
	}
}

func statusKinds(statuses []FileStatus) string {
	var kinds []string
	for _, status := range statuses {
		kinds = append(kinds, status.Kind)
	}
	return strings.Join(kinds, ",")
}

func allWarnings(statuses []FileStatus) string {
	var warnings []string
	for _, status := range statuses {
		warnings = append(warnings, status.Warnings...)
	}
	return strings.Join(warnings, "\n")
}

func jsonDecoder(data string) *json.Decoder {
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.UseNumber()
	return decoder
}

func jsonlessXMLDecoder(data string) *xml.Decoder {
	return xml.NewDecoder(strings.NewReader(data))
}

func TestAddTypeIdentityWarningsSkipsUnparseableStatus(t *testing.T) {
	statuses := []FileStatus{
		{Kind: "cfgeconomycore", Path: "core.xml"},
		{Kind: "types", Path: filepath.Join(t.TempDir(), "missing.xml")},
	}

	addTypeIdentityWarnings(statuses)

	assertEqual(t, len(statuses[1].Warnings), 0)
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

func writeTypesFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "types.xml")
	if err := os.WriteFile(path, []byte(`<?xml version="1.0" encoding="UTF-8"?>`+content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make test dir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func minimalLimits() LimitsDefinition {
	limits := newLimitsDefinition()
	limits.Categories["tools"] = true
	limits.Categories["weapons"] = true
	limits.Tags["floor"] = true
	limits.Usages["Military"] = true
	limits.Values["Tier1"] = true
	limits.Values["CustomTier"] = true
	return limits
}

func validLimitsXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?><lists><categories><category name="tools" /></categories><tags><tag name="floor" /></tags><usageflags><usage name="Military" /></usageflags><valueflags><value name="Tier1" /></valueflags></lists>`
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
