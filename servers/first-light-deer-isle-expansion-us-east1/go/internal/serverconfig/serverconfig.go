package serverconfig

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type FileMutation struct {
	Data    []byte
	Mode    fs.FileMode
	Changed bool
}

type FieldValue struct {
	Field string
	Value string
}

type UpdateFieldOptions struct {
	AllowUnknown bool
}

const unknownFieldAnnotation = "// #dzcli:allow-unknown"

type Document struct {
	Data        []byte
	Assignments []Assignment
	Classes     []ClassBlock
	LineEnding  string
}

type Assignment struct {
	Field string
	Key   string
	Value Value
	Start int
	End   int
}

type ClassBlock struct {
	Path       string
	Start      int
	CloseStart int
	End        int
}

type ValueKind int

const (
	ValueString ValueKind = iota
	ValueNumber
	ValueBool
	ValueArray
)

type Value struct {
	Kind   ValueKind
	Text   string
	String string
	Bool   bool
	Array  []Value
}

type ValidationErrors []string

func (errs ValidationErrors) Error() string {
	return strings.Join(errs, "; ")
}

type fieldType int

const (
	fieldString fieldType = iota
	fieldStringEnum
	fieldStringArray
	fieldBool
	fieldIntBool
	fieldFixedInt
	fieldInt
	fieldFloatRange
	fieldFloatMin
	fieldIntEnum
)

type fieldSpec struct {
	Name        string
	RenderKey   string
	Type        fieldType
	StringEnum  []string
	IntEnum     []int
	FixedInt    int
	Min         float64
	Max         float64
	Description string
}

var documentedFields = []fieldSpec{
	{Name: "hostname", Type: fieldString},
	{Name: "description", Type: fieldString},
	{Name: "password", Type: fieldString},
	{Name: "passwordAdmin", Type: fieldString},
	{Name: "enableWhitelist", Type: fieldIntBool},
	{Name: "disableBanlist", Type: fieldBool},
	{Name: "disablePrioritylist", Type: fieldBool},
	{Name: "maxPlayers", Type: fieldInt},
	{Name: "verifySignatures", Type: fieldFixedInt, FixedInt: 2},
	{Name: "forceSameBuild", Type: fieldIntBool},
	{Name: "disableVoN", Type: fieldIntBool},
	{Name: "vonCodecQuality", Type: fieldIntEnum, IntEnum: intRange(0, 30)},
	{Name: "disable3rdPerson", Type: fieldIntBool},
	{Name: "disableCrosshair", Type: fieldIntBool},
	{Name: "serverTime", Type: fieldString},
	{Name: "serverTimeAcceleration", Type: fieldFloatRange, Min: 0.1, Max: 64},
	{Name: "serverNightTimeAcceleration", Type: fieldFloatRange, Min: 0.1, Max: 64},
	{Name: "serverTimePersistent", Type: fieldIntBool},
	{Name: "guaranteedUpdates", Type: fieldFixedInt, FixedInt: 1},
	{Name: "loginQueueConcurrentPlayers", Type: fieldInt},
	{Name: "loginQueueMaxPlayers", Type: fieldInt},
	{Name: "instanceId", Type: fieldInt},
	{Name: "storageAutoFix", Type: fieldIntBool},
	{Name: "Missions.DayZ.template", RenderKey: "template", Type: fieldString},
	{Name: "respawnTime", Type: fieldInt},
	{Name: "motd", RenderKey: "motd[]", Type: fieldStringArray},
	{Name: "motdInterval", Type: fieldInt},
	{Name: "timeStampFormat", Type: fieldStringEnum, StringEnum: []string{"Full", "Short"}},
	{Name: "logAverageFps", Type: fieldInt},
	{Name: "logMemory", Type: fieldInt},
	{Name: "logPlayers", Type: fieldInt},
	{Name: "logFile", Type: fieldString},
	{Name: "adminLogPlayerHitsOnly", Type: fieldIntBool},
	{Name: "adminLogPlacement", Type: fieldIntBool},
	{Name: "adminLogBuildActions", Type: fieldIntBool},
	{Name: "adminLogPlayerList", Type: fieldIntBool},
	{Name: "disableMultiAccountMitigation", Type: fieldBool},
	{Name: "enableDebugMonitor", Type: fieldIntBool},
	{Name: "steamQueryPort", Type: fieldInt},
	{Name: "allowFilePatching", Type: fieldIntBool},
	{Name: "simulatedPlayersBatch", Type: fieldInt},
	{Name: "multithreadedReplication", Type: fieldIntBool},
	{Name: "speedhackDetection", Type: fieldFloatRange, Min: 1, Max: 10},
	{Name: "networkRangeClose", Type: fieldInt},
	{Name: "networkRangeNear", Type: fieldInt},
	{Name: "networkRangeFar", Type: fieldInt},
	{Name: "networkRangeDistantEffect", Type: fieldInt},
	{Name: "networkObjectBatchLogSlow", Type: fieldInt},
	{Name: "networkObjectBatchEnforceBandwidthLimits", Type: fieldIntBool},
	{Name: "networkObjectBatchUseEstimatedBandwidth", Type: fieldIntBool},
	{Name: "networkObjectBatchUseDynamicMaximumBandwidth", Type: fieldIntBool},
	{Name: "networkObjectBatchBandwidthLimit", Type: fieldFloatMin, Min: 0},
	{Name: "networkObjectBatchCompute", Type: fieldInt},
	{Name: "networkObjectBatchSendCreate", Type: fieldInt},
	{Name: "networkObjectBatchSendDelete", Type: fieldInt},
	{Name: "defaultVisibility", Type: fieldInt},
	{Name: "defaultObjectViewDistance", Type: fieldInt},
	{Name: "lightingConfig", Type: fieldIntEnum, IntEnum: []int{0, 1, 2}},
	{Name: "disablePersonalLight", Type: fieldIntBool},
	{Name: "disableBaseDamage", Type: fieldIntBool},
	{Name: "disableContainerDamage", Type: fieldIntBool},
	{Name: "disableRespawnDialog", Type: fieldIntBool},
	{Name: "pingWarning", Type: fieldInt},
	{Name: "pingCritical", Type: fieldInt},
	{Name: "MaxPing", Type: fieldInt},
	{Name: "serverFpsWarning", Type: fieldInt},
	{Name: "shotValidation", Type: fieldIntBool},
	{Name: "clientPort", Type: fieldInt},
}

var allowedClasses = map[string]bool{
	"Missions":      true,
	"Missions.DayZ": true,
}

var statFile = os.Stat
var readFile = os.ReadFile
var writeFile = os.WriteFile
var parseUpdatedData = ParseData

func ValidateFile(path string) error {
	data, err := readFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return ValidateData(data, path)
}

func ValidateData(data []byte, sourceName string) error {
	document, err := ParseData(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", sourceName, err)
	}
	return ValidateDocument(document)
}

func ValidateDocument(document Document) error {
	var errs ValidationErrors
	specs := fieldSpecMap()
	for _, class := range document.Classes {
		if !allowedClasses[class.Path] {
			errs = append(errs, fmt.Sprintf("unknown class %q", class.Path))
		}
	}
	for _, assignment := range document.Assignments {
		spec, ok := specs[assignment.Field]
		if !ok {
			if !hasAnnotationComment(document.Data, assignment) {
				errs = append(errs, fmt.Sprintf("unknown field %q requires annotation comment", assignment.Field))
			}
			continue
		}
		if err := validateAssignment(spec, assignment); err != nil {
			errs = append(errs, fmt.Sprintf("%s %v", assignment.Field, err))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func hasAnnotationComment(data []byte, assignment Assignment) bool {
	lineEnd := assignment.End
	for lineEnd < len(data) && data[lineEnd] != '\n' {
		lineEnd++
	}
	trailing := string(data[assignment.End:lineEnd])
	if strings.Contains(trailing, "//") || strings.Contains(trailing, "/*") {
		return true
	}

	previousStart, previousEnd := previousNonBlankLine(data, lineStart(data, assignment.Start))
	if previousStart < 0 {
		return false
	}
	previous := strings.TrimSpace(string(data[previousStart:previousEnd]))
	return strings.HasPrefix(previous, "//") || strings.HasPrefix(previous, "/*") || strings.HasPrefix(previous, "*") || strings.HasSuffix(previous, "*/")
}

func previousNonBlankLine(data []byte, offset int) (int, int) {
	for offset > 0 {
		end := offset
		if end > 0 && data[end-1] == '\n' {
			end--
		}
		if end > 0 && data[end-1] == '\r' {
			end--
		}
		start := lineStart(data, end)
		if strings.TrimSpace(string(data[start:end])) != "" {
			return start, end
		}
		offset = start
	}
	return -1, -1
}

func ListFieldsFile(path string) ([]FieldValue, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	document, err := ParseData(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	values := make([]FieldValue, 0, len(document.Assignments))
	for _, assignment := range document.Assignments {
		values = append(values, FieldValue{Field: assignment.Field, Value: DisplayValue(assignment.Value)})
	}
	return values, nil
}

func UpdateFieldFile(path string, field string, rawValues []string, options ...UpdateFieldOptions) (FileMutation, bool, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, false, err
	}
	updated, changed, existed, err := UpdateField(data, field, rawValues, options...)
	if err != nil {
		return FileMutation{}, false, err
	}
	return FileMutation{Data: updated, Mode: mode, Changed: changed}, existed, nil
}

func DeleteFieldFile(path string, field string) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	updated, changed, err := DeleteField(data, field)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: updated, Mode: mode, Changed: changed}, nil
}

func WriteFileMutation(path string, mutation FileMutation) error {
	if !mutation.Changed {
		return nil
	}
	return writeFile(path, mutation.Data, mutation.Mode)
}

func UpdateField(data []byte, field string, rawValues []string, options ...UpdateFieldOptions) ([]byte, bool, bool, error) {
	option := updateOptions(options)
	inputKey := strings.TrimSpace(field)
	field = NormalizeField(field)
	document, err := ParseData(data)
	if err != nil {
		return nil, false, false, err
	}
	ranges := matchingAssignmentRanges(document, field)
	spec, known := fieldSpecMap()[field]
	if !known && len(ranges) == 0 && !option.AllowUnknown {
		return nil, false, false, fmt.Errorf("unsupported server config field %q; pass --allow-unknown to insert it", field)
	}

	rendered, err := renderUpdatedAssignment(document, field, inputKey, rawValues, spec, known, ranges, option)
	if err != nil {
		return nil, false, false, err
	}
	if len(ranges) > 0 {
		updated := replaceRanges(data, ranges, rendered)
		if _, err := parseUpdatedData(updated); err != nil {
			return nil, false, false, err
		}
		return updated, !bytes.Equal(data, updated), true, nil
	}

	updated := insertMissingField(document, insertSpec(field, spec, known), rendered)
	if _, err := parseUpdatedData(updated); err != nil {
		return nil, false, false, err
	}
	return updated, !bytes.Equal(data, updated), false, nil
}

func updateOptions(options []UpdateFieldOptions) UpdateFieldOptions {
	if len(options) == 0 {
		return UpdateFieldOptions{}
	}
	return options[0]
}

func DeleteField(data []byte, field string) ([]byte, bool, error) {
	field = NormalizeField(field)
	document, err := ParseData(data)
	if err != nil {
		return nil, false, err
	}
	ranges := matchingAssignmentRanges(document, field)
	if len(ranges) == 0 {
		return data, false, nil
	}
	updated := removeRanges(data, ranges)
	if _, err := parseUpdatedData(updated); err != nil {
		return nil, false, err
	}
	return updated, !bytes.Equal(data, updated), nil
}

func ParseData(data []byte) (Document, error) {
	tokens, err := scanAll(data)
	if err != nil {
		return Document{}, err
	}
	parser := cfgParser{tokens: tokens, document: Document{Data: data, LineEnding: detectLineEnding(data)}}
	if err := parser.parseItems(nil, false); err != nil {
		return Document{}, err
	}
	return parser.document, nil
}

func DisplayValue(value Value) string {
	switch value.Kind {
	case ValueString:
		return value.String
	case ValueNumber:
		return value.Text
	case ValueBool:
		if value.Bool {
			return "true"
		}
		return "false"
	case ValueArray:
		parts := make([]string, 0, len(value.Array))
		for _, item := range value.Array {
			parts = append(parts, DisplayValue(item))
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

func NormalizeField(field string) string {
	field = strings.TrimSpace(field)
	return strings.TrimSuffix(field, "[]")
}

func ValueFromStrings(spec fieldSpec, rawValues []string) (Value, error) {
	if len(rawValues) == 0 {
		return Value{}, fmt.Errorf("--value is required")
	}
	if spec.Type == fieldStringArray {
		items := make([]Value, 0, len(rawValues))
		for _, raw := range rawValues {
			items = append(items, Value{Kind: ValueString, String: raw})
		}
		value := Value{Kind: ValueArray, Array: items}
		return value, validateValue(spec, value)
	}
	if len(rawValues) != 1 {
		return Value{}, fmt.Errorf("%s accepts exactly one --value", spec.Name)
	}
	return parseRawFieldValue(spec, rawValues[0])
}

func UnknownValueFromStrings(rawValues []string, existing *Value) (Value, error) {
	if len(rawValues) == 0 {
		return Value{}, fmt.Errorf("--value is required")
	}
	if existing != nil {
		return unknownValueFromExistingKind(rawValues, *existing)
	}
	if len(rawValues) > 1 {
		items := make([]Value, 0, len(rawValues))
		for _, raw := range rawValues {
			items = append(items, Value{Kind: ValueString, String: raw})
		}
		return Value{Kind: ValueArray, Array: items}, nil
	}
	return inferUnknownScalar(rawValues[0])
}

func FieldNames() []string {
	names := make([]string, 0, len(documentedFields))
	for _, spec := range documentedFields {
		names = append(names, spec.Name)
	}
	return names
}

func parseRawFieldValue(spec fieldSpec, raw string) (Value, error) {
	switch spec.Type {
	case fieldString, fieldStringEnum:
		value := Value{Kind: ValueString, String: raw}
		return value, validateValue(spec, value)
	case fieldBool:
		value, err := parseRawBool(raw)
		if err != nil {
			return Value{}, err
		}
		return value, validateValue(spec, value)
	case fieldIntBool, fieldFixedInt, fieldInt, fieldIntEnum:
		value := Value{Kind: ValueNumber, Text: raw}
		return value, validateValue(spec, value)
	case fieldFloatRange, fieldFloatMin:
		value := Value{Kind: ValueNumber, Text: raw}
		return value, validateValue(spec, value)
	default:
		return Value{}, fmt.Errorf("unsupported field type for %s", spec.Name)
	}
}

func parseRawBool(raw string) (Value, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return Value{Kind: ValueBool, Bool: true}, nil
	case "false":
		return Value{Kind: ValueBool}, nil
	default:
		return Value{}, fmt.Errorf("expected true or false, got %q", raw)
	}
}

func unknownValueFromExistingKind(rawValues []string, existing Value) (Value, error) {
	if existing.Kind == ValueArray {
		items := make([]Value, 0, len(rawValues))
		elementKind := ValueString
		if len(existing.Array) > 0 {
			elementKind = existing.Array[0].Kind
		}
		for _, raw := range rawValues {
			item, err := unknownScalarFromKind(raw, elementKind)
			if err != nil {
				return Value{}, err
			}
			items = append(items, item)
		}
		return Value{Kind: ValueArray, Array: items}, nil
	}
	if len(rawValues) != 1 {
		return Value{}, fmt.Errorf("existing scalar field accepts exactly one --value")
	}
	return unknownScalarFromKind(rawValues[0], existing.Kind)
}

func unknownScalarFromKind(raw string, kind ValueKind) (Value, error) {
	switch kind {
	case ValueString:
		return Value{Kind: ValueString, String: raw}, nil
	case ValueNumber:
		return parseUnknownNumber(raw)
	case ValueBool:
		return parseRawBool(raw)
	default:
		return inferUnknownScalar(raw)
	}
}

func inferUnknownScalar(raw string) (Value, error) {
	if value, err := parseRawBool(raw); err == nil {
		return value, nil
	}
	if value, err := parseUnknownNumber(raw); err == nil {
		return value, nil
	}
	return Value{Kind: ValueString, String: raw}, nil
}

func parseUnknownNumber(raw string) (Value, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Value{}, fmt.Errorf("expected number")
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
		return Value{}, fmt.Errorf("expected number, got %q", raw)
	}
	return Value{Kind: ValueNumber, Text: trimmed}, nil
}

func renderUpdatedAssignment(document Document, field string, inputKey string, rawValues []string, spec fieldSpec, known bool, ranges []span, option UpdateFieldOptions) (string, error) {
	if known {
		value, err := ValueFromStrings(spec, rawValues)
		if err != nil {
			return "", err
		}
		return renderAssignment(spec, value, ""), nil
	}

	var existing *Assignment
	for index := range document.Assignments {
		if document.Assignments[index].Field == field {
			existing = &document.Assignments[index]
			break
		}
	}
	var existingValue *Value
	key := inputKey
	if existing != nil {
		existingValue = &existing.Value
		key = existing.Key
	}
	value, err := UnknownValueFromStrings(rawValues, existingValue)
	if err != nil {
		return "", err
	}
	comment := ""
	if len(ranges) == 0 {
		if strings.Contains(field, ".") {
			return "", fmt.Errorf("unknown nested field %q cannot be inserted", field)
		}
		comment = " " + unknownFieldAnnotation
	}
	return renderAssignment(fieldSpec{Name: field, RenderKey: key}, value, "") + comment, nil
}

func insertSpec(field string, spec fieldSpec, known bool) fieldSpec {
	if known {
		return spec
	}
	return fieldSpec{Name: field, RenderKey: field}
}

func validateAssignment(spec fieldSpec, assignment Assignment) error {
	if spec.RenderKey == "motd[]" && assignment.Key != "motd[]" {
		return fmt.Errorf("must be written as motd[]")
	}
	return validateValue(spec, assignment.Value)
}

func validateValue(spec fieldSpec, value Value) error {
	switch spec.Type {
	case fieldString:
		return requireKind(value, ValueString, "string")
	case fieldStringEnum:
		if err := requireKind(value, ValueString, "string"); err != nil {
			return err
		}
		if !containsString(spec.StringEnum, value.String) {
			return fmt.Errorf("must be one of %s", strings.Join(spec.StringEnum, ", "))
		}
	case fieldStringArray:
		if err := requireKind(value, ValueArray, "string array"); err != nil {
			return err
		}
		for _, item := range value.Array {
			if err := requireKind(item, ValueString, "string array item"); err != nil {
				return err
			}
		}
	case fieldBool:
		return requireKind(value, ValueBool, "boolean")
	case fieldIntBool:
		number, err := intValue(value)
		if err != nil {
			return err
		}
		if number != 0 && number != 1 {
			return fmt.Errorf("must be 0 or 1")
		}
	case fieldFixedInt:
		number, err := intValue(value)
		if err != nil {
			return err
		}
		if number != spec.FixedInt {
			return fmt.Errorf("must be %d", spec.FixedInt)
		}
	case fieldInt:
		_, err := intValue(value)
		return err
	case fieldFloatRange:
		number, err := floatValue(value)
		if err != nil {
			return err
		}
		if number < spec.Min || number > spec.Max {
			return fmt.Errorf("must be between %g and %g", spec.Min, spec.Max)
		}
	case fieldFloatMin:
		number, err := floatValue(value)
		if err != nil {
			return err
		}
		if number < spec.Min {
			return fmt.Errorf("must be greater than or equal to %g", spec.Min)
		}
	case fieldIntEnum:
		number, err := intValue(value)
		if err != nil {
			return err
		}
		if !containsInt(spec.IntEnum, number) {
			return fmt.Errorf("must be one of %s", joinInts(spec.IntEnum))
		}
	}
	return nil
}

func requireKind(value Value, kind ValueKind, label string) error {
	if value.Kind != kind {
		return fmt.Errorf("expected %s", label)
	}
	return nil
}

func intValue(value Value) (int, error) {
	if value.Kind != ValueNumber {
		return 0, fmt.Errorf("expected integer")
	}
	if strings.Contains(value.Text, ".") {
		return 0, fmt.Errorf("expected integer, got %q", value.Text)
	}
	number, err := strconv.Atoi(value.Text)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", value.Text)
	}
	return number, nil
}

func floatValue(value Value) (float64, error) {
	if value.Kind != ValueNumber {
		return 0, fmt.Errorf("expected number")
	}
	number, err := strconv.ParseFloat(value.Text, 64)
	if err != nil {
		return 0, fmt.Errorf("expected number, got %q", value.Text)
	}
	return number, nil
}

func fieldSpecMap() map[string]fieldSpec {
	specs := make(map[string]fieldSpec, len(documentedFields))
	for _, spec := range documentedFields {
		if spec.RenderKey == "" {
			spec.RenderKey = spec.Name
		}
		specs[spec.Name] = spec
	}
	return specs
}

func fieldOrder(field string) int {
	for index, spec := range documentedFields {
		if spec.Name == field {
			return index
		}
	}
	return len(documentedFields)
}

func matchingAssignmentRanges(document Document, field string) []span {
	var spans []span
	for _, assignment := range document.Assignments {
		if assignment.Field == field {
			spans = append(spans, span{start: assignment.Start, end: assignment.End})
		}
	}
	return spans
}

func insertMissingField(document Document, spec fieldSpec, rendered string) []byte {
	if spec.Name == "Missions.DayZ.template" {
		return insertMissionTemplate(document, rendered)
	}
	offset := topLevelInsertOffset(document, spec.Name)
	text := rendered + document.LineEnding
	return insertAt(document.Data, offset, text, document.LineEnding)
}

func insertMissionTemplate(document Document, rendered string) []byte {
	offset := eofInsertOffset(document.Data)
	text := strings.Join([]string{
		"class Missions",
		"{",
		"    class DayZ",
		"    {",
		"        " + rendered,
		"    };",
		"};",
	}, document.LineEnding) + document.LineEnding
	return insertAt(document.Data, offset, text, document.LineEnding)
}

func topLevelInsertOffset(document Document, field string) int {
	targetOrder := fieldOrder(field)
	for _, assignment := range document.Assignments {
		if strings.Contains(assignment.Field, ".") {
			continue
		}
		if fieldOrder(assignment.Field) > targetOrder {
			return lineStart(document.Data, assignment.Start)
		}
	}
	for _, class := range document.Classes {
		if class.Path == "Missions" {
			return lineStart(document.Data, class.Start)
		}
	}
	return eofInsertOffset(document.Data)
}

func renderAssignment(spec fieldSpec, value Value, indent string) string {
	key := spec.RenderKey
	if key == "" {
		key = spec.Name
	}
	return indent + key + " = " + renderValue(value) + ";"
}

func renderValue(value Value) string {
	switch value.Kind {
	case ValueString:
		return quoteString(value.String)
	case ValueNumber:
		return value.Text
	case ValueBool:
		if value.Bool {
			return "true"
		}
		return "false"
	case ValueArray:
		parts := make([]string, 0, len(value.Array))
		for _, item := range value.Array {
			parts = append(parts, renderValue(item))
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	default:
		return ""
	}
}

func quoteString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(value) + `"`
}

func replaceRanges(data []byte, ranges []span, rendered string) []byte {
	sort.Slice(ranges, func(left, right int) bool {
		return ranges[left].start > ranges[right].start
	})
	updated := append([]byte(nil), data...)
	for _, target := range ranges {
		updated = replaceRange(updated, target.start, target.end, []byte(rendered))
	}
	return updated
}

func removeRanges(data []byte, ranges []span) []byte {
	sort.Slice(ranges, func(left, right int) bool {
		return ranges[left].start > ranges[right].start
	})
	updated := append([]byte(nil), data...)
	for _, target := range ranges {
		start := lineStart(updated, target.start)
		end := lineEndWithBreak(updated, target.end)
		updated = replaceRange(updated, start, end, nil)
	}
	return updated
}

func readMutableFile(path string) ([]byte, fs.FileMode, error) {
	info, err := statFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := readFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

type span struct {
	start int
	end   int
}

func insertAt(data []byte, offset int, text string, lineEnding string) []byte {
	prefix := string(data[:offset])
	suffix := string(data[offset:])
	if prefix != "" && !strings.HasSuffix(prefix, "\n") && !strings.HasSuffix(prefix, "\r") {
		text = lineEnding + text
	}
	if suffix != "" && !strings.HasPrefix(suffix, "\n") && !strings.HasPrefix(suffix, "\r") && !strings.HasSuffix(text, lineEnding) {
		text += lineEnding
	}
	return replaceRange(data, offset, offset, []byte(text))
}

func replaceRange(data []byte, start int, end int, replacement []byte) []byte {
	updated := make([]byte, 0, len(data)-(end-start)+len(replacement))
	updated = append(updated, data[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, data[end:]...)
	return updated
}

func detectLineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func lineStart(data []byte, offset int) int {
	if offset > len(data) {
		offset = len(data)
	}
	index := bytes.LastIndexByte(data[:offset], '\n')
	if index < 0 {
		return 0
	}
	return index + 1
}

func lineEndWithBreak(data []byte, offset int) int {
	for offset < len(data) && data[offset] != '\n' {
		offset++
	}
	if offset < len(data) {
		return offset + 1
	}
	return offset
}

func eofInsertOffset(data []byte) int {
	offset := len(data)
	for offset > 0 {
		switch data[offset-1] {
		case ' ', '\t', '\r', '\n':
			offset--
		default:
			return offset
		}
	}
	return offset
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}

func intRange(min int, max int) []int {
	values := make([]int, 0, max-min+1)
	for value := min; value <= max; value++ {
		values = append(values, value)
	}
	return values
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenString
	tokenNumber
	tokenEqual
	tokenSemicolon
	tokenLBrace
	tokenRBrace
	tokenLBracket
	tokenRBracket
	tokenComma
)

type token struct {
	kind  tokenKind
	text  string
	start int
	end   int
}

type scanner struct {
	data   []byte
	offset int
}

func scanAll(data []byte) ([]token, error) {
	s := scanner{data: data}
	var tokens []token
	for {
		token, err := s.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		if token.kind == tokenEOF {
			return tokens, nil
		}
	}
}

func (s *scanner) next() (token, error) {
	if err := s.skipIgnored(); err != nil {
		return token{}, err
	}
	if s.offset >= len(s.data) {
		return token{kind: tokenEOF, start: s.offset, end: s.offset}, nil
	}
	start := s.offset
	ch := s.data[s.offset]
	switch ch {
	case '=':
		s.offset++
		return token{kind: tokenEqual, text: "=", start: start, end: s.offset}, nil
	case ';':
		s.offset++
		return token{kind: tokenSemicolon, text: ";", start: start, end: s.offset}, nil
	case '{':
		s.offset++
		return token{kind: tokenLBrace, text: "{", start: start, end: s.offset}, nil
	case '}':
		s.offset++
		return token{kind: tokenRBrace, text: "}", start: start, end: s.offset}, nil
	case '[':
		s.offset++
		return token{kind: tokenLBracket, text: "[", start: start, end: s.offset}, nil
	case ']':
		s.offset++
		return token{kind: tokenRBracket, text: "]", start: start, end: s.offset}, nil
	case ',':
		s.offset++
		return token{kind: tokenComma, text: ",", start: start, end: s.offset}, nil
	case '"':
		return s.scanString()
	default:
		if isIdentStart(ch) {
			return s.scanIdent(), nil
		}
		if isNumberStart(ch, s.peekByte(1)) {
			return s.scanNumber(), nil
		}
		return token{}, fmt.Errorf("unexpected character %q at byte %d", ch, start)
	}
}

func (s *scanner) skipIgnored() error {
	for s.offset < len(s.data) {
		switch s.data[s.offset] {
		case ' ', '\t', '\r', '\n':
			s.offset++
		case '/':
			if s.peekByte(1) == '/' {
				s.offset += 2
				for s.offset < len(s.data) && s.data[s.offset] != '\n' {
					s.offset++
				}
				continue
			}
			if s.peekByte(1) == '*' {
				end := bytes.Index(s.data[s.offset+2:], []byte("*/"))
				if end < 0 {
					return fmt.Errorf("unterminated block comment at byte %d", s.offset)
				}
				s.offset += end + 4
				continue
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func (s scanner) peekByte(delta int) byte {
	index := s.offset + delta
	if index >= len(s.data) {
		return 0
	}
	return s.data[index]
}

func (s *scanner) scanString() (token, error) {
	start := s.offset
	s.offset++
	var builder strings.Builder
	for s.offset < len(s.data) {
		ch := s.data[s.offset]
		switch ch {
		case '"':
			s.offset++
			return token{kind: tokenString, text: builder.String(), start: start, end: s.offset}, nil
		case '\r', '\n':
			return token{}, fmt.Errorf("unterminated string at byte %d", start)
		case '\\':
			s.offset++
			if s.offset >= len(s.data) {
				return token{}, fmt.Errorf("unterminated string escape at byte %d", start)
			}
			builder.WriteByte(s.data[s.offset])
			s.offset++
		default:
			builder.WriteByte(ch)
			s.offset++
		}
	}
	return token{}, fmt.Errorf("unterminated string at byte %d", start)
}

func (s *scanner) scanIdent() token {
	start := s.offset
	s.offset++
	for s.offset < len(s.data) && isIdentPart(s.data[s.offset]) {
		s.offset++
	}
	return token{kind: tokenIdent, text: string(s.data[start:s.offset]), start: start, end: s.offset}
}

func (s *scanner) scanNumber() token {
	start := s.offset
	if s.data[s.offset] == '-' {
		s.offset++
	}
	for s.offset < len(s.data) && isDigit(s.data[s.offset]) {
		s.offset++
	}
	if s.offset < len(s.data) && s.data[s.offset] == '.' {
		s.offset++
		for s.offset < len(s.data) && isDigit(s.data[s.offset]) {
			s.offset++
		}
	}
	return token{kind: tokenNumber, text: string(s.data[start:s.offset]), start: start, end: s.offset}
}

func isIdentStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

func isNumberStart(ch byte, next byte) bool {
	return isDigit(ch) || (ch == '-' && isDigit(next))
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

type cfgParser struct {
	tokens   []token
	position int
	document Document
}

func (p *cfgParser) parseItems(path []string, stopAtBrace bool) error {
	for p.current().kind != tokenEOF {
		if p.current().kind == tokenRBrace {
			if stopAtBrace {
				return nil
			}
			return fmt.Errorf("unexpected } at byte %d", p.current().start)
		}
		if p.current().kind == tokenIdent && p.current().text == "class" {
			if err := p.parseClass(path); err != nil {
				return err
			}
			continue
		}
		if err := p.parseAssignment(path); err != nil {
			return err
		}
	}
	if stopAtBrace {
		return fmt.Errorf("expected } before end of file")
	}
	return nil
}

func (p *cfgParser) parseClass(path []string) error {
	start := p.current().start
	p.advance()
	name, err := p.expect(tokenIdent)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokenLBrace); err != nil {
		return err
	}
	classPath := appendPath(path, name.text)
	if err := p.parseItems(classPath, true); err != nil {
		return err
	}
	closeStart := p.current().start
	p.advance()
	end, err := p.expect(tokenSemicolon)
	if err != nil {
		return err
	}
	p.document.Classes = append(p.document.Classes, ClassBlock{
		Path:       strings.Join(classPath, "."),
		Start:      start,
		CloseStart: closeStart,
		End:        end.end,
	})
	return nil
}

func (p *cfgParser) parseAssignment(path []string) error {
	key, err := p.expect(tokenIdent)
	if err != nil {
		return err
	}
	renderKey := key.text
	if p.current().kind == tokenLBracket {
		p.advance()
		if _, err := p.expect(tokenRBracket); err != nil {
			return err
		}
		renderKey += "[]"
	}
	if _, err := p.expect(tokenEqual); err != nil {
		return err
	}
	value, err := p.parseValue()
	if err != nil {
		return err
	}
	end, err := p.expect(tokenSemicolon)
	if err != nil {
		return err
	}
	fieldPath := appendPath(path, NormalizeField(renderKey))
	p.document.Assignments = append(p.document.Assignments, Assignment{
		Field: strings.Join(fieldPath, "."),
		Key:   renderKey,
		Value: value,
		Start: key.start,
		End:   end.end,
	})
	return nil
}

func (p *cfgParser) parseValue() (Value, error) {
	current := p.current()
	switch current.kind {
	case tokenString:
		p.advance()
		return Value{Kind: ValueString, String: current.text}, nil
	case tokenNumber:
		p.advance()
		return Value{Kind: ValueNumber, Text: current.text}, nil
	case tokenIdent:
		if current.text == "true" || current.text == "false" {
			p.advance()
			return Value{Kind: ValueBool, Bool: current.text == "true"}, nil
		}
	case tokenLBrace:
		return p.parseArray()
	}
	return Value{}, fmt.Errorf("expected value at byte %d", current.start)
}

func (p *cfgParser) parseArray() (Value, error) {
	p.advance()
	var values []Value
	for p.current().kind != tokenRBrace {
		value, err := p.parseValue()
		if err != nil {
			return Value{}, err
		}
		values = append(values, value)
		if p.current().kind != tokenComma {
			break
		}
		p.advance()
		if p.current().kind == tokenRBrace {
			return Value{}, fmt.Errorf("expected value at byte %d", p.current().start)
		}
	}
	if _, err := p.expect(tokenRBrace); err != nil {
		return Value{}, err
	}
	return Value{Kind: ValueArray, Array: values}, nil
}

func (p cfgParser) current() token {
	return p.tokens[p.position]
}

func (p *cfgParser) advance() {
	if p.position < len(p.tokens)-1 {
		p.position++
	}
}

func (p *cfgParser) expect(kind tokenKind) (token, error) {
	current := p.current()
	if current.kind != kind {
		return token{}, fmt.Errorf("expected %s at byte %d", tokenName(kind), current.start)
	}
	p.advance()
	return current, nil
}

func tokenName(kind tokenKind) string {
	switch kind {
	case tokenEOF:
		return "end of file"
	case tokenIdent:
		return "identifier"
	case tokenString:
		return "string"
	case tokenNumber:
		return "number"
	case tokenEqual:
		return "="
	case tokenSemicolon:
		return ";"
	case tokenLBrace:
		return "{"
	case tokenRBrace:
		return "}"
	case tokenLBracket:
		return "["
	case tokenRBracket:
		return "]"
	case tokenComma:
		return ","
	default:
		return "token"
	}
}

func appendPath(path []string, value string) []string {
	next := append([]string(nil), path...)
	next = append(next, value)
	return next
}

func sortedFieldValues(values []FieldValue) []FieldValue {
	sort.Slice(values, func(left, right int) bool {
		if fieldOrder(values[left].Field) == fieldOrder(values[right].Field) {
			return values[left].Field < values[right].Field
		}
		return fieldOrder(values[left].Field) < fieldOrder(values[right].Field)
	})
	return values
}

func SortFieldValues(values []FieldValue) []FieldValue {
	return sortedFieldValues(values)
}

func ConfigFileName(path string) string {
	return filepath.Base(filepath.Clean(path))
}
