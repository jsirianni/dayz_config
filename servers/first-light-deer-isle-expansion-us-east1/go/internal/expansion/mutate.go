package expansion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type FileMutation struct {
	Data    []byte
	Mode    fs.FileMode
	Changed bool
}

type PatrolSelector struct {
	Index      int
	Name       string
	Occurrence int
}

type PatrolPatch struct {
	Strings         map[string]string
	Floats          map[string]float64
	Ints            map[string]int
	BoolInts        map[string]int
	InheritBools    map[string]int
	SetUnits        *[]string
	AddUnits        []string
	RemoveUnits     []string
	ClearUnits      bool
	SetWaypoints    *[]Vector
	AddWaypoints    []Vector
	UpdateWaypoints map[int]Vector
	RemoveWaypoints []int
	ClearWaypoints  bool
}

type PatrolUpdateOptions struct {
	Selector PatrolSelector
	Patch    PatrolPatch
}

type StringListPatch struct {
	Set    *[]string
	Add    []string
	Remove []string
	Clear  bool
}

type PrefabObjectPatch struct {
	Strings           map[string]string
	Floats            map[string]float64
	QuantityMin       *float64
	QuantityMax       *float64
	SetHealth         map[string]MinMax
	RemoveHealth      []string
	ClearHealth       bool
	ConstructionParts StringListPatch
}

type LoadoutItemAddOptions struct {
	ParentPath string
	Container  string
	Slot       string
	Patch      PrefabObjectPatch
}

type LoadoutDeletePlan struct {
	Path       string
	Exists     bool
	References int
}

var statMutable = os.Stat
var readMutable = os.ReadFile
var writeMutable = os.WriteFile
var removeMutable = os.Remove
var makeDirAllMutable = os.MkdirAll
var renderJSONForMutation = renderJSON

func CreatePatrolFile(path string, patch PatrolPatch) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	settings, context, err := parsePatrolSettingsForMutation(data, path)
	if err != nil {
		return FileMutation{}, err
	}
	before := cloneJSON(settings)
	patrol := DefaultPatrol()
	if err := ApplyPatrolPatch(&patrol, patch); err != nil {
		return FileMutation{}, err
	}
	settings.Patrols = append(settings.Patrols, patrol)
	return finishPatrolMutation(data, mode, before, settings, context)
}

func UpdatePatrolFile(path string, options PatrolUpdateOptions) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	settings, context, err := parsePatrolSettingsForMutation(data, path)
	if err != nil {
		return FileMutation{}, err
	}
	before := cloneJSON(settings)
	index, err := selectPatrolIndex(settings.Patrols, options.Selector)
	if err != nil {
		return FileMutation{}, err
	}
	if err := ApplyPatrolPatch(&settings.Patrols[index], options.Patch); err != nil {
		return FileMutation{}, err
	}
	return finishPatrolMutation(data, mode, before, settings, context)
}

func DeletePatrolFile(path string, selector PatrolSelector) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	settings, context, err := parsePatrolSettingsForMutation(data, path)
	if err != nil {
		return FileMutation{}, err
	}
	before := cloneJSON(settings)
	index, err := selectPatrolIndex(settings.Patrols, selector)
	if err != nil {
		return FileMutation{}, err
	}
	settings.Patrols = append(settings.Patrols[:index], settings.Patrols[index+1:]...)
	return finishPatrolMutation(data, mode, before, settings, context)
}

func DefaultPatrol() AIPatrol {
	return AIPatrol{
		Name:                            "",
		Persist:                         boolInt(false),
		Faction:                         "West",
		Formation:                       "",
		FormationScale:                  -1,
		FormationLooseness:              0,
		Loadout:                         "HumanLoadout",
		Units:                           []string{},
		NumberOfAI:                      1,
		NumberOfAIMax:                   1,
		Behaviour:                       "ALTERNATE",
		LootingBehaviour:                "DEFAULT",
		Speed:                           "JOG",
		UnderThreatSpeed:                "SPRINT",
		DefaultStance:                   "",
		DefaultLookAngle:                0,
		CanBeLooted:                     boolInt(true),
		LootDropOnDeath:                 "",
		UnlimitedReload:                 0,
		SniperProneDistanceThreshold:    0,
		AccuracyMin:                     -1,
		AccuracyMax:                     -1,
		ThreatDistanceLimit:             -1,
		NoiseInvestigationDistanceLimit: -1,
		MaxFlankingDistance:             -1,
		EnableFlankingOutsideCombat:     inheritBool(-1),
		DamageMultiplier:                -1,
		DamageReceivedMultiplier:        -1,
		HeadshotResistance:              0,
		ShoryukenChance:                 0,
		ShoryukenDamageMultiplier:       0,
		CanSpawnInContaminatedArea:      boolInt(false),
		CanBeTriggeredByAI:              boolInt(false),
		MinDistRadius:                   -1,
		MaxDistRadius:                   -1,
		DespawnRadius:                   -1,
		MinSpreadRadius:                 5,
		MaxSpreadRadius:                 20,
		Chance:                          1,
		DespawnTime:                     -1,
		RespawnTime:                     -2,
		LoadBalancingCategory:           "",
		ObjectClassName:                 "",
		WaypointInterpolation:           "",
		UseRandomWaypointAsStartPoint:   boolInt(true),
		Waypoints:                       []Vector{},
	}
}

func ApplyPatrolPatch(patrol *AIPatrol, patch PatrolPatch) error {
	for field, value := range patch.Strings {
		if err := setField(patrol, field, value, reflect.String); err != nil {
			return err
		}
	}
	for field, value := range patch.Floats {
		if err := setField(patrol, field, value, reflect.Float64); err != nil {
			return err
		}
	}
	for field, value := range patch.Ints {
		if err := setField(patrol, field, value, reflect.Int); err != nil {
			return err
		}
	}
	for field, value := range patch.BoolInts {
		converted, err := boolIntFromInt(value)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if err := setNamedField(patrol, field, converted); err != nil {
			return err
		}
	}
	for field, value := range patch.InheritBools {
		converted, err := inheritBoolFromInt(value)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if err := setNamedField(patrol, field, converted); err != nil {
			return err
		}
	}
	applyStringListPatch(&patrol.Units, StringListPatch{
		Set:    patch.SetUnits,
		Add:    patch.AddUnits,
		Remove: patch.RemoveUnits,
		Clear:  patch.ClearUnits,
	})
	return applyWaypointPatch(&patrol.Waypoints, patch)
}

func CreateLoadoutFile(path string, patch PrefabObjectPatch) (FileMutation, error) {
	if _, err := statMutable(path); err == nil {
		return FileMutation{}, fmt.Errorf("%s already exists", path)
	} else if !os.IsNotExist(err) {
		return FileMutation{}, fmt.Errorf("stat %s: %w", path, err)
	}
	loadout := DefaultPrefabObject()
	if err := ApplyPrefabObjectPatch(&loadout, patch); err != nil {
		return FileMutation{}, err
	}
	context := validationContextNear(path)
	if err := ValidateLoadout(loadout, loadoutName(path), context); err != nil {
		return FileMutation{}, err
	}
	data, err := renderJSONForMutation(nil, loadout)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: data, Mode: 0o644, Changed: true}, nil
}

func UpdateLoadoutFile(path string, patch PrefabObjectPatch) (FileMutation, error) {
	return updateLoadoutObject(path, func(root *PrefabObject) error {
		return ApplyPrefabObjectPatch(root, patch)
	})
}

func AddLoadoutItemFile(path string, options LoadoutItemAddOptions) (FileMutation, error) {
	return updateLoadoutObject(path, func(root *PrefabObject) error {
		parent, err := ResolvePrefabObjectPath(root, options.ParentPath)
		if err != nil {
			return err
		}
		item := DefaultPrefabObject()
		if err := ApplyPrefabObjectPatch(&item, options.Patch); err != nil {
			return err
		}
		return addPrefabChild(parent, options.Container, options.Slot, item)
	})
}

func UpdateLoadoutItemFile(path string, itemPath string, patch PrefabObjectPatch) (FileMutation, error) {
	return updateLoadoutObject(path, func(root *PrefabObject) error {
		item, err := ResolvePrefabObjectPath(root, itemPath)
		if err != nil {
			return err
		}
		return ApplyPrefabObjectPatch(item, patch)
	})
}

func RemoveLoadoutItemFile(path string, itemPath string) (FileMutation, error) {
	return updateLoadoutObject(path, func(root *PrefabObject) error {
		return RemovePrefabObjectPath(root, itemPath)
	})
}

func DefaultPrefabObject() PrefabObject {
	return PrefabObject{
		ClassName:              "",
		Include:                "",
		Chance:                 1,
		Quantity:               MinMax{Min: 0, Max: 0},
		Health:                 []Health{},
		InventoryAttachments:   []PrefabSlot{},
		InventoryCargo:         []PrefabObject{},
		ConstructionPartsBuilt: []string{},
		Sets:                   []PrefabObject{},
	}
}

func ApplyPrefabObjectPatch(object *PrefabObject, patch PrefabObjectPatch) error {
	for field, value := range patch.Strings {
		if err := setField(object, field, value, reflect.String); err != nil {
			return err
		}
	}
	for field, value := range patch.Floats {
		if err := setField(object, field, value, reflect.Float64); err != nil {
			return err
		}
	}
	if patch.QuantityMin != nil {
		object.Quantity.Min = *patch.QuantityMin
	}
	if patch.QuantityMax != nil {
		object.Quantity.Max = *patch.QuantityMax
	}
	if patch.ClearHealth {
		object.Health = []Health{}
	}
	for _, zone := range patch.RemoveHealth {
		object.Health = removeHealth(object.Health, zone)
	}
	if len(patch.SetHealth) > 0 {
		zones := make([]string, 0, len(patch.SetHealth))
		for zone := range patch.SetHealth {
			zones = append(zones, zone)
		}
		sort.Strings(zones)
		for _, zone := range zones {
			object.Health = setHealth(object.Health, zone, patch.SetHealth[zone])
		}
	}
	applyStringListPatch(&object.ConstructionPartsBuilt, patch.ConstructionParts)
	NormalizePrefabObject(object)
	return nil
}

func ResolvePrefabObjectPath(root *PrefabObject, path string) (*PrefabObject, error) {
	if path == "root" {
		return root, nil
	}
	if !strings.HasPrefix(path, "root/") {
		return nil, fmt.Errorf("loadout item path must start with root")
	}
	current := root
	for _, segment := range strings.Split(strings.TrimPrefix(path, "root/"), "/") {
		next, err := resolvePrefabSegment(current, segment)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func RemovePrefabObjectPath(root *PrefabObject, path string) error {
	if path == "root" {
		return fmt.Errorf("cannot remove root loadout object")
	}
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return fmt.Errorf("loadout item path must start with root")
	}
	parentPath := path[:index]
	segment := path[index+1:]
	parent, err := ResolvePrefabObjectPath(root, parentPath)
	if err != nil {
		return err
	}
	return removePrefabSegment(parent, segment)
}

func PlanLoadoutDelete(path string, patrolsFile string) (LoadoutDeletePlan, error) {
	if _, err := statMutable(path); err != nil {
		return LoadoutDeletePlan{}, fmt.Errorf("stat %s: %w", path, err)
	}
	references, err := countLoadoutReferences(path, patrolsFile)
	if err != nil {
		return LoadoutDeletePlan{}, err
	}
	return LoadoutDeletePlan{Path: path, Exists: true, References: references}, nil
}

func DeleteLoadoutFile(path string) error {
	if err := removeMutable(path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	return nil
}

func WriteFileMutation(path string, mutation FileMutation) error {
	if !mutation.Changed {
		return nil
	}
	if err := makeDirAllMutable(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	return writeMutable(path, mutation.Data, mutation.Mode)
}

func parsePatrolSettingsForMutation(data []byte, path string) (AIPatrolSettings, validateContext, error) {
	var settings AIPatrolSettings
	if err := decodeStrictJSON(data, &settings); err != nil {
		return AIPatrolSettings{}, validateContext{}, fmt.Errorf("parse %s: %w", path, err)
	}
	context := validationContextNear(path)
	if err := ValidateAIPatrolSettings(settings, context); err != nil {
		return AIPatrolSettings{}, validateContext{}, err
	}
	return settings, context, nil
}

func finishPatrolMutation(original []byte, mode fs.FileMode, before AIPatrolSettings, after AIPatrolSettings, context validateContext) (FileMutation, error) {
	NormalizeAIPatrolSettings(&after)
	if err := ValidateAIPatrolSettings(after, context); err != nil {
		return FileMutation{}, err
	}
	if reflect.DeepEqual(before, after) {
		return FileMutation{Data: original, Mode: mode, Changed: false}, nil
	}
	data, err := renderJSONForMutation(original, after)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: data, Mode: mode, Changed: !bytes.Equal(original, data)}, nil
}

func updateLoadoutObject(path string, update func(*PrefabObject) error) (FileMutation, error) {
	data, mode, err := readMutableFile(path)
	if err != nil {
		return FileMutation{}, err
	}
	var loadout PrefabObject
	if err := decodeStrictJSON(data, &loadout); err != nil {
		return FileMutation{}, fmt.Errorf("parse %s: %w", path, err)
	}
	context := validationContextNear(path)
	if err := ValidateLoadout(loadout, loadoutName(path), context); err != nil {
		return FileMutation{}, err
	}
	before := cloneJSON(loadout)
	if err := update(&loadout); err != nil {
		return FileMutation{}, err
	}
	NormalizePrefabObject(&loadout)
	if err := ValidateLoadout(loadout, loadoutName(path), context); err != nil {
		return FileMutation{}, err
	}
	if reflect.DeepEqual(before, loadout) {
		return FileMutation{Data: data, Mode: mode, Changed: false}, nil
	}
	rendered, err := renderJSONForMutation(data, loadout)
	if err != nil {
		return FileMutation{}, err
	}
	return FileMutation{Data: rendered, Mode: mode, Changed: !bytes.Equal(data, rendered)}, nil
}

func readMutableFile(path string) ([]byte, fs.FileMode, error) {
	info, err := statMutable(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := readMutable(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

func renderJSON(original []byte, value any) ([]byte, error) {
	indent := detectJSONIndent(original)
	rendered, err := json.MarshalIndent(value, "", indent)
	if err != nil {
		return nil, err
	}
	lineEnding := detectJSONLineEnding(original)
	if lineEnding != "\n" {
		rendered = bytes.ReplaceAll(rendered, []byte("\n"), []byte(lineEnding))
	}
	return append(rendered, []byte(lineEnding)...), nil
}

func cloneJSON[T any](value T) T {
	data, _ := json.Marshal(value)
	var clone T
	_ = json.Unmarshal(data, &clone)
	return clone
}

func detectJSONIndent(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, `"`) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "    "
}

func detectJSONLineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func setField(target any, field string, value any, kind reflect.Kind) error {
	targetValue := reflect.ValueOf(target).Elem().FieldByName(field)
	if !targetValue.IsValid() || targetValue.Kind() != kind {
		return fmt.Errorf("unsupported field %q", field)
	}
	targetValue.Set(reflect.ValueOf(value).Convert(targetValue.Type()))
	return nil
}

func setNamedField(target any, field string, value any) error {
	targetValue := reflect.ValueOf(target).Elem().FieldByName(field)
	if !targetValue.IsValid() || !reflect.TypeOf(value).AssignableTo(targetValue.Type()) {
		return fmt.Errorf("unsupported field %q", field)
	}
	targetValue.Set(reflect.ValueOf(value))
	return nil
}

func boolIntFromInt(value int) (boolInt, error) {
	switch value {
	case 0:
		return boolInt(false), nil
	case 1:
		return boolInt(true), nil
	default:
		return boolInt(false), fmt.Errorf("expected 0 or 1, got %d", value)
	}
}

func inheritBoolFromInt(value int) (inheritBool, error) {
	if value != -1 && value != 0 && value != 1 {
		return inheritBool(0), fmt.Errorf("expected -1, 0, or 1, got %d", value)
	}
	return inheritBool(value), nil
}

func selectPatrolIndex(patrols []AIPatrol, selector PatrolSelector) (int, error) {
	if selector.Index > 0 && selector.Name != "" {
		return 0, fmt.Errorf("use --index or --name, not both")
	}
	if selector.Index > 0 {
		if selector.Index > len(patrols) {
			return 0, fmt.Errorf("patrol index %d not found", selector.Index)
		}
		return selector.Index - 1, nil
	}
	if selector.Name == "" {
		return 0, fmt.Errorf("--index or --name is required")
	}
	var matches []int
	for index, patrol := range patrols {
		if patrol.Name == selector.Name {
			matches = append(matches, index)
		}
	}
	if len(matches) == 0 {
		return 0, fmt.Errorf("patrol named %q not found", selector.Name)
	}
	if len(matches) > 1 && selector.Occurrence < 1 {
		return 0, fmt.Errorf("patrol name %q appears %d times; use --occurrence to select one", selector.Name, len(matches))
	}
	if selector.Occurrence > 0 {
		if selector.Occurrence > len(matches) {
			return 0, fmt.Errorf("patrol name %q occurrence %d not found", selector.Name, selector.Occurrence)
		}
		return matches[selector.Occurrence-1], nil
	}
	return matches[0], nil
}

func applyWaypointPatch(waypoints *[]Vector, patch PatrolPatch) error {
	if patch.SetWaypoints != nil {
		*waypoints = append([]Vector(nil), (*patch.SetWaypoints)...)
	}
	if patch.ClearWaypoints {
		*waypoints = []Vector{}
	}
	for index, waypoint := range patch.UpdateWaypoints {
		if index < 1 || index > len(*waypoints) {
			return fmt.Errorf("waypoint index %d not found", index)
		}
		(*waypoints)[index-1] = waypoint
	}
	if len(patch.RemoveWaypoints) > 0 {
		for _, index := range sortedDescendingUniqueInts(patch.RemoveWaypoints) {
			if index < 1 || index > len(*waypoints) {
				return fmt.Errorf("waypoint index %d not found", index)
			}
			*waypoints = append((*waypoints)[:index-1], (*waypoints)[index:]...)
		}
	}
	*waypoints = append(*waypoints, patch.AddWaypoints...)
	if *waypoints == nil {
		*waypoints = []Vector{}
	}
	return nil
}

func applyStringListPatch(values *[]string, patch StringListPatch) {
	if patch.Set != nil {
		*values = append([]string(nil), (*patch.Set)...)
	}
	if patch.Clear {
		*values = []string{}
	}
	*values = removeStringValues(*values, patch.Remove)
	for _, addition := range patch.Add {
		if !containsStringValue(*values, addition) {
			*values = append(*values, addition)
		}
	}
	if *values == nil {
		*values = []string{}
	}
}

func resolvePrefabSegment(parent *PrefabObject, segment string) (*PrefabObject, error) {
	parts := strings.Split(segment, ":")
	if len(parts) == 2 && parts[0] == "cargo" {
		index, err := parsePositiveIndex(parts[1])
		if err != nil {
			return nil, err
		}
		if index > len(parent.InventoryCargo) {
			return nil, fmt.Errorf("cargo item %d not found", index)
		}
		return &parent.InventoryCargo[index-1], nil
	}
	if len(parts) == 2 && parts[0] == "sets" {
		index, err := parsePositiveIndex(parts[1])
		if err != nil {
			return nil, err
		}
		if index > len(parent.Sets) {
			return nil, fmt.Errorf("set item %d not found", index)
		}
		return &parent.Sets[index-1], nil
	}
	if len(parts) == 3 && parts[0] == "attachments" {
		index, err := parsePositiveIndex(parts[2])
		if err != nil {
			return nil, err
		}
		slotIndex := findSlot(parent.InventoryAttachments, parts[1])
		if slotIndex < 0 || index > len(parent.InventoryAttachments[slotIndex].Items) {
			return nil, fmt.Errorf("attachment %q item %d not found", parts[1], index)
		}
		return &parent.InventoryAttachments[slotIndex].Items[index-1], nil
	}
	return nil, fmt.Errorf("invalid loadout item path segment %q", segment)
}

func removePrefabSegment(parent *PrefabObject, segment string) error {
	parts := strings.Split(segment, ":")
	if len(parts) == 2 && parts[0] == "cargo" {
		index, err := parsePositiveIndex(parts[1])
		if err != nil {
			return err
		}
		if index > len(parent.InventoryCargo) {
			return fmt.Errorf("cargo item %d not found", index)
		}
		parent.InventoryCargo = append(parent.InventoryCargo[:index-1], parent.InventoryCargo[index:]...)
		return nil
	}
	if len(parts) == 2 && parts[0] == "sets" {
		index, err := parsePositiveIndex(parts[1])
		if err != nil {
			return err
		}
		if index > len(parent.Sets) {
			return fmt.Errorf("set item %d not found", index)
		}
		parent.Sets = append(parent.Sets[:index-1], parent.Sets[index:]...)
		return nil
	}
	if len(parts) == 3 && parts[0] == "attachments" {
		index, err := parsePositiveIndex(parts[2])
		if err != nil {
			return err
		}
		slotIndex := findSlot(parent.InventoryAttachments, parts[1])
		if slotIndex < 0 || index > len(parent.InventoryAttachments[slotIndex].Items) {
			return fmt.Errorf("attachment %q item %d not found", parts[1], index)
		}
		slot := &parent.InventoryAttachments[slotIndex]
		slot.Items = append(slot.Items[:index-1], slot.Items[index:]...)
		if len(slot.Items) == 0 {
			parent.InventoryAttachments = append(parent.InventoryAttachments[:slotIndex], parent.InventoryAttachments[slotIndex+1:]...)
		}
		return nil
	}
	return fmt.Errorf("invalid loadout item path segment %q", segment)
}

func addPrefabChild(parent *PrefabObject, container string, slot string, item PrefabObject) error {
	switch container {
	case "attachment":
		index := findSlot(parent.InventoryAttachments, slot)
		if index < 0 {
			parent.InventoryAttachments = append(parent.InventoryAttachments, PrefabSlot{SlotName: slot, Items: []PrefabObject{}})
			index = len(parent.InventoryAttachments) - 1
		}
		parent.InventoryAttachments[index].Items = append(parent.InventoryAttachments[index].Items, item)
	case "cargo":
		if slot != "" {
			return fmt.Errorf("--slot is only supported for attachment items")
		}
		parent.InventoryCargo = append(parent.InventoryCargo, item)
	case "set":
		if slot != "" {
			return fmt.Errorf("--slot is only supported for attachment items")
		}
		parent.Sets = append(parent.Sets, item)
	default:
		return fmt.Errorf("container must be attachment, cargo, or set")
	}
	return nil
}

func parsePositiveIndex(text string) (int, error) {
	index, err := strconv.Atoi(text)
	if err != nil || index < 1 {
		return 0, fmt.Errorf("index must be greater than 0")
	}
	return index, nil
}

func removeHealth(values []Health, zone string) []Health {
	filtered := values[:0]
	for _, value := range values {
		if value.Zone != zone {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func setHealth(values []Health, zone string, minMax MinMax) []Health {
	for index := range values {
		if values[index].Zone == zone {
			values[index].Min = minMax.Min
			values[index].Max = minMax.Max
			return values
		}
	}
	return append(values, Health{Min: minMax.Min, Max: minMax.Max, Zone: zone})
}

func findSlot(slots []PrefabSlot, slotName string) int {
	for index, slot := range slots {
		if slot.SlotName == slotName {
			return index
		}
	}
	return -1
}

func NormalizeAIPatrolSettings(settings *AIPatrolSettings) {
	if settings.LoadBalancingCategories == nil {
		settings.LoadBalancingCategories = map[string][]LoadBalancingRule{}
	}
	if settings.Patrols == nil {
		settings.Patrols = []AIPatrol{}
	}
	for index := range settings.Patrols {
		if settings.Patrols[index].Units == nil {
			settings.Patrols[index].Units = []string{}
		}
		if settings.Patrols[index].Waypoints == nil {
			settings.Patrols[index].Waypoints = []Vector{}
		}
	}
}

func NormalizePrefabObject(object *PrefabObject) {
	if object.Health == nil {
		object.Health = []Health{}
	}
	if object.InventoryAttachments == nil {
		object.InventoryAttachments = []PrefabSlot{}
	}
	if object.InventoryCargo == nil {
		object.InventoryCargo = []PrefabObject{}
	}
	if object.ConstructionPartsBuilt == nil {
		object.ConstructionPartsBuilt = []string{}
	}
	if object.Sets == nil {
		object.Sets = []PrefabObject{}
	}
	for slotIndex := range object.InventoryAttachments {
		if object.InventoryAttachments[slotIndex].Items == nil {
			object.InventoryAttachments[slotIndex].Items = []PrefabObject{}
		}
		for itemIndex := range object.InventoryAttachments[slotIndex].Items {
			NormalizePrefabObject(&object.InventoryAttachments[slotIndex].Items[itemIndex])
		}
	}
	for index := range object.InventoryCargo {
		NormalizePrefabObject(&object.InventoryCargo[index])
	}
	for index := range object.Sets {
		NormalizePrefabObject(&object.Sets[index])
	}
}

func validationContextNear(path string) validateContext {
	dir := filepath.Dir(filepath.Clean(path))
	for {
		files, err := DiscoverAIConfigFiles(dir)
		if err == nil {
			return validationContextFromFiles(files)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return validateContext{}
		}
		dir = parent
	}
}

func validationContextFromFiles(files []AIFile) validateContext {
	context := validateContext{loadoutNames: map[string]string{}, lootDrops: map[string]string{}}
	for _, file := range files {
		switch file.Kind {
		case KindAILoadout:
			context.loadoutNames[loadoutName(file.Path)] = file.Path
		case KindAILootDrop:
			context.lootDrops[filepath.Base(file.Path)] = file.Path
		}
	}
	return context
}

func countLoadoutReferences(path string, patrolsFile string) (int, error) {
	if patrolsFile == "" {
		patrolsFile = discoverPatrolsFileNear(path)
	}
	if patrolsFile == "" {
		return 0, nil
	}
	settings, err := ParseAIPatrolSettingsFile(patrolsFile)
	if err != nil {
		return 0, err
	}
	name := loadoutName(path)
	count := 0
	for _, patrol := range settings.Patrols {
		if patrol.Loadout == name {
			count++
		}
	}
	return count, nil
}

func discoverPatrolsFileNear(path string) string {
	dir := filepath.Dir(filepath.Clean(path))
	for {
		files, err := DiscoverAIConfigFiles(dir)
		if err == nil {
			for _, file := range files {
				if file.Kind == KindAIPatrols {
					return file.Path
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func sortedDescendingUniqueInts(values []int) []int {
	seen := map[int]bool{}
	var unique []int
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(unique)))
	return unique
}

func removeStringValues(values []string, removals []string) []string {
	filtered := values[:0]
	for _, value := range values {
		if !containsStringValue(removals, value) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func containsStringValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
