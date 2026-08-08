package economyconfig

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type FileStatus struct {
	Path      string
	Kind      string
	TypeCount int
	Warnings  []string
	Err       error
}

type ValidationErrors []string

func (errs ValidationErrors) Error() string {
	return strings.Join(errs, "; ")
}

type missionPath struct {
	Root     string
	CorePath string
}

var rootEconomyFiles = map[string]bool{
	"cfgeconomycore.xml":          true,
	"cfglimitsdefinition.xml":     true,
	"cfglimitsdefinitionuser.xml": true,
	"cfgeventspawns.xml":          true,
	"cfgrandompresets.xml":        true,
	"cfgspawnabletypes.xml":       true,
	"cfgplayerspawnpoints.xml":    true,
	"cfgenvironment.xml":          true,
	"cfgeffectarea.json":          true,
	"cfgignorelist.xml":           true,
}

func InspectEconomy(path string) ([]FileStatus, error) {
	resolved, err := ResolveMissionPath(path)
	if err != nil {
		return nil, err
	}

	statuses, err := InspectEconomyCore(resolved.CorePath)
	if err != nil {
		return nil, err
	}

	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfglimitsdefinition.xml"), "cfglimitsdefinition", validateLimitsDefinitionStatus)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfglimitsdefinitionuser.xml"), "cfglimitsdefinitionuser", validateUserLimitsDefinitionStatus)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "db", "events.xml"), "events", ValidateEventsFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "db", "globals.xml"), "globals", ValidateGlobalsFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "db", "messages.xml"), "messages", ValidateMessagesFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgeventspawns.xml"), "cfgeventspawns", ValidateEventSpawnsFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgrandompresets.xml"), "cfgrandompresets", ValidateRandomPresetsFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgspawnabletypes.xml"), "cfgspawnabletypes", ValidateSpawnableTypesFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgplayerspawnpoints.xml"), "cfgplayerspawnpoints", ValidatePlayerSpawnPointsFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgenvironment.xml"), "cfgenvironment", ValidateEnvironmentFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgEffectArea.json"), "cfgEffectArea", ValidateEffectAreaFile)...)
	statuses = append(statuses, inspectExistingFile(filepath.Join(resolved.Root, "cfgIgnoreList.xml"), "cfgIgnoreList", ValidateIgnoreListFile)...)
	addAggregateWarnings(statuses, resolved.Root)
	return statuses, nil
}

func ResolveMissionPath(path string) (missionPath, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return missionPath{}, fmt.Errorf("stat %s: %w", cleanPath, err)
	}
	if info.IsDir() {
		return missionPath{Root: cleanPath, CorePath: filepath.Join(cleanPath, "cfgeconomycore.xml")}, nil
	}

	base := strings.ToLower(filepath.Base(cleanPath))
	parent := filepath.Dir(cleanPath)
	parentBase := strings.ToLower(filepath.Base(parent))
	switch {
	case base == "cfgeconomycore.xml":
		return missionPath{Root: parent, CorePath: cleanPath}, nil
	case rootEconomyFiles[base]:
		return missionPath{Root: parent, CorePath: filepath.Join(parent, "cfgeconomycore.xml")}, nil
	case parentBase == "db":
		root := filepath.Dir(parent)
		return missionPath{Root: root, CorePath: filepath.Join(root, "cfgeconomycore.xml")}, nil
	case parentBase == "env":
		root := filepath.Dir(parent)
		return missionPath{Root: root, CorePath: filepath.Join(root, "cfgeconomycore.xml")}, nil
	default:
		return missionPath{Root: parent, CorePath: cleanPath}, nil
	}
}

type statusValidator func(string) ([]string, error)

func inspectExistingFile(path string, kind string, validate statusValidator) []FileStatus {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	warnings, err := validate(path)
	return []FileStatus{{Path: filepath.Clean(path), Kind: kind, Warnings: warnings, Err: err}}
}

func validateLimitsDefinitionStatus(path string) ([]string, error) {
	_, err := ParseLimitsDefinitionFile(path)
	return nil, err
}

func validateUserLimitsDefinitionStatus(path string) ([]string, error) {
	limits := newLimitsDefinition()
	err := AppendUserLimitsDefinitionFile(path, &limits)
	return nil, err
}

func addAggregateWarnings(statuses []FileStatus, missionRoot string) {
	addEventSpawnWarnings(statuses, missionRoot)
	addRandomPresetWarnings(statuses, missionRoot)
	addEnvironmentWarnings(statuses, missionRoot)
}

func addEventSpawnWarnings(statuses []FileStatus, missionRoot string) {
	events, err := eventPositions(filepath.Join(missionRoot, "db", "events.xml"))
	if err != nil {
		return
	}
	spawns, err := eventSpawnNames(filepath.Join(missionRoot, "cfgeventspawns.xml"))
	if err != nil {
		return
	}
	for _, event := range sortedMapKeysString(events) {
		if events[event] != "fixed" || spawns[event] {
			continue
		}
		appendWarning(statuses, "events", fmt.Sprintf("fixed event %q has no matching cfgeventspawns.xml event", event))
	}
}

func addRandomPresetWarnings(statuses []FileStatus, missionRoot string) {
	presets, err := randomPresetNames(filepath.Join(missionRoot, "cfgrandompresets.xml"))
	if err != nil {
		return
	}
	refs, err := spawnablePresetRefs(filepath.Join(missionRoot, "cfgspawnabletypes.xml"))
	if err != nil {
		return
	}
	for _, ref := range sortedMapKeysBool(refs) {
		if presets[ref] {
			continue
		}
		appendWarning(statuses, "cfgspawnabletypes", fmt.Sprintf("references random preset %q not defined in cfgrandompresets.xml", ref))
	}
}

func addEnvironmentWarnings(statuses []FileStatus, missionRoot string) {
	paths, usables, err := environmentFileRefs(filepath.Join(missionRoot, "cfgenvironment.xml"))
	if err != nil {
		return
	}
	usableFiles := map[string]bool{}
	for _, path := range paths {
		fullPath := filepath.Join(missionRoot, filepath.FromSlash(path))
		if _, err := os.Stat(fullPath); err != nil && errors.Is(err, os.ErrNotExist) {
			appendWarning(statuses, "cfgenvironment", fmt.Sprintf("references missing territory file %q", path))
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		usableFiles[base] = true
	}
	for _, usable := range sortedMapKeysBool(usables) {
		if usableFiles[usable] {
			continue
		}
		appendWarning(statuses, "cfgenvironment", fmt.Sprintf("references usable file %q not registered by path", usable))
	}
}

func appendWarning(statuses []FileStatus, kind string, warning string) {
	for index := range statuses {
		if statuses[index].Kind == kind && statuses[index].Err == nil {
			statuses[index].Warnings = append(statuses[index].Warnings, warning)
			return
		}
	}
}

func sortedMapKeysString(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeysBool(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func InspectEconomyCore(path string) ([]FileStatus, error) {
	cleanPath := filepath.Clean(path)

	economyCore, err := ParseEconomyCoreFile(cleanPath)
	if err != nil {
		return nil, err
	}

	missionDir := filepath.Dir(cleanPath)
	limits, err := LoadLimitsDefinitions(missionDir)
	if err != nil {
		return nil, err
	}

	typeFiles := []string{filepath.Join(missionDir, "db", "types.xml")}
	for _, ref := range economyCore.TypeFileRefs() {
		typeFiles = append(typeFiles, filepath.Join(missionDir, ref.Folder, ref.Name))
	}

	statuses := make([]FileStatus, 0, len(typeFiles)+1)
	statuses = append(statuses, FileStatus{
		Path: cleanPath,
		Kind: "cfgeconomycore",
	})
	for i, typeFile := range typeFiles {
		status := FileStatus{
			Path: typeFile,
			Kind: "types",
		}
		if i == 0 {
			status.Kind = "base-types"
		}

		types, warnings, parseErr := ValidateTypesFile(typeFile, limits)
		if parseErr != nil {
			status.Err = parseErr
		} else {
			status.TypeCount = len(types.Types)
			status.Warnings = append(status.Warnings, warnings...)
		}
		statuses = append(statuses, status)
	}

	addTypeIdentityWarnings(statuses)

	return statuses, nil
}

type EconomyCore struct {
	XMLName xml.Name
	CEs     []CEBlock `xml:"ce"`
}

type CEBlock struct {
	Folder string      `xml:"folder,attr"`
	Files  []CEFileRef `xml:"file"`
}

type CEFileRef struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type TypeFileRef struct {
	Folder string
	Name   string
}

func ParseEconomyCoreFile(path string) (EconomyCore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EconomyCore{}, fmt.Errorf("read %s: %w", path, err)
	}

	var core EconomyCore
	if err := xml.Unmarshal(data, &core); err != nil {
		return EconomyCore{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if core.XMLName.Local != "economycore" {
		return EconomyCore{}, fmt.Errorf("parse %s: expected <economycore> root, got <%s>", path, core.XMLName.Local)
	}

	return core, nil
}

func (core EconomyCore) TypeFileRefs() []TypeFileRef {
	var refs []TypeFileRef
	for _, ce := range core.CEs {
		for _, file := range ce.Files {
			if file.Type != "types" {
				continue
			}
			refs = append(refs, TypeFileRef{
				Folder: ce.Folder,
				Name:   file.Name,
			})
		}
	}
	return refs
}

type TypesFile struct {
	XMLName xml.Name
	Types   []TypeEntry `xml:"type"`
}

type TypeEntry struct {
	Name       string
	Nominal    int
	Lifetime   int
	Restock    int
	Min        int
	QuantMin   int
	QuantMax   int
	Cost       int
	Flags      []FlagSetting
	Categories []Category
	Tags       []NamedField
	Usages     []NamedField
	Values     []ValueField

	seenFields map[string]bool
}

func (entry *TypeEntry) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local != "name" {
			return fmt.Errorf("type has unknown attribute %q", attr.Name.Local)
		}
		entry.Name = attr.Value
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		switch typedToken := token.(type) {
		case xml.StartElement:
			if err := entry.decodeTypeField(decoder, typedToken); err != nil {
				return err
			}
		case xml.EndElement:
			if typedToken.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func (entry *TypeEntry) decodeTypeField(decoder *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "nominal":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.Nominal, integerRange{Min: 0})
	case "lifetime":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.Lifetime, integerRange{Min: 0})
	case "restock":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.Restock, integerRange{Min: 0})
	case "min":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.Min, integerRange{Min: 0})
	case "quantmin":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.QuantMin, quantRange)
	case "quantmax":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.QuantMax, quantRange)
	case "cost":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		return decodeIntegerField(decoder, start, &entry.Cost, integerRange{Min: 0})
	case "flags":
		if err := entry.markSingletonField(start.Name.Local); err != nil {
			return err
		}
		var flags FlagsField
		if err := decoder.DecodeElement(&flags, &start); err != nil {
			return err
		}
		entry.Flags = append(entry.Flags, flags.Flags...)
		return nil
	case "category":
		var category CategoryField
		if err := decoder.DecodeElement(&category, &start); err != nil {
			return err
		}
		entry.Categories = append(entry.Categories, category.Name)
		return nil
	case "tag":
		var tag NamedField
		err := decoder.DecodeElement(&tag, &start)
		entry.Tags = append(entry.Tags, tag)
		return err
	case "usage":
		var usage NamedField
		err := decoder.DecodeElement(&usage, &start)
		entry.Usages = append(entry.Usages, usage)
		return err
	case "value":
		var value ValueField
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return err
		}
		entry.Values = append(entry.Values, value)
		return nil
	default:
		return fmt.Errorf("type %q has unknown field <%s>", entry.Name, start.Name.Local)
	}
}

func (entry *TypeEntry) markSingletonField(name string) error {
	if entry.seenFields == nil {
		entry.seenFields = make(map[string]bool)
	}
	if entry.seenFields[name] {
		return fmt.Errorf("type %q has duplicate <%s> field", entry.Name, name)
	}
	entry.seenFields[name] = true
	return nil
}

func (entry TypeEntry) hasField(name string) bool {
	return entry.seenFields[name]
}

type integerRange struct {
	Min          int
	Max          int
	AllowMax     bool
	AllowedExtra map[int]bool
}

var quantRange = integerRange{
	Min:          0,
	Max:          100,
	AllowMax:     true,
	AllowedExtra: map[int]bool{-1: true},
}

func decodeIntegerField(decoder *xml.Decoder, start xml.StartElement, target *int, validRange integerRange) error {
	for _, attr := range start.Attr {
		return fmt.Errorf("<%s> has unknown attribute %q", start.Name.Local, attr.Name.Local)
	}

	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		switch typedToken := token.(type) {
		case xml.CharData:
			text.Write([]byte(typedToken))
		case xml.StartElement:
			return fmt.Errorf("<%s> expected integer value, got child <%s>", start.Name.Local, typedToken.Name.Local)
		case xml.EndElement:
			if typedToken.Name.Local == start.Name.Local {
				return assignIntegerField(start.Name.Local, text.String(), target, validRange)
			}
		}
	}
}

func assignIntegerField(fieldName string, text string, target *int, validRange integerRange) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("<%s> expected integer value", fieldName)
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("<%s> expected integer value, got %q", fieldName, trimmed)
	}
	if !validRange.Contains(value) {
		return fmt.Errorf("<%s> value %d is outside allowed range", fieldName, value)
	}

	*target = value
	return nil
}

func (validRange integerRange) Contains(value int) bool {
	if validRange.AllowedExtra[value] {
		return true
	}
	if value < validRange.Min {
		return false
	}
	if validRange.AllowMax && value > validRange.Max {
		return false
	}
	return true
}

type Flag string

const (
	FlagCountInCargo   Flag = "count_in_cargo"
	FlagCountInHoarder Flag = "count_in_hoarder"
	FlagCountInMap     Flag = "count_in_map"
	FlagCountInPlayer  Flag = "count_in_player"
	FlagCrafted        Flag = "crafted"
	FlagDeloot         Flag = "deloot"
)

var validFlags = map[Flag]bool{
	FlagCountInCargo:   true,
	FlagCountInHoarder: true,
	FlagCountInMap:     true,
	FlagCountInPlayer:  true,
	FlagCrafted:        true,
	FlagDeloot:         true,
}

type FlagSetting struct {
	Name  Flag
	Value bool
}

type FlagsField struct {
	Flags []FlagSetting
}

func (field *FlagsField) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		flag := Flag(attr.Name.Local)
		if !validFlags[flag] {
			return fmt.Errorf("<flags> has unknown flag %q", attr.Name.Local)
		}
		value, err := parseBooleanFlag(attr.Value)
		if err != nil {
			return fmt.Errorf("<flags> flag %q %w", attr.Name.Local, err)
		}
		field.Flags = append(field.Flags, FlagSetting{Name: flag, Value: value})
	}
	return decodeEmptyElement(decoder, start)
}

func parseBooleanFlag(value string) (bool, error) {
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("expected 0 or 1, got %q", value)
	}
}

type Category string

type LimitsDefinition struct {
	Categories map[string]bool
	Tags       map[string]bool
	Usages     map[string]bool
	Values     map[string]bool
}

type CategoryField struct {
	Name Category
}

func (field *CategoryField) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	name, err := readOnlyNameAttribute(start)
	if err != nil {
		return fmt.Errorf("<category> %w", err)
	}

	field.Name = Category(name)
	return decodeEmptyElement(decoder, start)
}

type NamedField struct {
	Name string
}

func (field *NamedField) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	name, err := readOnlyNameAttribute(start)
	if err != nil {
		return fmt.Errorf("<%s> %w", start.Name.Local, err)
	}
	field.Name = name
	return decodeEmptyElement(decoder, start)
}

type ValueField struct {
	Name string
}

var valueNamePattern = regexp.MustCompile(`^name=.+$`)

func (field *ValueField) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var name string
	matchedNameExpression := false
	for _, attr := range start.Attr {
		expression := attr.Name.Local + "=" + attr.Value
		if attr.Name.Local != "name" {
			return fmt.Errorf("<value> has unknown attribute %q", attr.Name.Local)
		}
		if valueNamePattern.MatchString(expression) {
			matchedNameExpression = true
		}
		name = attr.Value
	}
	if !matchedNameExpression {
		return fmt.Errorf("<value> expected name attribute with a value")
	}
	field.Name = name
	return decodeEmptyElement(decoder, start)
}

func decodeEmptyElement(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		switch typedToken := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(typedToken)) != "" {
				return fmt.Errorf("<%s> expected empty element", start.Name.Local)
			}
		case xml.StartElement:
			return fmt.Errorf("<%s> expected empty element, got child <%s>", start.Name.Local, typedToken.Name.Local)
		case xml.EndElement:
			if typedToken.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func readOnlyNameAttribute(start xml.StartElement) (string, error) {
	var name string
	for _, attr := range start.Attr {
		if attr.Name.Local != "name" {
			return "", fmt.Errorf("has unknown attribute %q", attr.Name.Local)
		}
		name = attr.Value
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("expected name attribute with a value")
	}
	return name, nil
}

func ParseTypesFile(path string) (TypesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TypesFile{}, fmt.Errorf("read %s: %w", path, err)
	}

	return ParseTypesData(data, path)
}

func ParseTypesData(data []byte, sourceName string) (TypesFile, error) {
	var types TypesFile
	if err := xml.Unmarshal(data, &types); err != nil {
		return TypesFile{}, fmt.Errorf("parse %s: %w", sourceName, err)
	}
	if types.XMLName.Local != "types" {
		return TypesFile{}, fmt.Errorf("parse %s: expected <types> root, got <%s>", sourceName, types.XMLName.Local)
	}
	for _, entry := range types.Types {
		if strings.TrimSpace(entry.Name) == "" {
			return TypesFile{}, fmt.Errorf("parse %s: type entry is missing name", sourceName)
		}
	}

	return types, nil
}

func ValidateTypesFile(path string, limits LimitsDefinition) (TypesFile, []string, error) {
	types, err := ParseTypesFile(path)
	if err != nil {
		return TypesFile{}, nil, err
	}
	return types, validateTypes(types, limits), nil
}

func validateTypes(types TypesFile, limits LimitsDefinition) []string {
	var warnings []string
	for _, entry := range types.Types {
		warnings = append(warnings, validateLimitReferences(entry, limits)...)
		warnings = append(warnings, validateRelationships(entry)...)
	}
	return warnings
}

func validateLimitReferences(entry TypeEntry, limits LimitsDefinition) []string {
	var warnings []string
	for _, category := range entry.Categories {
		if !limits.Categories[string(category)] {
			warnings = append(warnings, fmt.Sprintf("type %q references category %q not defined in cfglimitsdefinition.xml", entry.Name, category))
		}
	}
	for _, tag := range entry.Tags {
		if !limits.Tags[tag.Name] {
			warnings = append(warnings, fmt.Sprintf("type %q references tag %q not defined in cfglimitsdefinition.xml", entry.Name, tag.Name))
		}
	}
	for _, usage := range entry.Usages {
		if !limits.Usages[usage.Name] {
			warnings = append(warnings, fmt.Sprintf("type %q references usage %q not defined in cfglimitsdefinition.xml", entry.Name, usage.Name))
		}
	}
	for _, value := range entry.Values {
		if !limits.Values[value.Name] {
			warnings = append(warnings, fmt.Sprintf("type %q references value %q not defined in cfglimitsdefinition.xml", entry.Name, value.Name))
		}
	}
	return warnings
}

func validateRelationships(entry TypeEntry) []string {
	var warnings []string
	if entry.hasField("nominal") && entry.hasField("min") && entry.Min > entry.Nominal {
		warnings = append(warnings, fmt.Sprintf("type %q has min greater than nominal", entry.Name))
	}
	if entry.hasField("quantmin") && entry.hasField("quantmax") && entry.QuantMin != -1 && entry.QuantMax != -1 && entry.QuantMin > entry.QuantMax {
		warnings = append(warnings, fmt.Sprintf("type %q has quantmin greater than quantmax", entry.Name))
	}
	return warnings
}

func addTypeIdentityWarnings(statuses []FileStatus) {
	type location struct {
		Path string
	}

	seen := make(map[string]location)
	for statusIndex := range statuses {
		status := &statuses[statusIndex]
		if status.Err != nil || (status.Kind != "base-types" && status.Kind != "types") {
			continue
		}

		types, err := ParseTypesFile(status.Path)
		if err != nil {
			continue
		}
		for _, entry := range types.Types {
			previous, exists := seen[entry.Name]
			if exists {
				status.Warnings = append(status.Warnings, fmt.Sprintf("type %q duplicates a type already loaded from %s", entry.Name, previous.Path))
				continue
			}
			seen[entry.Name] = location{Path: status.Path}
		}
	}
}

type limitsFile struct {
	XMLName    xml.Name
	Categories []NamedField `xml:"categories>category"`
	Tags       []NamedField `xml:"tags>tag"`
	Usages     []NamedField `xml:"usageflags>usage"`
	Values     []NamedField `xml:"valueflags>value"`
}

type userLimitsFile struct {
	XMLName    xml.Name
	UsageUsers []userLimit `xml:"usageflags>user"`
	ValueUsers []userLimit `xml:"valueflags>user"`
}

type userLimit struct {
	Name   string
	Usages []NamedField `xml:"usage"`
	Values []NamedField `xml:"value"`
}

func (limit *userLimit) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	name, err := readOnlyNameAttribute(start)
	if err != nil {
		return fmt.Errorf("<user> %w", err)
	}
	limit.Name = name

	type userLimitAlias userLimit
	return decoder.DecodeElement((*userLimitAlias)(limit), &start)
}

func LoadLimitsDefinitions(missionDir string) (LimitsDefinition, error) {
	limits, err := ParseLimitsDefinitionFile(filepath.Join(missionDir, "cfglimitsdefinition.xml"))
	if err != nil {
		return LimitsDefinition{}, err
	}

	userPath := filepath.Join(missionDir, "cfglimitsdefinitionuser.xml")
	if err := AppendUserLimitsDefinitionFile(userPath, &limits); err != nil && !errors.Is(err, os.ErrNotExist) {
		return LimitsDefinition{}, err
	}
	return limits, nil
}

func ParseLimitsDefinitionFile(path string) (LimitsDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LimitsDefinition{}, fmt.Errorf("read %s: %w", path, err)
	}

	return ParseLimitsDefinitionData(data, path)
}

func ParseLimitsDefinitionData(data []byte, sourceName string) (LimitsDefinition, error) {
	var parsed limitsFile
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return LimitsDefinition{}, fmt.Errorf("parse %s: %w", sourceName, err)
	}
	if parsed.XMLName.Local != "lists" {
		return LimitsDefinition{}, fmt.Errorf("parse %s: expected <lists> root, got <%s>", sourceName, parsed.XMLName.Local)
	}

	limits := newLimitsDefinition()
	addNamedFields(limits.Categories, parsed.Categories)
	addNamedFields(limits.Tags, parsed.Tags)
	addNamedFields(limits.Usages, parsed.Usages)
	addNamedFields(limits.Values, parsed.Values)
	return limits, nil
}

func AppendUserLimitsDefinitionFile(path string, limits *LimitsDefinition) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return AppendUserLimitsDefinitionData(data, path, limits)
}

func AppendUserLimitsDefinitionData(data []byte, sourceName string, limits *LimitsDefinition) error {
	var parsed userLimitsFile
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse %s: %w", sourceName, err)
	}
	if parsed.XMLName.Local != "user_lists" {
		return fmt.Errorf("parse %s: expected <user_lists> root, got <%s>", sourceName, parsed.XMLName.Local)
	}

	for _, user := range parsed.UsageUsers {
		limits.Usages[user.Name] = true
		addNamedFields(limits.Usages, user.Usages)
	}
	for _, user := range parsed.ValueUsers {
		limits.Values[user.Name] = true
		addNamedFields(limits.Values, user.Values)
	}
	return nil
}

func newLimitsDefinition() LimitsDefinition {
	return LimitsDefinition{
		Categories: make(map[string]bool),
		Tags:       make(map[string]bool),
		Usages:     make(map[string]bool),
		Values:     make(map[string]bool),
	}
}

func addNamedFields(target map[string]bool, fields []NamedField) {
	for _, field := range fields {
		target[field.Name] = true
	}
}

func ValidateGlobalsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nil, ValidateGlobalsData(data, path)
}

func ValidateGlobalsData(data []byte, sourceName string) error {
	var parsed globalsDocument
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse %s: %w", sourceName, err)
	}
	if parsed.XMLName.Local != "variables" {
		return fmt.Errorf("parse %s: expected <variables> root, got <%s>", sourceName, parsed.XMLName.Local)
	}
	var errs ValidationErrors
	for _, variable := range parsed.Vars {
		validateGlobalVar(variable, &errs)
	}
	return validationErrorsOrNil(errs)
}

type globalsDocument struct {
	XMLName xml.Name
	Vars    []globalVar `xml:"var"`
}

type globalVar struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type globalSpec struct {
	Type    string
	Min     *float64
	Max     *float64
	MaxInt  *int
	Allowed map[string]bool
}

var globalSpecs = map[string]globalSpec{
	"AnimalMaxCount":              {Type: "0"},
	"CleanupAvoidance":            {Type: "0"},
	"CleanupLifetimeDeadAnimal":   {Type: "0"},
	"CleanupLifetimeDeadInfected": {Type: "0"},
	"CleanupLifetimeDeadPlayer":   {Type: "0"},
	"CleanupLifetimeDefault":      {Type: "0"},
	"CleanupLifetimeLimit":        {Type: "0"},
	"CleanupLifetimeRuined":       {Type: "0"},
	"FlagRefreshFrequency":        {Type: "0"},
	"FlagRefreshMaxDuration":      {Type: "0"},
	"FoodDecay":                   {Type: "0"},
	"IdleModeCountdown":           {Type: "0"},
	"IdleModeStartup":             {Type: "0"},
	"InitialSpawn":                {Type: "0"},
	"LootDamageMin":               {Type: "1", Min: floatPtrConfig(0), Max: floatPtrConfig(1)},
	"LootDamageMax":               {Type: "1", Min: floatPtrConfig(0), Max: floatPtrConfig(1)},
	"LootProxyPlacement":          {Type: "0"},
	"LootSpawnAvoidance":          {Type: "0"},
	"RespawnAttempt":              {Type: "0"},
	"RespawnLimit":                {Type: "0"},
	"RespawnTypes":                {Type: "0"},
	"RestartSpawn":                {Type: "0"},
	"SpawnInitial":                {Type: "0"},
	"TimeHopping":                 {Type: "0"},
	"TimeLogin":                   {Type: "0", MaxInt: intPtrConfig(65536)},
	"TimeLogout":                  {Type: "0", MaxInt: intPtrConfig(65536)},
	"TimePenalty":                 {Type: "0"},
	"WorldWetTempUpdate":          {Type: "0"},
	"ZombieMaxCount":              {Type: "0"},
	"ZoneSpawnDist":               {Type: "0"},
}

func validateGlobalVar(variable globalVar, errs *ValidationErrors) {
	if strings.TrimSpace(variable.Name) == "" {
		addValidationError(errs, "<var> missing name")
		return
	}
	spec, ok := globalSpecs[variable.Name]
	if !ok {
		addValidationError(errs, "unknown global variable %q", variable.Name)
		return
	}
	if variable.Type != spec.Type {
		addValidationError(errs, "global %q type must be %s", variable.Name, spec.Type)
		return
	}
	switch spec.Type {
	case "0":
		value, err := strconv.Atoi(strings.TrimSpace(variable.Value))
		if err != nil {
			addValidationError(errs, "global %q expected integer value", variable.Name)
			return
		}
		if spec.MaxInt != nil && value > *spec.MaxInt {
			addValidationError(errs, "global %q must be <= %d", variable.Name, *spec.MaxInt)
		}
	case "1":
		value, err := strconv.ParseFloat(strings.TrimSpace(variable.Value), 64)
		if err != nil {
			addValidationError(errs, "global %q expected float value", variable.Name)
			return
		}
		validateFloatRange(errs, "global "+variable.Name, value, spec.Min, spec.Max)
	}
}

func ValidateEventsFile(path string) ([]string, error) {
	_, err := eventPositions(path)
	return nil, err
}

func eventPositions(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseEventsData(data, path)
}

func ParseEventsData(data []byte, sourceName string) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	positions := map[string]string{}
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "events", sourceName, func(start xml.StartElement) error {
		return parseEventsRoot(decoder, start, positions, &errs)
	}); err != nil {
		return nil, err
	}
	return positions, validationErrorsOrNil(errs)
}

func parseEventsRoot(decoder *xml.Decoder, root xml.StartElement, positions map[string]string, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "event" {
				if err := parseEventElement(decoder, value, positions, errs); err != nil {
					return err
				}
				continue
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func parseEventElement(decoder *xml.Decoder, start xml.StartElement, positions map[string]string, errs *ValidationErrors) error {
	name := attrValue(start, "name")
	if strings.TrimSpace(name) == "" {
		addValidationError(errs, "<event> missing name")
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if err := validateEventChild(decoder, value, name, positions, errs); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateEventChild(decoder *xml.Decoder, start xml.StartElement, eventName string, positions map[string]string, errs *ValidationErrors) error {
	switch start.Name.Local {
	case "nominal", "min", "max", "lifetime", "restock", "saferadius", "distanceradius", "cleanupradius":
		text, err := readTextElement(decoder, start.Name.Local)
		if err != nil {
			return err
		}
		if _, parseErr := strconv.Atoi(strings.TrimSpace(text)); parseErr != nil {
			addValidationError(errs, "event %q <%s> expected integer", eventName, start.Name.Local)
		}
	case "active":
		text, err := readTextElement(decoder, start.Name.Local)
		if err != nil {
			return err
		}
		if !isZeroOne(strings.TrimSpace(text)) {
			addValidationError(errs, "event %q <active> expected 0 or 1", eventName)
		}
	case "position":
		text, err := readTextElement(decoder, start.Name.Local)
		if err != nil {
			return err
		}
		position := strings.TrimSpace(text)
		if position != "fixed" && position != "player" && position != "uniform" {
			addValidationError(errs, "event %q <position> expected fixed, player, or uniform", eventName)
		}
		positions[eventName] = position
	case "limit":
		text, err := readTextElement(decoder, start.Name.Local)
		if err != nil {
			return err
		}
		limit := strings.TrimSpace(text)
		if limit != "child" && limit != "custom" && limit != "mixed" && limit != "parent" {
			addValidationError(errs, "event %q <limit> expected child, custom, mixed, or parent", eventName)
		}
	case "flags":
		for _, attr := range start.Attr {
			if attr.Name.Local == "deletable" || attr.Name.Local == "init_random" || attr.Name.Local == "remove_damaged" {
				validateZeroOne(errs, fmt.Sprintf("event %q flag %s", eventName, attr.Name.Local), attr.Value)
			}
		}
		return skipXMLElement(decoder, start.Name.Local)
	case "children":
		return parseEventChildren(decoder, start, eventName, errs)
	default:
		return skipXMLElement(decoder, start.Name.Local)
	}
	return nil
}

func parseEventChildren(decoder *xml.Decoder, start xml.StartElement, eventName string, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "child" {
				validateEventChildAttributes(value, eventName, errs)
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateEventChildAttributes(start xml.StartElement, eventName string, errs *ValidationErrors) {
	if strings.TrimSpace(attrValue(start, "type")) == "" {
		addValidationError(errs, "event %q child missing type", eventName)
	}
	for _, attr := range start.Attr {
		if attr.Name.Local == "lootmax" || attr.Name.Local == "lootmin" || attr.Name.Local == "max" || attr.Name.Local == "min" {
			if _, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err != nil {
				addValidationError(errs, "event %q child %s expected integer", eventName, attr.Name.Local)
			}
		}
	}
}

func ValidateMessagesFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nil, ValidateMessagesData(data, path)
}

func ValidateMessagesData(data []byte, sourceName string) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "messages", sourceName, func(start xml.StartElement) error {
		return parseMessagesRoot(decoder, start, &errs)
	}); err != nil {
		return err
	}
	return validationErrorsOrNil(errs)
}

func parseMessagesRoot(decoder *xml.Decoder, root xml.StartElement, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "message" {
				if err := parseMessageElement(decoder, value, errs); err != nil {
					return err
				}
				continue
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func parseMessageElement(decoder *xml.Decoder, start xml.StartElement, errs *ValidationErrors) error {
	for _, attr := range start.Attr {
		validateMessageField(attr.Name.Local, attr.Value, errs)
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			text, readErr := readTextElement(decoder, value.Name.Local)
			if readErr != nil {
				return readErr
			}
			validateMessageField(value.Name.Local, text, errs)
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateMessageField(name string, value string, errs *ValidationErrors) {
	switch name {
	case "deadline":
		if _, err := strconv.Atoi(strings.TrimSpace(value)); err != nil {
			addValidationError(errs, "message deadline expected integer")
		}
	case "shutdown":
		validateZeroOne(errs, "message shutdown", value)
	case "text":
	default:
	}
}

func ValidateEventSpawnsFile(path string) ([]string, error) {
	_, err := eventSpawnNames(path)
	return nil, err
}

func eventSpawnNames(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseEventSpawnsData(data, path)
}

func ParseEventSpawnsData(data []byte, sourceName string) (map[string]bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	names := map[string]bool{}
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "eventposdef", sourceName, func(start xml.StartElement) error {
		return parseEventSpawnsRoot(decoder, start, names, &errs)
	}); err != nil {
		return nil, err
	}
	return names, validationErrorsOrNil(errs)
}

func parseEventSpawnsRoot(decoder *xml.Decoder, root xml.StartElement, names map[string]bool, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "event" {
				if err := parseEventSpawnEvent(decoder, value, names, errs); err != nil {
					return err
				}
				continue
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func parseEventSpawnEvent(decoder *xml.Decoder, start xml.StartElement, names map[string]bool, errs *ValidationErrors) error {
	name := attrValue(start, "name")
	if strings.TrimSpace(name) == "" {
		addValidationError(errs, "cfgeventspawns event missing name")
	} else {
		names[name] = true
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "pos" {
				validateEventSpawnPos(value, name, errs)
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateEventSpawnPos(start xml.StartElement, eventName string, errs *ValidationErrors) {
	for _, name := range []string{"x", "z"} {
		value := attrValue(start, name)
		if strings.TrimSpace(value) == "" {
			addValidationError(errs, "event %q pos missing %s", eventName, name)
			continue
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			addValidationError(errs, "event %q pos %s expected float", eventName, name)
		}
	}
	angleText := attrValue(start, "a")
	if strings.TrimSpace(angleText) == "" {
		return
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(angleText), 64); err != nil {
		addValidationError(errs, "event %q pos a expected float", eventName)
	}
}

func ValidateRandomPresetsFile(path string) ([]string, error) {
	_, err := randomPresetNames(path)
	return nil, err
}

func randomPresetNames(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseRandomPresetsData(data, path)
}

func ParseRandomPresetsData(data []byte, sourceName string) (map[string]bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	presets := map[string]bool{}
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "randompresets", sourceName, func(start xml.StartElement) error {
		return parseRandomPresetsRoot(decoder, start, presets, &errs)
	}); err != nil {
		return nil, err
	}
	return presets, validationErrorsOrNil(errs)
}

func parseRandomPresetsRoot(decoder *xml.Decoder, root xml.StartElement, presets map[string]bool, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "cargo" || value.Name.Local == "attachments" {
				if err := parseRandomPreset(decoder, value, presets, errs); err != nil {
					return err
				}
				continue
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func parseRandomPreset(decoder *xml.Decoder, start xml.StartElement, presets map[string]bool, errs *ValidationErrors) error {
	name := attrValue(start, "name")
	if strings.TrimSpace(name) == "" {
		addValidationError(errs, "<%s> preset missing name", start.Name.Local)
	} else {
		presets[name] = true
	}
	if chance := attrValue(start, "chance"); chance != "" {
		validateFloatAttr(errs, start.Name.Local+" chance", chance, floatPtrConfig(0), floatPtrConfig(1))
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "item" {
				validateRandomPresetItem(value, name, errs)
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateRandomPresetItem(start xml.StartElement, presetName string, errs *ValidationErrors) {
	if strings.TrimSpace(attrValue(start, "name")) == "" {
		addValidationError(errs, "random preset %q item missing name", presetName)
	}
	if chance := attrValue(start, "chance"); chance != "" {
		validateFloatAttr(errs, "random preset "+presetName+" item chance", chance, nil, nil)
	}
}

func ValidateSpawnableTypesFile(path string) ([]string, error) {
	_, err := spawnablePresetRefs(path)
	return nil, err
}

func spawnablePresetRefs(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseSpawnableTypesData(data, path)
}

func ParseSpawnableTypesData(data []byte, sourceName string) (map[string]bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	refs := map[string]bool{}
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "spawnabletypes", sourceName, func(start xml.StartElement) error {
		return parseSpawnableTypesRoot(decoder, start, refs, &errs)
	}); err != nil {
		return nil, err
	}
	return refs, validationErrorsOrNil(errs)
}

func parseSpawnableTypesRoot(decoder *xml.Decoder, root xml.StartElement, refs map[string]bool, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "type" {
				if err := parseSpawnableType(decoder, value, refs, errs); err != nil {
					return err
				}
				continue
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func parseSpawnableType(decoder *xml.Decoder, start xml.StartElement, refs map[string]bool, errs *ValidationErrors) error {
	typeName := attrValue(start, "name")
	if strings.TrimSpace(typeName) == "" {
		addValidationError(errs, "spawnable type missing name")
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "attachments", "cargo":
				if err := parseSpawnableContainer(decoder, value, typeName, refs, errs); err != nil {
					return err
				}
			case "hoarder":
				if err := parseSpawnableHoarder(decoder, value, typeName, errs); err != nil {
					return err
				}
			default:
				if err := skipXMLElement(decoder, value.Name.Local); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func parseSpawnableContainer(decoder *xml.Decoder, start xml.StartElement, typeName string, refs map[string]bool, errs *ValidationErrors) error {
	preset := attrValue(start, "preset")
	chance := attrValue(start, "chance")
	if strings.TrimSpace(preset) == "" && strings.TrimSpace(chance) == "" {
		addValidationError(errs, "spawnable type %q <%s> expected preset or chance", typeName, start.Name.Local)
	}
	if strings.TrimSpace(preset) != "" {
		refs[preset] = true
	}
	if chance != "" {
		validateFloatAttr(errs, "spawnable type "+typeName+" "+start.Name.Local+" chance", chance, floatPtrConfig(0), floatPtrConfig(1))
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "item" {
				validateSpawnableItem(value, typeName, errs)
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func parseSpawnableHoarder(decoder *xml.Decoder, start xml.StartElement, typeName string, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "item" {
				if strings.TrimSpace(attrValue(value, "name")) == "" {
					addValidationError(errs, "spawnable type %q hoarder item missing name", typeName)
				}
				if count := attrValue(value, "count_in_hoarder"); count != "" {
					validateZeroOne(errs, "spawnable type "+typeName+" hoarder count_in_hoarder", count)
				}
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func validateSpawnableItem(start xml.StartElement, typeName string, errs *ValidationErrors) {
	if strings.TrimSpace(attrValue(start, "name")) == "" {
		addValidationError(errs, "spawnable type %q item missing name", typeName)
	}
	if chance := attrValue(start, "chance"); chance != "" {
		validateFloatAttr(errs, "spawnable type "+typeName+" item chance", chance, nil, nil)
	}
}

func ValidatePlayerSpawnPointsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nil, ValidatePlayerSpawnPointsData(data, path)
}

func ValidatePlayerSpawnPointsData(data []byte, sourceName string) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "playerspawnpoints", sourceName, func(start xml.StartElement) error {
		return parsePlayerSpawnRoot(decoder, start, &errs)
	}); err != nil {
		return err
	}
	return validationErrorsOrNil(errs)
}

func parsePlayerSpawnRoot(decoder *xml.Decoder, root xml.StartElement, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			validatePlayerSpawnElement(value, errs)
			if err := parsePlayerSpawnRoot(decoder, value, errs); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func validatePlayerSpawnElement(start xml.StartElement, errs *ValidationErrors) {
	switch start.Name.Local {
	case "generator":
		if attrValue(start, "type") == "" {
			addValidationError(errs, "generator missing type")
		}
	case "spawn":
		if id := attrValue(start, "id"); id != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(id)); err != nil {
				addValidationError(errs, "spawn id expected integer")
			}
		}
	case "pos":
		for _, name := range []string{"x", "z"} {
			value := attrValue(start, name)
			if strings.TrimSpace(value) == "" {
				addValidationError(errs, "spawn pos missing %s", name)
				continue
			}
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				addValidationError(errs, "spawn pos %s expected float", name)
			}
		}
	}
}

func ValidateEnvironmentFile(path string) ([]string, error) {
	_, _, err := environmentFileRefs(path)
	return nil, err
}

func environmentFileRefs(path string) ([]string, map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseEnvironmentData(data, path)
}

func ParseEnvironmentData(data []byte, sourceName string) ([]string, map[string]bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var paths []string
	usables := map[string]bool{}
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "env", sourceName, func(start xml.StartElement) error {
		return parseEnvironmentRoot(decoder, start, &paths, usables, &errs)
	}); err != nil {
		return nil, nil, err
	}
	return paths, usables, validationErrorsOrNil(errs)
}

func parseEnvironmentRoot(decoder *xml.Decoder, root xml.StartElement, paths *[]string, usables map[string]bool, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			validateEnvironmentElement(value, paths, usables, errs)
			if err := parseEnvironmentRoot(decoder, value, paths, usables, errs); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func validateEnvironmentElement(start xml.StartElement, paths *[]string, usables map[string]bool, errs *ValidationErrors) {
	switch start.Name.Local {
	case "territory":
		if territoryType := attrValue(start, "type"); territoryType != "" && strings.TrimSpace(territoryType) == "" {
			addValidationError(errs, "territory type expected string")
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "x" || attr.Name.Local == "z" || attr.Name.Local == "width" || attr.Name.Local == "height" {
				if _, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err != nil {
					addValidationError(errs, "territory %s expected integer", attr.Name.Local)
				}
			}
		}
	case "file":
		if path := attrValue(start, "path"); path != "" {
			*paths = append(*paths, path)
		}
		if usable := attrValue(start, "usable"); usable != "" {
			usables[usable] = true
		}
		if attrValue(start, "path") == "" && attrValue(start, "usable") == "" {
			addValidationError(errs, "file expected path or usable")
		}
	}
}

func ValidateEffectAreaFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nil, ValidateEffectAreaData(data, path)
}

func ValidateEffectAreaData(data []byte, sourceName string) error {
	value, err := parseStrictJSON(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", sourceName, err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("parse %s: root must be an object", sourceName)
	}
	var errs ValidationErrors
	validateEffectAreaRoot(root, &errs)
	return validationErrorsOrNil(errs)
}

func validateEffectAreaRoot(root map[string]any, errs *ValidationErrors) {
	areasValue, exists := root["Areas"]
	if exists {
		areas, ok := areasValue.([]any)
		if !ok {
			addValidationError(errs, "Areas expected array")
		} else {
			for index, area := range areas {
				validateEffectArea(area, index, errs)
			}
		}
	}
}

func validateEffectArea(area any, index int, errs *ValidationErrors) {
	object, ok := area.(map[string]any)
	if !ok {
		addValidationError(errs, "Areas[%d] expected object", index)
		return
	}
	validateOptionalString(object, "AreaName", fmt.Sprintf("Areas[%d].AreaName", index), errs)
	validateOptionalEnum(object, "Type", fmt.Sprintf("Areas[%d].Type", index), []string{"ContaminatedArea_Static", "ContaminatedArea_Dynamic"}, errs)
	validateOptionalEnum(object, "TriggerType", fmt.Sprintf("Areas[%d].TriggerType", index), []string{"ContaminatedTrigger"}, errs)
	if data, ok := object["Data"]; ok {
		validateEffectAreaDataObject(data, index, errs)
	}
	if playerData, ok := object["PlayerData"]; ok {
		validateEffectAreaPlayerData(playerData, index, errs)
	}
}

func validateEffectAreaDataObject(value any, index int, errs *ValidationErrors) {
	object, ok := value.(map[string]any)
	if !ok {
		addValidationError(errs, "Areas[%d].Data expected object", index)
		return
	}
	validateNumberArray(object, "Pos", 3, fmt.Sprintf("Areas[%d].Data.Pos", index), errs)
	for _, name := range []string{"Radius", "PosHeight", "NegHeight", "VerticalOffset"} {
		validateOptionalNumber(object, name, fmt.Sprintf("Areas[%d].Data.%s", index, name), errs)
	}
	for _, name := range []string{"InnerRingRatio", "OuterRingRatio"} {
		validateOptionalNumberRange(object, name, fmt.Sprintf("Areas[%d].Data.%s", index, name), floatPtrConfig(0), floatPtrConfig(1), errs)
	}
}

func validateEffectAreaPlayerData(value any, index int, errs *ValidationErrors) {
	object, ok := value.(map[string]any)
	if !ok {
		addValidationError(errs, "Areas[%d].PlayerData expected object", index)
		return
	}
	for _, name := range []string{"AroundPartName", "TinyPartName", "PPERequesterType"} {
		validateOptionalString(object, name, fmt.Sprintf("Areas[%d].PlayerData.%s", index, name), errs)
	}
}

func ValidateIgnoreListFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nil, ValidateIgnoreListData(data, path)
}

func ValidateIgnoreListData(data []byte, sourceName string) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var errs ValidationErrors
	if err := walkXMLRoot(decoder, "ignore", sourceName, func(start xml.StartElement) error {
		return parseIgnoreRoot(decoder, start, &errs)
	}); err != nil {
		return err
	}
	return validationErrorsOrNil(errs)
}

func parseIgnoreRoot(decoder *xml.Decoder, root xml.StartElement, errs *ValidationErrors) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "type" && strings.TrimSpace(attrValue(value, "name")) == "" {
				addValidationError(errs, "ignore type missing name")
			}
			if err := skipXMLElement(decoder, value.Name.Local); err != nil {
				return err
			}
		case xml.EndElement:
			if value.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func walkXMLRoot(decoder *xml.Decoder, expectedRoot string, sourceName string, handleRoot func(xml.StartElement) error) error {
	seenRoot := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("parse %s: %w", sourceName, err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if seenRoot {
				return fmt.Errorf("parse %s: multiple root elements", sourceName)
			}
			seenRoot = true
			if value.Name.Local != expectedRoot {
				return fmt.Errorf("parse %s: expected <%s> root, got <%s>", sourceName, expectedRoot, value.Name.Local)
			}
			if err := handleRoot(value); err != nil {
				if errors.Is(err, io.EOF) {
					return fmt.Errorf("parse %s: %w", sourceName, err)
				}
				return fmt.Errorf("parse %s: %w", sourceName, err)
			}
		case xml.CharData:
			if strings.TrimSpace(strings.TrimLeft(string(value), "\ufeff")) != "" {
				return fmt.Errorf("parse %s: unexpected text outside root", sourceName)
			}
		}
	}
	if !seenRoot {
		return fmt.Errorf("parse %s: missing <%s> root", sourceName, expectedRoot)
	}
	return nil
}

func readTextElement(decoder *xml.Decoder, elementName string) (string, error) {
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.CharData:
			builder.Write([]byte(value))
		case xml.StartElement:
			return "", fmt.Errorf("<%s> expected text, got <%s>", elementName, value.Name.Local)
		case xml.EndElement:
			if value.Name.Local == elementName {
				return builder.String(), nil
			}
		}
	}
}

func skipXMLElement(decoder *xml.Decoder, elementName string) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func addValidationError(errs *ValidationErrors, format string, values ...any) {
	*errs = append(*errs, fmt.Sprintf(format, values...))
}

func validationErrorsOrNil(errs ValidationErrors) error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateZeroOne(errs *ValidationErrors, label string, value string) {
	if !isZeroOne(strings.TrimSpace(value)) {
		addValidationError(errs, "%s expected 0 or 1", label)
	}
}

func isZeroOne(value string) bool {
	return value == "0" || value == "1"
}

func validateFloatAttr(errs *ValidationErrors, label string, value string, min *float64, max *float64) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		addValidationError(errs, "%s expected float", label)
		return
	}
	validateFloatRange(errs, label, parsed, min, max)
}

func validateFloatRange(errs *ValidationErrors, label string, value float64, min *float64, max *float64) {
	if min != nil && value < *min {
		addValidationError(errs, "%s must be >= %s", label, formatFloat(*min))
	}
	if max != nil && value > *max {
		addValidationError(errs, "%s must be <= %s", label, formatFloat(*max))
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func floatPtrConfig(value float64) *float64 {
	return &value
}

func intPtrConfig(value int) *int {
	return &value
}

func parseStrictJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := parseJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	token, err := decoder.Token()
	if err == nil {
		return nil, fmt.Errorf("expected one JSON document, found %v", token)
	}
	if !errors.Is(err, io.EOF) {
		return nil, err
	}
	return value, nil
}

func parseJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			return parseJSONObject(decoder)
		case '[':
			return parseJSONArray(decoder)
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	case json.Number:
		return value, nil
	case string, bool, nil:
		return value, nil
	default:
		return nil, fmt.Errorf("unexpected JSON token %v", value)
	}
}

func parseJSONObject(decoder *json.Decoder) (map[string]any, error) {
	object := map[string]any{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key, got %v", token)
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		value, err := parseJSONValue(decoder)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	if token, err := decoder.Token(); err != nil {
		return nil, err
	} else if token != json.Delim('}') {
		return nil, fmt.Errorf("expected object end, got %v", token)
	}
	return object, nil
}

func parseJSONArray(decoder *json.Decoder) ([]any, error) {
	var array []any
	for decoder.More() {
		value, err := parseJSONValue(decoder)
		if err != nil {
			return nil, err
		}
		array = append(array, value)
	}
	if token, err := decoder.Token(); err != nil {
		return nil, err
	} else if token != json.Delim(']') {
		return nil, fmt.Errorf("expected array end, got %v", token)
	}
	return array, nil
}

func validateOptionalString(object map[string]any, key string, label string, errs *ValidationErrors) {
	value, exists := object[key]
	if !exists {
		return
	}
	if _, ok := value.(string); !ok {
		addValidationError(errs, "%s expected string", label)
	}
}

func validateOptionalEnum(object map[string]any, key string, label string, allowed []string, errs *ValidationErrors) {
	value, exists := object[key]
	if !exists {
		return
	}
	text, ok := value.(string)
	if !ok {
		addValidationError(errs, "%s expected string", label)
		return
	}
	for _, allowedValue := range allowed {
		if text == allowedValue {
			return
		}
	}
	addValidationError(errs, "%s has invalid value %q", label, text)
}

func validateOptionalNumber(object map[string]any, key string, label string, errs *ValidationErrors) {
	if value, exists := object[key]; exists && !isJSONNumber(value) {
		addValidationError(errs, "%s expected number", label)
	}
}

func validateOptionalNumberRange(object map[string]any, key string, label string, min *float64, max *float64, errs *ValidationErrors) {
	value, exists := object[key]
	if !exists {
		return
	}
	number, ok := value.(json.Number)
	if !ok {
		addValidationError(errs, "%s expected number", label)
		return
	}
	floatValue, err := number.Float64()
	if err != nil {
		addValidationError(errs, "%s expected number", label)
		return
	}
	validateFloatRange(errs, label, floatValue, min, max)
}

func validateNumberArray(object map[string]any, key string, length int, label string, errs *ValidationErrors) {
	value, exists := object[key]
	if !exists {
		return
	}
	array, ok := value.([]any)
	if !ok {
		addValidationError(errs, "%s expected array", label)
		return
	}
	if len(array) != length {
		addValidationError(errs, "%s expected %d values", label, length)
		return
	}
	for _, item := range array {
		if !isJSONNumber(item) {
			addValidationError(errs, "%s expected numeric values", label)
			return
		}
	}
}

func isJSONNumber(value any) bool {
	_, ok := value.(json.Number)
	return ok
}
