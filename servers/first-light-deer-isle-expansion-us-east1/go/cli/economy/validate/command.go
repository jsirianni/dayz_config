package validate

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"dzcli/cli/validation"

	"github.com/spf13/cobra"
)

type FileStatus struct {
	Path      string
	Kind      string
	TypeCount int
	Warnings  []string
	Err       error
}

func NewCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <cfgeconomycore.xml>",
		Short: "Validate cfgeconomycore.xml and referenced types files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateEconomyCore(args[0], stdout)
		},
	}
}

func ValidateEconomyCore(economyCorePath string, stdout io.Writer) error {
	statuses, err := InspectEconomyCore(economyCorePath)
	if err != nil {
		return fmt.Errorf("cfgeconomycore: failed: %w", err)
	}

	allOK := true
	for _, status := range statuses {
		if status.Err != nil {
			allOK = false
			fmt.Fprintf(stdout, "%s %s failed: %v\n", status.Kind, status.Path, status.Err)
			continue
		}
		if status.Kind == "cfgeconomycore" {
			fmt.Fprintf(stdout, "%s %s ok\n", status.Kind, status.Path)
			printWarnings(stdout, status)
			continue
		}
		fmt.Fprintf(stdout, "%s %s ok (%d types)\n", status.Kind, status.Path, status.TypeCount)
		printWarnings(stdout, status)
	}

	if !allOK {
		return validation.ErrFailed
	}
	return nil
}

func printWarnings(stdout io.Writer, status FileStatus) {
	for _, warning := range status.Warnings {
		fmt.Fprintf(stdout, "%s %s warning: %s\n", status.Kind, status.Path, warning)
	}
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
