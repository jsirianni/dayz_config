package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/cli/validation"
)

func TestNewCommandMetadata(t *testing.T) {
	command := NewCommand(&bytes.Buffer{})

	assertEqual(t, command.Use, "validate <cfgeconomycore.xml>")
	assertEqual(t, command.Short, "Validate cfgeconomycore.xml and referenced types files")
}

func TestNewCommandExecutesValidation(t *testing.T) {
	var stdout bytes.Buffer
	command := NewCommand(&stdout)
	command.SetArgs([]string{fixturePath(t, "mission", "cfgeconomycore.xml")})

	err := command.Execute()

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	assertContains(t, stdout.String(), "cfgeconomycore.xml ok")
}

func TestValidateEconomyCoreReturnsNilWhenAllFilesParse(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomyCore(fixturePath(t, "mission", "cfgeconomycore.xml"), &stdout)

	if err != nil {
		t.Fatalf("ValidateEconomyCore returned error: %v", err)
	}
	assertContains(t, stdout.String(), "cfgeconomycore.xml ok")
	assertContains(t, stdout.String(), filepath.Join("db", "types.xml")+" ok (2 types)")
}

func TestValidateEconomyCoreReturnsConfigError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomyCore(fixturePath(t, "mission", "mods", "valid_types.xml"), &stdout)

	if err == nil {
		t.Fatal("err = nil, want config error")
	}
	assertContains(t, err.Error(), "cfgeconomycore: failed")
}

func TestValidateEconomyCoreReturnsValidationError(t *testing.T) {
	var stdout bytes.Buffer

	err := ValidateEconomyCore(fixturePath(t, "mission", "cfgeconomycore_bad_ref.xml"), &stdout)

	if err != validation.ErrFailed {
		t.Fatalf("err = %v, want validation.ErrFailed", err)
	}
	assertContains(t, stdout.String(), "failed")
}

func TestPrintWarningsWritesEachWarning(t *testing.T) {
	var stdout bytes.Buffer

	printWarnings(&stdout, FileStatus{
		Kind:     "types",
		Path:     "types.xml",
		Warnings: []string{"first", "second"},
	})

	assertContains(t, stdout.String(), "types types.xml warning: first")
	assertContains(t, stdout.String(), "types types.xml warning: second")
}

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

func TestValidateEconomyCorePrintsSameFileDuplicateTypeWarnings(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "cfglimitsdefinition.xml"), validLimitsXML())
	if err := os.Mkdir(filepath.Join(dir, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	writeTestFile(t, filepath.Join(dir, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore />`)
	writeTestFile(t, filepath.Join(dir, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Duplicate"><nominal>1</nominal></type><type name="Duplicate"><nominal>2</nominal></type></types>`)

	var stdout bytes.Buffer
	err := ValidateEconomyCore(filepath.Join(dir, "cfgeconomycore.xml"), &stdout)

	if err != nil {
		t.Fatalf("ValidateEconomyCore returned error: %v", err)
	}
	assertContains(t, stdout.String(), "base-types")
	assertContains(t, stdout.String(), `warning: type "Duplicate" duplicates a type already loaded from`)
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

	args := append([]string{filepath.Dir(file), "..", "..", "..", "testdata"}, parts...)
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
