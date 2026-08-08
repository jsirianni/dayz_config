package economy

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dzcli/internal/economyconfig"
)

type FileMutation struct {
	Data    []byte
	Mode    fs.FileMode
	Changed bool
}

type CollectionUpdate struct {
	Set    []string
	Add    []string
	Remove []string
	Clear  bool
}

type TypeUpdateOptions struct {
	TypeName       string
	Occurrence     int
	OccurrenceSet  bool
	Rename         string
	Scalars        map[string]int
	Flags          map[string]string
	RemoveFlags    []string
	RemoveAllFlags bool
	Collections    map[string]CollectionUpdate
}

type LimitAction string

const (
	LimitAdd    LimitAction = "add"
	LimitRemove LimitAction = "remove"
)

type UserGroupAction string

const (
	UserGroupAdd          UserGroupAction = "group-add"
	UserGroupRemove       UserGroupAction = "group-remove"
	UserGroupMemberAdd    UserGroupAction = "member-add"
	UserGroupMemberRemove UserGroupAction = "member-remove"
)

type UserLimitGroupOptions struct {
	Kind      string
	GroupName string
	Members   []string
	Member    string
	Action    UserGroupAction
}

type UserLimitGroup struct {
	Name    string
	Members []string
}

type elementRange struct {
	Start       int
	StartTagEnd int
	End         int
}

type editableType struct {
	Name          string
	Scalars       map[string]string
	ScalarPresent map[string]bool
	Flags         map[string]string
	FlagsPresent  bool
	Collections   map[string][]string
}

type listSpec struct {
	Section string
	Element string
}

type userGroup struct {
	Name    string
	Members []string
}

var scalarFieldOrder = []string{"nominal", "lifetime", "restock", "min", "quantmin", "quantmax", "cost"}
var collectionFieldOrder = []string{"category", "tag", "usage", "value"}
var flagFieldOrder = []string{"count_in_cargo", "count_in_hoarder", "count_in_map", "count_in_player", "crafted", "deloot"}

var baseLimitSpecs = map[string]listSpec{
	"category": {Section: "categories", Element: "category"},
	"tag":      {Section: "tags", Element: "tag"},
	"usage":    {Section: "usageflags", Element: "usage"},
	"value":    {Section: "valueflags", Element: "value"},
}

var userLimitSpecs = map[string]listSpec{
	"usage": {Section: "usageflags", Element: "usage"},
	"value": {Section: "valueflags", Element: "value"},
}

var parseTypesData = economyconfig.ParseTypesData
var parseLimitsDefinitionData = economyconfig.ParseLimitsDefinitionData
var appendUserLimitsDefinitionData = economyconfig.AppendUserLimitsDefinitionData
var decodeEditableTypeFunc = decodeEditableType
var decodeFlatListNamesFunc = decodeFlatListNames

func UpdateTypesFile(path string, options TypeUpdateOptions) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	updated, changed, err := UpdateTypesXML(data, options)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: updated, Mode: mode, Changed: changed}, nil
}

func ResolveTypesFileForType(economyCorePath string, typeName string) (string, error) {
	if strings.TrimSpace(typeName) == "" {
		return "", fmt.Errorf("type name is required")
	}
	core, err := economyconfig.ParseEconomyCoreFile(economyCorePath)
	if err != nil {
		return "", err
	}
	missionDir := filepath.Dir(filepath.Clean(economyCorePath))
	typeFiles := []string{filepath.Join(missionDir, "db", "types.xml")}
	for _, ref := range core.TypeFileRefs() {
		typeFiles = append(typeFiles, filepath.Join(missionDir, ref.Folder, ref.Name))
	}

	var matches []string
	for _, path := range typeFiles {
		types, err := economyconfig.ParseTypesFile(path)
		if err != nil {
			return "", err
		}
		for _, entry := range types.Types {
			if entry.Name == typeName {
				matches = append(matches, path)
				break
			}
		}
	}

	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("type %q not found in %s", typeName, economyCorePath)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("type %q appears in %d files; provide --file", typeName, len(matches))
	}
}

func UpdateTypesXML(data []byte, options TypeUpdateOptions) ([]byte, bool, error) {
	if strings.TrimSpace(options.TypeName) == "" {
		return nil, false, fmt.Errorf("type name is required")
	}
	if options.OccurrenceSet && options.Occurrence < 1 {
		return nil, false, fmt.Errorf("occurrence must be greater than 0")
	}
	if _, err := parseTypesData(data, "types.xml"); err != nil {
		return nil, false, err
	}

	ranges, err := findTypeRanges(data, options.TypeName)
	if err != nil {
		return nil, false, err
	}
	if len(ranges) == 0 {
		return nil, false, fmt.Errorf("type %q not found", options.TypeName)
	}
	if len(ranges) > 1 && !options.OccurrenceSet {
		return nil, false, fmt.Errorf("type %q appears %d times; use --occurrence to select one", options.TypeName, len(ranges))
	}

	index := 0
	if options.OccurrenceSet {
		if options.Occurrence > len(ranges) {
			return nil, false, fmt.Errorf("type %q occurrence %d not found", options.TypeName, options.Occurrence)
		}
		index = options.Occurrence - 1
	}

	targetRange := ranges[index]
	block := data[targetRange.Start:targetRange.End]
	entry, err := decodeEditableTypeFunc(block)
	if err != nil {
		return nil, false, err
	}

	if !applyTypeUpdates(&entry, options) {
		return data, false, nil
	}

	rendered := renderEditableType(data, targetRange, block, entry)
	updated := replaceRange(data, targetRange.Start, targetRange.End, rendered)
	if _, err := parseTypesData(updated, "types.xml"); err != nil {
		return nil, false, err
	}
	return updated, !bytes.Equal(data, updated), nil
}

func UpdateLimitsFile(path string, kind string, name string, action LimitAction) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	updated, changed, err := UpdateLimitsXML(data, kind, name, action)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: updated, Mode: mode, Changed: changed}, nil
}

func UpdateLimitsXML(data []byte, kind string, name string, action LimitAction) ([]byte, bool, error) {
	spec, ok := baseLimitSpecs[kind]
	if !ok {
		return nil, false, fmt.Errorf("unsupported limits kind %q", kind)
	}
	if strings.TrimSpace(name) == "" {
		return nil, false, fmt.Errorf("%s name is required", kind)
	}
	if action != LimitAdd && action != LimitRemove {
		return nil, false, fmt.Errorf("unsupported limits action %q", action)
	}
	if _, err := parseLimitsDefinitionData(data, "cfglimitsdefinition.xml"); err != nil {
		return nil, false, err
	}

	updated, changed, err := updateFlatListSection(data, spec, name, action)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return data, false, nil
	}
	if _, err := parseLimitsDefinitionData(updated, "cfglimitsdefinition.xml"); err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

func UpdateUserLimitGroupFile(path string, options UserLimitGroupOptions) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	updated, changed, err := UpdateUserLimitGroupXML(data, options)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: updated, Mode: mode, Changed: changed}, nil
}

func UpdateUserLimitGroupXML(data []byte, options UserLimitGroupOptions) ([]byte, bool, error) {
	spec, ok := userLimitSpecs[options.Kind]
	if !ok {
		return nil, false, fmt.Errorf("unsupported user limits kind %q; only usage and value are supported", options.Kind)
	}
	if strings.TrimSpace(options.GroupName) == "" {
		return nil, false, fmt.Errorf("%s group name is required", options.Kind)
	}
	if options.Action != UserGroupAdd && options.Action != UserGroupRemove && options.Action != UserGroupMemberAdd && options.Action != UserGroupMemberRemove {
		return nil, false, fmt.Errorf("unsupported user group action %q", options.Action)
	}
	if err := validateUserLimitsData(data); err != nil {
		return nil, false, err
	}

	updated, changed, err := updateUserGroupSection(data, spec, options)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return data, false, nil
	}
	if err := validateUserLimitsData(updated); err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

func WriteFileMutation(path string, mutation FileMutation) error {
	if !mutation.Changed {
		return nil
	}
	return os.WriteFile(path, mutation.Data, mutation.Mode)
}

func ListLimitNamesFile(path string, kind string) ([]string, error) {
	limits, err := economyconfig.ParseLimitsDefinitionFile(path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "category":
		return sortedMapKeys(limits.Categories), nil
	case "tag":
		return sortedMapKeys(limits.Tags), nil
	case "usage":
		return sortedMapKeys(limits.Usages), nil
	case "value":
		return sortedMapKeys(limits.Values), nil
	default:
		return nil, fmt.Errorf("unsupported limits kind %q", kind)
	}
}

func ListUserLimitGroupsFile(path string, kind string) ([]UserLimitGroup, error) {
	spec, ok := userLimitSpecs[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported user limits kind %q; only usage and value are supported", kind)
	}
	data, _, err := readMutableFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateUserLimitsData(data); err != nil {
		return nil, err
	}
	targetRange, err := findElementRange(data, spec.Section)
	if err != nil {
		return nil, err
	}
	groups, err := decodeUserGroups(data[targetRange.Start:targetRange.End], spec.Element)
	if err != nil {
		return nil, err
	}
	result := make([]UserLimitGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, UserLimitGroup{Name: group.Name, Members: append([]string(nil), group.Members...)})
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Name < result[right].Name
	})
	return result, nil
}

func readMutableFile(path string) ([]byte, fs.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

func validateUserLimitsData(data []byte) error {
	limits := economyconfig.LimitsDefinition{
		Categories: map[string]bool{},
		Tags:       map[string]bool{},
		Usages:     map[string]bool{},
		Values:     map[string]bool{},
	}
	return appendUserLimitsDefinitionData(data, "cfglimitsdefinitionuser.xml", &limits)
}

func findTypeRanges(data []byte, typeName string) ([]elementRange, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var ranges []elementRange
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return ranges, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "type" {
			continue
		}

		startEnd := int(decoder.InputOffset())
		startOffset, err := findStartOffset(data, startEnd, start.Name.Local)
		if err != nil {
			return nil, err
		}
		endOffset, err := consumeElement(decoder, start.Name.Local)
		if err != nil {
			return nil, err
		}
		if attributeValue(start, "name") == typeName {
			ranges = append(ranges, elementRange{Start: startOffset, StartTagEnd: startEnd, End: endOffset})
		}
	}
}

func findElementRange(data []byte, elementName string) (elementRange, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return elementRange{}, fmt.Errorf("<%s> section not found", elementName)
			}
			return elementRange{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != elementName {
			continue
		}
		startEnd := int(decoder.InputOffset())
		startOffset, err := findStartOffset(data, startEnd, start.Name.Local)
		if err != nil {
			return elementRange{}, err
		}
		endOffset, err := consumeElement(decoder, start.Name.Local)
		if err != nil {
			return elementRange{}, err
		}
		return elementRange{Start: startOffset, StartTagEnd: startEnd, End: endOffset}, nil
	}
}

func findStartOffset(data []byte, startTagEnd int, elementName string) (int, error) {
	prefix := []byte("<" + elementName)
	for index := startTagEnd - len(prefix); index >= 0; index-- {
		if !bytes.HasPrefix(data[index:], prefix) {
			continue
		}
		after := index + len(prefix)
		if after < len(data) && (data[after] == ' ' || data[after] == '\t' || data[after] == '\r' || data[after] == '\n' || data[after] == '>' || data[after] == '/') {
			return index, nil
		}
	}
	return 0, fmt.Errorf("could not locate <%s> start offset", elementName)
}

func consumeElement(decoder *xml.Decoder, elementName string) (int, error) {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return 0, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && typed.Name.Local == elementName {
				return int(decoder.InputOffset()), nil
			}
		}
	}
	return 0, fmt.Errorf("could not consume <%s>", elementName)
}

func decodeEditableType(block []byte) (editableType, error) {
	decoder := xml.NewDecoder(bytes.NewReader(block))
	entry := editableType{
		Scalars:       map[string]string{},
		ScalarPresent: map[string]bool{},
		Flags:         map[string]string{},
		Collections:   map[string][]string{},
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return editableType{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "type" {
			return editableType{}, fmt.Errorf("expected <type>, got <%s>", start.Name.Local)
		}
		entry.Name = attributeValue(start, "name")
		return decodeTypeChildren(decoder, start.Name.Local, entry)
	}
}

func decodeTypeChildren(decoder *xml.Decoder, rootName string, entry editableType) (editableType, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return editableType{}, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if containsString(scalarFieldOrder, typed.Name.Local) {
				value, err := decodeTextElement(decoder, typed.Name.Local)
				if err != nil {
					return editableType{}, err
				}
				entry.Scalars[typed.Name.Local] = strings.TrimSpace(value)
				entry.ScalarPresent[typed.Name.Local] = true
				continue
			}
			if typed.Name.Local == "flags" {
				entry.FlagsPresent = true
				for _, attr := range typed.Attr {
					entry.Flags[attr.Name.Local] = attr.Value
				}
				if err := skipElement(decoder, typed.Name.Local); err != nil {
					return editableType{}, err
				}
				continue
			}
			if containsString(collectionFieldOrder, typed.Name.Local) {
				entry.Collections[typed.Name.Local] = append(entry.Collections[typed.Name.Local], attributeValue(typed, "name"))
				if err := skipElement(decoder, typed.Name.Local); err != nil {
					return editableType{}, err
				}
				continue
			}
			if err := skipElement(decoder, typed.Name.Local); err != nil {
				return editableType{}, err
			}
		case xml.EndElement:
			if typed.Name.Local == rootName {
				return entry, nil
			}
		}
	}
}

func decodeTextElement(decoder *xml.Decoder, elementName string) (string, error) {
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch typed := token.(type) {
		case xml.CharData:
			builder.Write([]byte(typed))
		case xml.StartElement:
			if err := skipElement(decoder, typed.Name.Local); err != nil {
				return "", err
			}
		case xml.EndElement:
			if typed.Name.Local == elementName {
				return builder.String(), nil
			}
		}
	}
}

func skipElement(decoder *xml.Decoder, elementName string) error {
	_, err := consumeElement(decoder, elementName)
	return err
}

func applyTypeUpdates(entry *editableType, options TypeUpdateOptions) bool {
	changed := false
	if options.Rename != "" && entry.Name != options.Rename {
		entry.Name = options.Rename
		changed = true
	}
	for field, value := range options.Scalars {
		text := strconv.Itoa(value)
		if !entry.ScalarPresent[field] || entry.Scalars[field] != text {
			entry.Scalars[field] = text
			entry.ScalarPresent[field] = true
			changed = true
		}
	}
	if options.RemoveAllFlags && entry.FlagsPresent {
		entry.FlagsPresent = false
		entry.Flags = map[string]string{}
		changed = true
	}
	for _, flag := range options.RemoveFlags {
		if _, exists := entry.Flags[flag]; exists {
			delete(entry.Flags, flag)
			changed = true
		}
	}
	for flag, value := range options.Flags {
		if !entry.FlagsPresent {
			entry.FlagsPresent = true
		}
		if entry.Flags[flag] != value {
			entry.Flags[flag] = value
			changed = true
		}
	}
	if entry.FlagsPresent && len(entry.Flags) == 0 {
		entry.FlagsPresent = false
	}
	for field, update := range options.Collections {
		values, fieldChanged := applyCollectionUpdate(entry.Collections[field], update)
		if fieldChanged {
			entry.Collections[field] = values
			changed = true
		}
	}
	return changed
}

func applyCollectionUpdate(values []string, update CollectionUpdate) ([]string, bool) {
	original := append([]string(nil), values...)
	if update.Set != nil {
		values = append([]string(nil), update.Set...)
	}
	if update.Clear {
		values = nil
	}
	if len(update.Remove) > 0 {
		values = removeStrings(values, update.Remove)
	}
	for _, value := range update.Add {
		if !containsString(values, value) {
			values = append(values, value)
		}
	}
	return values, !equalStrings(original, values)
}

func renderEditableType(document []byte, targetRange elementRange, originalBlock []byte, entry editableType) []byte {
	lineEnding := detectLineEnding(document)
	typeIndent := detectElementIndent(document, targetRange.Start)
	childIndent := detectChildIndent(originalBlock, typeIndent)
	emptyElementEnd := detectEmptyElementEnd(originalBlock)

	lines := []string{`<type name="` + escapeAttribute(entry.Name) + `">`}
	for _, field := range scalarFieldOrder {
		if entry.ScalarPresent[field] {
			lines = append(lines, childIndent+"<"+field+">"+entry.Scalars[field]+"</"+field+">")
		}
	}
	if entry.FlagsPresent && len(entry.Flags) > 0 {
		lines = append(lines, childIndent+"<flags "+renderFlagAttributes(entry.Flags)+emptyElementEnd)
	}
	for _, field := range collectionFieldOrder {
		for _, value := range entry.Collections[field] {
			lines = append(lines, childIndent+"<"+field+` name="`+escapeAttribute(value)+`"`+emptyElementEnd)
		}
	}
	lines = append(lines, typeIndent+"</type>")
	return []byte(strings.Join(lines, lineEnding))
}

func renderFlagAttributes(flags map[string]string) string {
	var parts []string
	seen := map[string]bool{}
	for _, flag := range flagFieldOrder {
		if value, ok := flags[flag]; ok {
			parts = append(parts, flag+`="`+escapeAttribute(value)+`"`)
			seen[flag] = true
		}
	}
	var extras []string
	for flag := range flags {
		if !seen[flag] {
			extras = append(extras, flag)
		}
	}
	sort.Strings(extras)
	for _, flag := range extras {
		parts = append(parts, flag+`="`+escapeAttribute(flags[flag])+`"`)
	}
	return strings.Join(parts, " ")
}

func updateFlatListSection(data []byte, spec listSpec, name string, action LimitAction) ([]byte, bool, error) {
	targetRange, err := findElementRange(data, spec.Section)
	if err != nil {
		return nil, false, err
	}
	block := data[targetRange.Start:targetRange.End]
	names, err := decodeFlatListNamesFunc(block, spec.Element)
	if err != nil {
		return nil, false, err
	}

	updatedNames := append([]string(nil), names...)
	switch action {
	case LimitAdd:
		if containsString(updatedNames, name) {
			return data, false, nil
		}
		updatedNames = append(updatedNames, name)
	case LimitRemove:
		updatedNames = removeStrings(updatedNames, []string{name})
		if equalStrings(names, updatedNames) {
			return data, false, nil
		}
	}

	rendered := renderFlatListSection(data, targetRange, block, spec, updatedNames)
	return replaceRange(data, targetRange.Start, targetRange.End, rendered), true, nil
}

func decodeFlatListNames(block []byte, elementName string) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(block))
	var names []string
	seenSection := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return names, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !seenSection {
			seenSection = true
			continue
		}
		if start.Name.Local == elementName {
			names = append(names, attributeValue(start, "name"))
		}
		if err := skipElement(decoder, start.Name.Local); err != nil {
			return nil, err
		}
	}
}

func renderFlatListSection(document []byte, targetRange elementRange, block []byte, spec listSpec, names []string) []byte {
	lineEnding := detectLineEnding(document)
	sectionIndent := detectElementIndent(document, targetRange.Start)
	childIndent := detectChildIndent(block, sectionIndent)
	emptyElementEnd := detectEmptyElementEnd(block)
	openTag := renderOpeningTag(document, targetRange)

	lines := []string{openTag}
	for _, name := range names {
		lines = append(lines, childIndent+"<"+spec.Element+` name="`+escapeAttribute(name)+`"`+emptyElementEnd)
	}
	lines = append(lines, sectionIndent+"</"+spec.Section+">")
	return []byte(strings.Join(lines, lineEnding))
}

func updateUserGroupSection(data []byte, spec listSpec, options UserLimitGroupOptions) ([]byte, bool, error) {
	targetRange, err := findElementRange(data, spec.Section)
	if err != nil {
		return nil, false, err
	}
	block := data[targetRange.Start:targetRange.End]
	groups, err := decodeUserGroups(block, spec.Element)
	if err != nil {
		return nil, false, err
	}

	updatedGroups, changed, err := applyUserGroupUpdate(groups, options)
	if err != nil || !changed {
		return data, false, err
	}
	rendered := renderUserGroupSection(data, targetRange, block, spec, updatedGroups)
	return replaceRange(data, targetRange.Start, targetRange.End, rendered), true, nil
}

func applyUserGroupUpdate(groups []userGroup, options UserLimitGroupOptions) ([]userGroup, bool, error) {
	index := findGroupIndex(groups, options.GroupName)
	switch options.Action {
	case UserGroupAdd:
		if index < 0 {
			return append(groups, userGroup{Name: options.GroupName, Members: uniqueStrings(options.Members)}), true, nil
		}
		members, changed := addMembers(groups[index].Members, options.Members)
		groups[index].Members = members
		return groups, changed, nil
	case UserGroupRemove:
		if index < 0 {
			return groups, false, nil
		}
		return append(groups[:index], groups[index+1:]...), true, nil
	case UserGroupMemberAdd:
		if index < 0 {
			return nil, false, fmt.Errorf("%s group %q not found", options.Kind, options.GroupName)
		}
		members, changed := addMembers(groups[index].Members, []string{options.Member})
		groups[index].Members = members
		return groups, changed, nil
	case UserGroupMemberRemove:
		if index < 0 {
			return nil, false, fmt.Errorf("%s group %q not found", options.Kind, options.GroupName)
		}
		members := removeStrings(groups[index].Members, []string{options.Member})
		changed := !equalStrings(groups[index].Members, members)
		groups[index].Members = members
		return groups, changed, nil
	}
	return nil, false, fmt.Errorf("unsupported user group action %q", options.Action)
}

func decodeUserGroups(block []byte, memberElement string) ([]userGroup, error) {
	decoder := xml.NewDecoder(bytes.NewReader(block))
	var groups []userGroup
	seenSection := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return groups, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !seenSection {
			seenSection = true
			continue
		}
		if start.Name.Local == "user" {
			group, err := decodeUserGroup(decoder, start, memberElement)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
			continue
		}
		if err := skipElement(decoder, start.Name.Local); err != nil {
			return nil, err
		}
	}
}

func decodeUserGroup(decoder *xml.Decoder, start xml.StartElement, memberElement string) (userGroup, error) {
	group := userGroup{Name: attributeValue(start, "name")}
	for {
		token, err := decoder.Token()
		if err != nil {
			return userGroup{}, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local != memberElement {
				return userGroup{}, fmt.Errorf("<user> expected <%s>, got <%s>", memberElement, typed.Name.Local)
			}
			group.Members = append(group.Members, attributeValue(typed, "name"))
			if err := skipElement(decoder, typed.Name.Local); err != nil {
				return userGroup{}, err
			}
		case xml.EndElement:
			if typed.Name.Local == start.Name.Local {
				return group, nil
			}
		}
	}
}

func renderUserGroupSection(document []byte, targetRange elementRange, block []byte, spec listSpec, groups []userGroup) []byte {
	lineEnding := detectLineEnding(document)
	sectionIndent := detectElementIndent(document, targetRange.Start)
	groupIndent := detectChildIndent(block, sectionIndent)
	memberIndent := detectGrandchildIndent(block, groupIndent)
	emptyElementEnd := detectEmptyElementEnd(block)
	openTag := renderOpeningTag(document, targetRange)

	lines := []string{openTag}
	for _, group := range groups {
		lines = append(lines, groupIndent+`<user name="`+escapeAttribute(group.Name)+`">`)
		for _, member := range group.Members {
			lines = append(lines, memberIndent+"<"+spec.Element+` name="`+escapeAttribute(member)+`"`+emptyElementEnd)
		}
		lines = append(lines, groupIndent+"</user>")
	}
	lines = append(lines, sectionIndent+"</"+spec.Section+">")
	return []byte(strings.Join(lines, lineEnding))
}

func detectLineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func renderOpeningTag(document []byte, targetRange elementRange) string {
	openTag := string(document[targetRange.Start:targetRange.StartTagEnd])
	trimmed := strings.TrimRight(openTag, " \t\r\n")
	if strings.HasSuffix(trimmed, "/>") {
		return strings.TrimRight(strings.TrimSuffix(trimmed, "/>"), " \t\r\n") + ">"
	}
	return openTag
}

func detectElementIndent(data []byte, start int) string {
	lineStart := bytes.LastIndexByte(data[:start], '\n')
	if lineStart < 0 {
		return ""
	}
	return leadingWhitespace(string(data[lineStart+1 : start]))
}

func detectChildIndent(block []byte, fallback string) string {
	lines := strings.Split(string(block), detectLineEnding(block))
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "</") {
			return leadingWhitespace(line)
		}
	}
	return fallback + "  "
}

func detectGrandchildIndent(block []byte, fallback string) string {
	lines := strings.Split(string(block), detectLineEnding(block))
	sawUser := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<user") {
			sawUser = true
			continue
		}
		if sawUser && strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "</") {
			return leadingWhitespace(line)
		}
	}
	return fallback + "  "
}

func detectEmptyElementEnd(block []byte) string {
	if bytes.Contains(block, []byte(" />")) {
		return " />"
	}
	return "/>"
}

func leadingWhitespace(value string) string {
	for index, char := range value {
		if char != ' ' && char != '\t' {
			return value[:index]
		}
	}
	return value
}

func replaceRange(data []byte, start int, end int, replacement []byte) []byte {
	updated := make([]byte, 0, len(data)-(end-start)+len(replacement))
	updated = append(updated, data[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, data[end:]...)
	return updated
}

func attributeValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func escapeAttribute(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(value)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeStrings(values []string, removals []string) []string {
	var filtered []string
	for _, value := range values {
		if !containsString(removals, value) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	var unique []string
	for _, value := range values {
		if !containsString(unique, value) {
			unique = append(unique, value)
		}
	}
	return unique
}

func addMembers(values []string, additions []string) ([]string, bool) {
	updated := append([]string(nil), values...)
	for _, addition := range additions {
		if !containsString(updated, addition) {
			updated = append(updated, addition)
		}
	}
	return updated, !equalStrings(values, updated)
}

func findGroupIndex(groups []userGroup, groupName string) int {
	for index, group := range groups {
		if group.Name == groupName {
			return index
		}
	}
	return -1
}

func sortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
