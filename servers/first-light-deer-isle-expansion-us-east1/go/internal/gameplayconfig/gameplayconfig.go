package gameplayconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
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
	Clear bool
}

type ValidationErrors []string

func (errs ValidationErrors) Error() string {
	return strings.Join(errs, "; ")
}

type valueKind int

const (
	valueObject valueKind = iota
	valueArray
	valueString
	valueNumber
	valueBool
	valueNull
)

type Value struct {
	Kind   valueKind
	Object map[string]*Value
	Order  []string
	Array  []*Value
	String string
	Number string
	Bool   bool
}

type fieldType int

const (
	fieldBool fieldType = iota
	fieldInt
	fieldFloat
	fieldString
	fieldStringArray
	fieldFloatArray
)

type fieldSpec struct {
	Path      string
	Type      fieldType
	Canonical string
	IntEnum   []int
	Min       *float64
	Max       *float64
	Len       int
	Pattern   *regexp.Regexp
}

var argbPattern = regexp.MustCompile(`(?i)^0x[0-9a-f]{8}$`)

var documentedFields = []fieldSpec{
	{Path: "version", Type: fieldInt},

	{Path: "GeneralData.disableBaseDamage", Type: fieldBool},
	{Path: "GeneralData.disableContainerDamage", Type: fieldBool},
	{Path: "GeneralData.disableRespawnDialog", Type: fieldBool},
	{Path: "GeneralData.disableRespawnInUnconsciousness", Type: fieldBool},

	{Path: "PlayerData.disablePersonalLight", Type: fieldBool},
	{Path: "PlayerData.spawnGearPresetFiles", Type: fieldStringArray},
	{Path: "PlayerData.StaminaData.sprintStaminaModifierErc", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.sprintStaminaModifierCro", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.staminaWeightLimitThreshold", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.staminaMax", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.staminaKgToStaminaPercentPenalty", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.staminaMinCap", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.sprintSwimmingStaminaModifier", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.sprintLadderStaminaModifier", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.meleeStaminaModifier", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.obstacleTraversalStaminaModifier", Type: fieldFloat},
	{Path: "PlayerData.StaminaData.holdBreathStaminaModifier", Type: fieldFloat},
	{Path: "PlayerData.ShockHandlingData.shockRefillSpeedConscious", Type: fieldFloat},
	{Path: "PlayerData.ShockHandlingData.shockRefillSpeedUnconscious", Type: fieldFloat},
	{Path: "PlayerData.ShockHandlingData.allowRefillSpeedModifier", Type: fieldBool},
	{Path: "PlayerData.MovementData.timeToStrafeJog", Type: fieldFloat, Min: floatPtr(0.01)},
	{Path: "PlayerData.MovementData.rotationSpeedJog", Type: fieldFloat, Min: floatPtr(0.01)},
	{Path: "PlayerData.MovementData.timeToSprint", Type: fieldFloat, Min: floatPtr(0.01)},
	{Path: "PlayerData.MovementData.timeToStrafeSprint", Type: fieldFloat, Min: floatPtr(0.01)},
	{Path: "PlayerData.MovementData.rotationSpeedSprint", Type: fieldFloat, Min: floatPtr(0.01)},
	{Path: "PlayerData.MovementData.allowStaminaAffectInertia", Type: fieldBool},
	{Path: "PlayerData.DrowningData.staminaDepletionSpeed", Type: fieldFloat},
	{Path: "PlayerData.DrowningData.healthDepletionSpeed", Type: fieldFloat},
	{Path: "PlayerData.DrowningData.shockDepletionSpeed", Type: fieldFloat},
	{Path: "PlayerData.WeaponObstructionData.staticMode", Type: fieldInt, IntEnum: []int{0, 1, 2}},
	{Path: "PlayerData.WeaponObstructionData.dynamicMode", Type: fieldInt, IntEnum: []int{0, 1, 2}},

	{Path: "WorldsData.lightingConfig", Type: fieldInt, IntEnum: []int{0, 1, 2}},
	{Path: "WorldsData.objectSpawnersArr", Type: fieldStringArray},
	{Path: "WorldsData.environmentMinTemps", Type: fieldFloatArray, Len: 12},
	{Path: "WorldsData.environmentMaxTemps", Type: fieldFloatArray, Len: 12},
	{Path: "WorldsData.wetnessWeightModifiers", Type: fieldFloatArray, Len: 5},
	{Path: "WorldsData.playerRestrictedAreaFiles", Type: fieldStringArray},

	{Path: "BaseBuildingData.HologramData.disableIsCollidingBBoxCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsCollidingPlayerCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsClippingRoofCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsBaseViableCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsCollidingGPlotCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsCollidingAngleCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsPlacementPermittedCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableHeightPlacementCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsUnderwaterCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableIsInTerrainCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableColdAreaPlacementCheck", Type: fieldBool},
	{Path: "BaseBuildingData.HologramData.disableColdAreaBuildingCheck", Type: fieldBool, Canonical: "BaseBuildingData.HologramData.disableColdAreaPlacementCheck"},
	{Path: "BaseBuildingData.HologramData.disallowedTypesInUnderground", Type: fieldStringArray},
	{Path: "BaseBuildingData.ConstructionData.disablePerformRoofCheck", Type: fieldBool},
	{Path: "BaseBuildingData.ConstructionData.disableIsCollidingCheck", Type: fieldBool},
	{Path: "BaseBuildingData.ConstructionData.disableDistanceCheck", Type: fieldBool},

	{Path: "UIData.use3DMap", Type: fieldBool},
	{Path: "UIData.HitIndicationData.hitDirectionOverrideEnabled", Type: fieldBool},
	{Path: "UIData.HitIndicationData.hitDirectionBehaviour", Type: fieldInt, IntEnum: []int{0, 1, 2}},
	{Path: "UIData.HitIndicationData.hitDirectionStyle", Type: fieldInt, IntEnum: []int{0, 1, 2}},
	{Path: "UIData.HitIndicationData.hitDirectionIndicatorColorStr", Type: fieldString, Pattern: argbPattern},
	{Path: "UIData.HitIndicationData.hitDirectionMaxDuration", Type: fieldFloat},
	{Path: "UIData.HitIndicationData.hitDirectionBreakPointRelative", Type: fieldFloat, Min: floatPtr(0), Max: floatPtr(1)},
	{Path: "UIData.HitIndicationData.hitDirectionScatter", Type: fieldFloat},
	{Path: "UIData.HitIndicationData.hitIndicationPostProcessEnabled", Type: fieldBool},

	{Path: "MapData.ignoreMapOwnership", Type: fieldBool},
	{Path: "MapData.ignoreNavItemsOwnership", Type: fieldBool},
	{Path: "MapData.displayPlayerPosition", Type: fieldBool},
	{Path: "MapData.displayNavInfo", Type: fieldBool},

	{Path: "VehicleData.boatDecayMultiplier", Type: fieldFloat},
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

func ValidateDocument(document *Value) error {
	var errs ValidationErrors
	if document == nil || document.Kind != valueObject {
		errs = append(errs, "root must be an object")
		return errs
	}
	validateNode("", document, &errs)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func ParseData(data []byte) (*Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := parseValue(decoder, "")
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
	if value.Kind != valueObject {
		return nil, fmt.Errorf("root must be an object")
	}
	return value, nil
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
	return ListFields(document), nil
}

func ListFields(document *Value) []FieldValue {
	var values []FieldValue
	walkLeaves("", document, &values)
	SortFieldValues(values)
	return values
}

func GetFieldFile(path string, field string) (FieldValue, bool, error) {
	data, err := readFile(path)
	if err != nil {
		return FieldValue{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	document, err := ParseData(data)
	if err != nil {
		return FieldValue{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	foundPath, value := findRequestedPath(document, NormalizeField(field))
	if value == nil {
		return FieldValue{}, false, nil
	}
	return FieldValue{Field: foundPath, Value: DisplayValue(value)}, true, nil
}

func UpdateFieldFile(path string, field string, rawValues []string, options ...UpdateFieldOptions) (FileMutation, bool, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, false, err
	}
	updated, changed, existed, err := UpdateField(data, field, rawValues, updateOptions(options))
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

func UpdateField(data []byte, field string, rawValues []string, options UpdateFieldOptions) ([]byte, bool, bool, error) {
	document, err := ParseData(data)
	if err != nil {
		return nil, false, false, err
	}
	requested := NormalizeField(field)
	spec, ok := specForPath(requested)
	if !ok {
		return nil, false, false, fmt.Errorf("unsupported gameplay field %q", requested)
	}
	value, err := ValueFromStrings(spec, rawValues, options.Clear)
	if err != nil {
		return nil, false, false, err
	}
	if err := validateField(spec.canonicalPath(), value, spec); err != nil {
		return nil, false, false, err
	}
	existingPath, existing := findRequestedPath(document, requested)
	targetPath := spec.canonicalPath()
	if existing != nil {
		targetPath = existingPath
		if DisplayValue(existing) == DisplayValue(value) {
			return data, false, true, nil
		}
	}
	if err := setPath(document, targetPath, value); err != nil {
		return nil, false, false, err
	}
	updated := renderJSON(data, document)
	if _, err := parseUpdatedData(updated); err != nil {
		return nil, false, false, err
	}
	return updated, !bytes.Equal(data, updated), existing != nil, nil
}

func DeleteField(data []byte, field string) ([]byte, bool, error) {
	document, err := ParseData(data)
	if err != nil {
		return nil, false, err
	}
	foundPath, value := findRequestedPath(document, NormalizeField(field))
	if value == nil {
		return data, false, nil
	}
	_ = deletePath(document, foundPath)
	updated := renderJSON(data, document)
	if _, err := parseUpdatedData(updated); err != nil {
		return nil, false, err
	}
	return updated, !bytes.Equal(data, updated), nil
}

func ValueFromStrings(spec fieldSpec, rawValues []string, clear bool) (*Value, error) {
	if clear {
		if spec.Type != fieldStringArray && spec.Type != fieldFloatArray {
			return nil, fmt.Errorf("--clear is only valid for array fields")
		}
		return &Value{Kind: valueArray, Array: []*Value{}}, nil
	}
	if len(rawValues) == 0 {
		return nil, fmt.Errorf("--value is required")
	}
	switch spec.Type {
	case fieldStringArray:
		items := make([]*Value, 0, len(rawValues))
		for _, raw := range rawValues {
			items = append(items, &Value{Kind: valueString, String: raw})
		}
		return &Value{Kind: valueArray, Array: items}, nil
	case fieldFloatArray:
		items := make([]*Value, 0, len(rawValues))
		for _, raw := range rawValues {
			if _, err := strconv.ParseFloat(raw, 64); err != nil {
				return nil, fmt.Errorf("%s value %q is not a number", spec.Path, raw)
			}
			items = append(items, &Value{Kind: valueNumber, Number: raw})
		}
		return &Value{Kind: valueArray, Array: items}, nil
	}
	if len(rawValues) != 1 {
		return nil, fmt.Errorf("%s accepts exactly one --value", spec.Path)
	}
	return scalarValueFromString(spec, rawValues[0])
}

func DisplayValue(value *Value) string {
	var builder strings.Builder
	renderCompact(&builder, value)
	return builder.String()
}

func SortFieldValues(values []FieldValue) {
	sort.Slice(values, func(left, right int) bool {
		leftOrder, leftKnown := schemaOrder[canonicalFor(values[left].Field)]
		rightOrder, rightKnown := schemaOrder[canonicalFor(values[right].Field)]
		if leftKnown && rightKnown && leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return values[left].Field < values[right].Field
	})
}

func NormalizeField(field string) string {
	return strings.Trim(strings.TrimSpace(field), ".")
}

func parseValue(decoder *json.Decoder, path string) (*Value, error) {
	token, err := decoder.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("empty JSON document")
		}
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			return parseObject(decoder, path)
		case '[':
			return parseArray(decoder, path)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", value)
		}
	case string:
		return &Value{Kind: valueString, String: value}, nil
	case json.Number:
		return &Value{Kind: valueNumber, Number: value.String()}, nil
	case bool:
		return &Value{Kind: valueBool, Bool: value}, nil
	case nil:
		return &Value{Kind: valueNull}, nil
	default:
		return nil, fmt.Errorf("unsupported JSON token %T", token)
	}
}

func parseObject(decoder *json.Decoder, path string) (*Value, error) {
	result := &Value{Kind: valueObject, Object: map[string]*Value{}}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key := token.(string)
		childPath := joinPath(path, key)
		if _, exists := result.Object[key]; exists {
			return nil, fmt.Errorf("duplicate key %q at %s", key, displayPath(path))
		}
		child, err := parseValue(decoder, childPath)
		if err != nil {
			return nil, err
		}
		result.Object[key] = child
		result.Order = append(result.Order, key)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return result, nil
}

func parseArray(decoder *json.Decoder, path string) (*Value, error) {
	result := &Value{Kind: valueArray}
	for decoder.More() {
		child, err := parseValue(decoder, path+"[]")
		if err != nil {
			return nil, err
		}
		result.Array = append(result.Array, child)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return result, nil
}

func validateNode(path string, value *Value, errs *ValidationErrors) {
	switch value.Kind {
	case valueObject:
		if path != "" && !objectPaths[path] {
			if spec, ok := specForPath(path); ok {
				addValidationError(errs, validateField(path, value, spec))
				return
			}
			*errs = append(*errs, fmt.Sprintf("unknown field %q", path))
			return
		}
		keys := append([]string(nil), value.Order...)
		for _, key := range keys {
			validateNode(joinPath(path, key), value.Object[key], errs)
		}
	case valueArray, valueString, valueNumber, valueBool, valueNull:
		spec, ok := specForPath(path)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("unknown field %q", path))
			return
		}
		addValidationError(errs, validateField(path, value, spec))
	}
}

func validateField(path string, value *Value, spec fieldSpec) error {
	switch spec.Type {
	case fieldBool:
		if value.Kind != valueBool {
			return fmt.Errorf("%s must be a boolean", path)
		}
	case fieldInt:
		number, err := parseIntValue(value)
		if err != nil {
			return fmt.Errorf("%s %w", path, err)
		}
		if len(spec.IntEnum) > 0 && !containsInt(spec.IntEnum, number) {
			return fmt.Errorf("%s must be one of %s", path, formatInts(spec.IntEnum))
		}
	case fieldFloat:
		number, err := parseFloatValue(value)
		if err != nil {
			return fmt.Errorf("%s %w", path, err)
		}
		if spec.Min != nil && number < *spec.Min {
			return fmt.Errorf("%s must be greater than or equal to %s", path, formatFloat(*spec.Min))
		}
		if spec.Max != nil && number > *spec.Max {
			return fmt.Errorf("%s must be less than or equal to %s", path, formatFloat(*spec.Max))
		}
	case fieldString:
		if value.Kind != valueString {
			return fmt.Errorf("%s must be a string", path)
		}
		if spec.Pattern != nil && !spec.Pattern.MatchString(value.String) {
			return fmt.Errorf("%s must match 0xAARRGGBB", path)
		}
	case fieldStringArray:
		if err := validateArray(path, value, spec.Len); err != nil {
			return err
		}
		for index, item := range value.Array {
			if item.Kind != valueString {
				return fmt.Errorf("%s[%d] must be a string", path, index)
			}
		}
	case fieldFloatArray:
		if err := validateArray(path, value, spec.Len); err != nil {
			return err
		}
		for index, item := range value.Array {
			if _, err := parseFloatValue(item); err != nil {
				return fmt.Errorf("%s[%d] %w", path, index, err)
			}
		}
	}
	return nil
}

func validateArray(path string, value *Value, length int) error {
	if value.Kind != valueArray {
		return fmt.Errorf("%s must be an array", path)
	}
	if length > 0 && len(value.Array) != length {
		return fmt.Errorf("%s must contain exactly %d values", path, length)
	}
	return nil
}

func scalarValueFromString(spec fieldSpec, raw string) (*Value, error) {
	switch spec.Type {
	case fieldBool:
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true":
			return &Value{Kind: valueBool, Bool: true}, nil
		case "false":
			return &Value{Kind: valueBool}, nil
		default:
			return nil, fmt.Errorf("%s must be true or false", spec.Path)
		}
	case fieldInt:
		if _, err := strconv.Atoi(raw); err != nil {
			return nil, fmt.Errorf("%s value %q is not an integer", spec.Path, raw)
		}
		return &Value{Kind: valueNumber, Number: raw}, nil
	case fieldFloat:
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return nil, fmt.Errorf("%s value %q is not a number", spec.Path, raw)
		}
		return &Value{Kind: valueNumber, Number: raw}, nil
	case fieldString:
		return &Value{Kind: valueString, String: raw}, nil
	default:
		return nil, fmt.Errorf("%s is an array field", spec.Path)
	}
}

func parseIntValue(value *Value) (int, error) {
	if value.Kind != valueNumber {
		return 0, fmt.Errorf("must be an integer")
	}
	if strings.ContainsAny(value.Number, ".eE") {
		return 0, fmt.Errorf("must be an integer")
	}
	number, err := strconv.Atoi(value.Number)
	if err != nil {
		return 0, fmt.Errorf("must be an integer")
	}
	return number, nil
}

func parseFloatValue(value *Value) (float64, error) {
	if value.Kind != valueNumber {
		return 0, fmt.Errorf("must be a number")
	}
	number, err := strconv.ParseFloat(value.Number, 64)
	if err != nil {
		return 0, fmt.Errorf("must be a number")
	}
	return number, nil
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

func findRequestedPath(root *Value, requested string) (string, *Value) {
	paths := []string{requested}
	if spec, ok := specForPath(requested); ok {
		paths = append(paths, spec.canonicalPath())
		paths = append(paths, aliasPaths(spec.canonicalPath())...)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		if value := findPath(root, path); value != nil {
			return path, value
		}
	}
	return "", nil
}

func findPath(root *Value, path string) *Value {
	current := root
	if path == "" {
		return current
	}
	for _, part := range strings.Split(path, ".") {
		if current == nil || current.Kind != valueObject {
			return nil
		}
		current = current.Object[part]
	}
	return current
}

func setPath(root *Value, path string, value *Value) error {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		child := current.Object[part]
		if child == nil {
			child = &Value{Kind: valueObject, Object: map[string]*Value{}}
			current.Object[part] = child
			current.Order = append(current.Order, part)
		}
		if child.Kind != valueObject {
			return fmt.Errorf("%s is not an object", part)
		}
		current = child
	}
	leaf := parts[len(parts)-1]
	if current.Object[leaf] == nil {
		current.Order = append(current.Order, leaf)
	}
	current.Object[leaf] = value
	return nil
}

func deletePath(root *Value, path string) error {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = current.Object[part]
		if current == nil || current.Kind != valueObject {
			return fmt.Errorf("%s not found", path)
		}
	}
	leaf := parts[len(parts)-1]
	delete(current.Object, leaf)
	current.Order = removeString(current.Order, leaf)
	return nil
}

func walkLeaves(path string, value *Value, values *[]FieldValue) {
	if value == nil {
		return
	}
	if value.Kind != valueObject {
		*values = append(*values, FieldValue{Field: path, Value: DisplayValue(value)})
		return
	}
	for _, key := range value.Order {
		walkLeaves(joinPath(path, key), value.Object[key], values)
	}
}

func renderJSON(original []byte, value *Value) []byte {
	var builder strings.Builder
	renderValue(&builder, "", value, 0, detectJSONIndent(original), detectJSONLineEnding(original))
	lineEnding := detectJSONLineEnding(original)
	builder.WriteString(lineEnding)
	return []byte(builder.String())
}

func renderValue(builder *strings.Builder, path string, value *Value, depth int, indent string, lineEnding string) {
	switch value.Kind {
	case valueObject:
		renderObject(builder, path, value, depth, indent, lineEnding)
	case valueArray:
		renderArray(builder, value)
	case valueString:
		data, _ := json.Marshal(value.String)
		builder.Write(data)
	case valueNumber:
		builder.WriteString(value.Number)
	case valueBool:
		builder.WriteString(strconv.FormatBool(value.Bool))
	case valueNull:
		builder.WriteString("null")
	}
}

func renderObject(builder *strings.Builder, path string, value *Value, depth int, indent string, lineEnding string) {
	builder.WriteByte('{')
	keys := sortedObjectKeys(path, value)
	if len(keys) == 0 {
		builder.WriteByte('}')
		return
	}
	for index, key := range keys {
		builder.WriteString(lineEnding)
		builder.WriteString(strings.Repeat(indent, depth+1))
		data, _ := json.Marshal(key)
		builder.Write(data)
		builder.WriteString(": ")
		renderValue(builder, joinPath(path, key), value.Object[key], depth+1, indent, lineEnding)
		if index < len(keys)-1 {
			builder.WriteByte(',')
		}
	}
	builder.WriteString(lineEnding)
	builder.WriteString(strings.Repeat(indent, depth))
	builder.WriteByte('}')
}

func renderArray(builder *strings.Builder, value *Value) {
	builder.WriteByte('[')
	for index, item := range value.Array {
		if index > 0 {
			builder.WriteString(", ")
		}
		renderCompact(builder, item)
	}
	builder.WriteByte(']')
}

func renderCompact(builder *strings.Builder, value *Value) {
	switch value.Kind {
	case valueObject:
		builder.WriteByte('{')
		keys := sortedObjectKeys("", value)
		for index, key := range keys {
			if index > 0 {
				builder.WriteByte(',')
			}
			data, _ := json.Marshal(key)
			builder.Write(data)
			builder.WriteByte(':')
			renderCompact(builder, value.Object[key])
		}
		builder.WriteByte('}')
	case valueArray:
		builder.WriteByte('[')
		for index, item := range value.Array {
			if index > 0 {
				builder.WriteByte(',')
			}
			renderCompact(builder, item)
		}
		builder.WriteByte(']')
	case valueString:
		data, _ := json.Marshal(value.String)
		builder.Write(data)
	case valueNumber:
		builder.WriteString(value.Number)
	case valueBool:
		builder.WriteString(strconv.FormatBool(value.Bool))
	case valueNull:
		builder.WriteString("null")
	}
}

func sortedObjectKeys(path string, value *Value) []string {
	keys := append([]string(nil), value.Order...)
	sort.SliceStable(keys, func(left, right int) bool {
		leftPath := joinPath(path, keys[left])
		rightPath := joinPath(path, keys[right])
		leftOrder, leftKnown := schemaOrder[canonicalFor(leftPath)]
		rightOrder, rightKnown := schemaOrder[canonicalFor(rightPath)]
		if leftKnown && rightKnown && leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return keys[left] < keys[right]
	})
	return keys
}

func detectJSONIndent(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, `"`) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "\t"
}

func detectJSONLineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func updateOptions(options []UpdateFieldOptions) UpdateFieldOptions {
	if len(options) == 0 {
		return UpdateFieldOptions{}
	}
	return options[0]
}

func addValidationError(errs *ValidationErrors, err error) {
	if err != nil {
		*errs = append(*errs, err.Error())
	}
}

func displayPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}

func joinPath(parent string, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func formatInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func removeString(values []string, target string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func floatPtr(value float64) *float64 {
	return &value
}

func canonicalFor(path string) string {
	if spec, ok := specForPath(path); ok {
		return spec.canonicalPath()
	}
	return path
}

func (spec fieldSpec) canonicalPath() string {
	if spec.Canonical != "" {
		return spec.Canonical
	}
	return spec.Path
}

var specMap = buildSpecMap()
var objectPaths = buildObjectPaths()
var schemaOrder = buildSchemaOrder()

func specForPath(path string) (fieldSpec, bool) {
	spec, ok := specMap[path]
	return spec, ok
}

func aliasPaths(canonical string) []string {
	var paths []string
	for _, spec := range documentedFields {
		if spec.Canonical == canonical {
			paths = append(paths, spec.Path)
		}
	}
	return paths
}

func buildSpecMap() map[string]fieldSpec {
	result := map[string]fieldSpec{}
	for _, spec := range documentedFields {
		result[spec.Path] = spec
	}
	return result
}

func buildObjectPaths() map[string]bool {
	result := map[string]bool{"": true}
	for _, spec := range documentedFields {
		parts := strings.Split(spec.Path, ".")
		for index := 1; index < len(parts); index++ {
			result[strings.Join(parts[:index], ".")] = true
		}
	}
	return result
}

func buildSchemaOrder() map[string]int {
	result := map[string]int{}
	for index, spec := range documentedFields {
		parts := strings.Split(spec.canonicalPath(), ".")
		for partIndex := 1; partIndex <= len(parts); partIndex++ {
			path := strings.Join(parts[:partIndex], ".")
			if _, exists := result[path]; !exists {
				result[path] = index
			}
		}
	}
	return result
}
