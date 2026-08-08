package expansion

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	KindAISettings  = "expansion-ai-settings"
	KindAIPatrols   = "expansion-ai-patrols"
	KindAILocations = "expansion-ai-locations"
	KindAILoadout   = "expansion-ai-loadout"
	KindAILootDrop  = "expansion-ai-loot-drop"
)

type AIFile struct {
	Path string
	Kind string
}

type AIFileStatus struct {
	Path    string
	Kind    string
	Summary string
	Err     error
}

type AISettings struct {
	Version                             int             `json:"m_Version"`
	AccuracyMin                         float64         `json:"AccuracyMin"`
	AccuracyMax                         float64         `json:"AccuracyMax"`
	ThreatDistanceLimit                 float64         `json:"ThreatDistanceLimit"`
	NoiseInvestigationDistanceLimit     float64         `json:"NoiseInvestigationDistanceLimit"`
	MaxFlankingDistance                 float64         `json:"MaxFlankingDistance"`
	EnableFlankingOutsideCombat         boolInt         `json:"EnableFlankingOutsideCombat"`
	DamageMultiplier                    float64         `json:"DamageMultiplier"`
	DamageReceivedMultiplier            float64         `json:"DamageReceivedMultiplier"`
	ShoryukenChance                     float64         `json:"ShoryukenChance"`
	ShoryukenDamageMultiplier           float64         `json:"ShoryukenDamageMultiplier"`
	Admins                              []string        `json:"Admins"`
	Vaulting                            boolInt         `json:"Vaulting"`
	SniperProneDistanceThreshold        float64         `json:"SniperProneDistanceThreshold"`
	AggressionTimeout                   float64         `json:"AggressionTimeout"`
	GuardAggressionTimeout              float64         `json:"GuardAggressionTimeout"`
	Manners                             boolInt         `json:"Manners"`
	MemeLevel                           int             `json:"MemeLevel"`
	CanRecruitFriendly                  boolInt         `json:"CanRecruitFriendly"`
	CanRecruitGuards                    boolInt         `json:"CanRecruitGuards"`
	MaxRecruitableAI                    int             `json:"MaxRecruitableAI"`
	PreventClimb                        []string        `json:"PreventClimb"`
	FormationScale                      float64         `json:"FormationScale"`
	PlayerFactions                      []string        `json:"PlayerFactions"`
	LogAIHitBy                          boolInt         `json:"LogAIHitBy"`
	LogAIKilled                         boolInt         `json:"LogAIKilled"`
	EnableZombieVehicleAttackHandler    boolInt         `json:"EnableZombieVehicleAttackHandler"`
	EnableZombieVehicleAttackPhysics    boolInt         `json:"EnableZombieVehicleAttackPhysics"`
	OverrideClientWeaponFiring          boolInt         `json:"OverrideClientWeaponFiring"`
	RecreateWeaponNetworkRepresentation boolInt         `json:"RecreateWeaponNetworkRepresentation"`
	LightingConfigMinNightVisibility    map[int]float64 `json:"LightingConfigMinNightVisibilityMeters"`
}

type AIPatrolSettings struct {
	Version                         int                            `json:"m_Version"`
	Enabled                         boolInt                        `json:"Enabled"`
	FormationScale                  float64                        `json:"FormationScale"`
	DespawnTime                     float64                        `json:"DespawnTime"`
	RespawnTime                     float64                        `json:"RespawnTime"`
	MinDistRadius                   float64                        `json:"MinDistRadius"`
	MaxDistRadius                   float64                        `json:"MaxDistRadius"`
	DespawnRadius                   float64                        `json:"DespawnRadius"`
	AccuracyMin                     float64                        `json:"AccuracyMin"`
	AccuracyMax                     float64                        `json:"AccuracyMax"`
	ThreatDistanceLimit             float64                        `json:"ThreatDistanceLimit"`
	NoiseInvestigationDistanceLimit float64                        `json:"NoiseInvestigationDistanceLimit"`
	MaxFlankingDistance             float64                        `json:"MaxFlankingDistance"`
	EnableFlankingOutsideCombat     inheritBool                    `json:"EnableFlankingOutsideCombat"`
	DamageMultiplier                float64                        `json:"DamageMultiplier"`
	DamageReceivedMultiplier        float64                        `json:"DamageReceivedMultiplier"`
	ShoryukenChance                 float64                        `json:"ShoryukenChance"`
	ShoryukenDamageMultiplier       float64                        `json:"ShoryukenDamageMultiplier"`
	LoadBalancingCategories         map[string][]LoadBalancingRule `json:"LoadBalancingCategories"`
	Patrols                         []AIPatrol                     `json:"Patrols"`
}

type LoadBalancingRule struct {
	MinPlayers int `json:"MinPlayers"`
	MaxPlayers int `json:"MaxPlayers"`
	MaxPatrols int `json:"MaxPatrols"`
}

type AIPatrol struct {
	Name                            string      `json:"Name"`
	Persist                         boolInt     `json:"Persist"`
	Faction                         string      `json:"Faction"`
	Formation                       string      `json:"Formation"`
	FormationScale                  float64     `json:"FormationScale"`
	FormationLooseness              float64     `json:"FormationLooseness"`
	Loadout                         string      `json:"Loadout"`
	Units                           []string    `json:"Units"`
	NumberOfAI                      int         `json:"NumberOfAI"`
	NumberOfAIMax                   int         `json:"NumberOfAIMax"`
	Behaviour                       string      `json:"Behaviour"`
	LootingBehaviour                string      `json:"LootingBehaviour"`
	Speed                           string      `json:"Speed"`
	UnderThreatSpeed                string      `json:"UnderThreatSpeed"`
	DefaultStance                   string      `json:"DefaultStance"`
	DefaultLookAngle                float64     `json:"DefaultLookAngle"`
	CanBeLooted                     boolInt     `json:"CanBeLooted"`
	LootDropOnDeath                 string      `json:"LootDropOnDeath"`
	UnlimitedReload                 int         `json:"UnlimitedReload"`
	SniperProneDistanceThreshold    float64     `json:"SniperProneDistanceThreshold"`
	AccuracyMin                     float64     `json:"AccuracyMin"`
	AccuracyMax                     float64     `json:"AccuracyMax"`
	ThreatDistanceLimit             float64     `json:"ThreatDistanceLimit"`
	NoiseInvestigationDistanceLimit float64     `json:"NoiseInvestigationDistanceLimit"`
	MaxFlankingDistance             float64     `json:"MaxFlankingDistance"`
	EnableFlankingOutsideCombat     inheritBool `json:"EnableFlankingOutsideCombat"`
	DamageMultiplier                float64     `json:"DamageMultiplier"`
	DamageReceivedMultiplier        float64     `json:"DamageReceivedMultiplier"`
	HeadshotResistance              float64     `json:"HeadshotResistance"`
	ShoryukenChance                 float64     `json:"ShoryukenChance"`
	ShoryukenDamageMultiplier       float64     `json:"ShoryukenDamageMultiplier"`
	CanSpawnInContaminatedArea      boolInt     `json:"CanSpawnInContaminatedArea"`
	CanBeTriggeredByAI              boolInt     `json:"CanBeTriggeredByAI"`
	MinDistRadius                   float64     `json:"MinDistRadius"`
	MaxDistRadius                   float64     `json:"MaxDistRadius"`
	DespawnRadius                   float64     `json:"DespawnRadius"`
	MinSpreadRadius                 float64     `json:"MinSpreadRadius"`
	MaxSpreadRadius                 float64     `json:"MaxSpreadRadius"`
	Chance                          float64     `json:"Chance"`
	DespawnTime                     float64     `json:"DespawnTime"`
	RespawnTime                     float64     `json:"RespawnTime"`
	LoadBalancingCategory           string      `json:"LoadBalancingCategory"`
	ObjectClassName                 string      `json:"ObjectClassName"`
	WaypointInterpolation           string      `json:"WaypointInterpolation"`
	UseRandomWaypointAsStartPoint   boolInt     `json:"UseRandomWaypointAsStartPoint"`
	Waypoints                       []Vector    `json:"Waypoints"`
}

type AILocationSettings struct {
	Version                  int               `json:"m_Version"`
	RoamingLocations         []RoamingLocation `json:"RoamingLocations"`
	ExcludedRoamingBuildings []string          `json:"ExcludedRoamingBuildings"`
	NoGoAreas                []NoGoArea        `json:"NoGoAreas"`
}

type RoamingLocation struct {
	Name     string  `json:"Name"`
	Position Vector  `json:"Position"`
	Radius   float64 `json:"Radius"`
	Type     string  `json:"Type"`
	Enabled  boolInt `json:"Enabled"`
}

type NoGoArea struct {
	Name     string  `json:"Name"`
	Position Vector  `json:"Position"`
	Radius   float64 `json:"Radius"`
	Height   float64 `json:"Height"`
}

type PrefabObject struct {
	ClassName              string         `json:"ClassName"`
	Include                string         `json:"Include"`
	Chance                 float64        `json:"Chance"`
	Quantity               MinMax         `json:"Quantity"`
	Health                 []Health       `json:"Health"`
	InventoryAttachments   []PrefabSlot   `json:"InventoryAttachments"`
	InventoryCargo         []PrefabObject `json:"InventoryCargo"`
	ConstructionPartsBuilt []string       `json:"ConstructionPartsBuilt"`
	Sets                   []PrefabObject `json:"Sets"`
}

type PrefabSlot struct {
	SlotName string         `json:"SlotName"`
	Items    []PrefabObject `json:"Items"`
}

type MinMax struct {
	Min float64 `json:"Min"`
	Max float64 `json:"Max"`
}

type Health struct {
	Min  float64 `json:"Min"`
	Max  float64 `json:"Max"`
	Zone string  `json:"Zone"`
}

type Vector [3]float64

func (vector *Vector) UnmarshalJSON(data []byte) error {
	var values []float64
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 3 {
		return fmt.Errorf("vector must contain exactly 3 numbers")
	}
	*vector = Vector{values[0], values[1], values[2]}
	return nil
}

type boolInt bool

func (value *boolInt) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	switch trimmed {
	case "true", "1":
		*value = true
		return nil
	case "false", "0":
		*value = false
		return nil
	default:
		return fmt.Errorf("expected boolean or 0/1, got %s", trimmed)
	}
}

func (value boolInt) MarshalJSON() ([]byte, error) {
	if value {
		return []byte("1"), nil
	}
	return []byte("0"), nil
}

type inheritBool int

func (value *inheritBool) UnmarshalJSON(data []byte) error {
	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	if number != -1 && number != 0 && number != 1 {
		return fmt.Errorf("expected -1, 0, or 1, got %d", number)
	}
	*value = inheritBool(number)
	return nil
}

func (value inheritBool) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", value)), nil
}

type validateContext struct {
	loadoutNames map[string]string
	lootDrops    map[string]string
}

type validationErrors []string

func (errs validationErrors) Error() string {
	return strings.Join(errs, "; ")
}

func (errs *validationErrors) add(format string, args ...any) {
	*errs = append(*errs, fmt.Sprintf(format, args...))
}

func (errs validationErrors) err() error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}

var stat = os.Stat
var readDir = os.ReadDir
var readFile = os.ReadFile

func InspectAIPath(path string) ([]AIFileStatus, error) {
	files, err := DiscoverAIConfigFiles(path)
	if err != nil {
		return nil, err
	}

	context := validateContext{
		loadoutNames: map[string]string{},
		lootDrops:    map[string]string{},
	}
	for _, file := range files {
		switch file.Kind {
		case KindAILoadout:
			context.loadoutNames[loadoutName(file.Path)] = file.Path
		case KindAILootDrop:
			context.lootDrops[filepath.Base(file.Path)] = file.Path
		}
	}

	statuses := make([]AIFileStatus, 0, len(files))
	for _, file := range files {
		status := AIFileStatus{Path: file.Path, Kind: file.Kind}
		status.Summary, status.Err = validateAIFile(file, context)
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func DiscoverAIConfigFiles(path string) ([]AIFile, error) {
	cleanPath := filepath.Clean(path)
	info, err := stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", cleanPath, err)
	}
	if !info.IsDir() {
		kind, err := classifyAIFile(cleanPath)
		if err != nil {
			return nil, err
		}
		return []AIFile{{Path: cleanPath, Kind: kind}}, nil
	}

	found := map[string]string{}
	addMissionSettings(found, cleanPath)
	entries, err := readDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", cleanPath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			addMissionSettings(found, filepath.Join(cleanPath, entry.Name()))
		}
	}
	addIfExists(found, filepath.Join(cleanPath, "profiles", "ExpansionMod", "Settings", "AISettings.json"), KindAISettings)
	addJSONDirectory(found, filepath.Join(cleanPath, "profiles", "ExpansionMod", "Loadouts"), KindAILoadout)
	addJSONDirectory(found, filepath.Join(cleanPath, "profiles", "ExpansionMod", "AI", "LootDrops"), KindAILootDrop)

	if len(found) == 0 {
		return nil, fmt.Errorf("no Expansion AI config files found under %s", cleanPath)
	}

	files := make([]AIFile, 0, len(found))
	for path, kind := range found {
		files = append(files, AIFile{Path: path, Kind: kind})
	}
	sort.Slice(files, func(left, right int) bool {
		if files[left].Kind == files[right].Kind {
			return files[left].Path < files[right].Path
		}
		return files[left].Kind < files[right].Kind
	})
	return files, nil
}

func DiscoverAIConfigFilesNear(start string) ([]AIFile, error) {
	dir := nearestExistingDir(start)
	for {
		files, err := DiscoverAIConfigFiles(dir)
		if err == nil {
			return files, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no Expansion AI config files found near %s", filepath.Clean(start))
		}
		dir = parent
	}
}

func FindAIPatrolsFileNear(start string) (string, error) {
	files, err := DiscoverAIConfigFilesNear(start)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, file := range files {
		if file.Kind == KindAIPatrols {
			matches = append(matches, file.Path)
		}
	}
	return requireSinglePath(matches, "AIPatrolSettings.json", "--file")
}

func FindAILoadoutsDirNear(start string) (string, error) {
	dir := nearestExistingDir(start)
	for {
		candidate := filepath.Join(dir, "profiles", "ExpansionMod", "Loadouts")
		info, err := stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no Expansion AI loadouts directory found near %s; provide --file", filepath.Clean(start))
		}
		dir = parent
	}
}

func FindAILoadoutFileNear(name string, start string) (string, error) {
	targetName := NormalizeLoadoutName(name)
	files, err := DiscoverAIConfigFilesNear(start)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, file := range files {
		if file.Kind == KindAILoadout && LoadoutName(file.Path) == targetName {
			matches = append(matches, file.Path)
		}
	}
	return requireSinglePath(matches, "loadout "+targetName, "--file")
}

func LoadoutName(path string) string {
	return loadoutName(path)
}

func NormalizeLoadoutName(name string) string {
	trimmed := strings.TrimSpace(name)
	if strings.EqualFold(filepath.Ext(trimmed), ".json") {
		return strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	}
	return trimmed
}

func nearestExistingDir(start string) string {
	clean := filepath.Clean(start)
	info, err := stat(clean)
	if err == nil {
		if info.IsDir() {
			return clean
		}
		return filepath.Dir(clean)
	}
	if filepath.Ext(clean) != "" {
		return filepath.Dir(clean)
	}
	return clean
}

func requireSinglePath(matches []string, resource string, flag string) (string, error) {
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%s not found; provide %s", resource, flag)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%s is ambiguous (%d matches); provide %s", resource, len(matches), flag)
	}
}

func addMissionSettings(found map[string]string, root string) {
	settingsPath := filepath.Join(root, "expansion", "settings")
	addIfExists(found, filepath.Join(settingsPath, "AILocationSettings.json"), KindAILocations)
	addIfExists(found, filepath.Join(settingsPath, "AIPatrolSettings.json"), KindAIPatrols)
}

func addIfExists(found map[string]string, path string, kind string) {
	info, err := stat(path)
	if err == nil && !info.IsDir() {
		found[filepath.Clean(path)] = kind
	}
}

func addJSONDirectory(found map[string]string, dir string, kind string) {
	entries, err := readDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			found[filepath.Clean(filepath.Join(dir, entry.Name()))] = kind
		}
	}
}

func classifyAIFile(path string) (string, error) {
	name := filepath.Base(path)
	switch name {
	case "AISettings.json":
		return KindAISettings, nil
	case "AIPatrolSettings.json":
		return KindAIPatrols, nil
	case "AILocationSettings.json":
		return KindAILocations, nil
	}
	if !strings.EqualFold(filepath.Ext(name), ".json") {
		return "", fmt.Errorf("%s is not a supported Expansion AI config file", path)
	}
	switch filepath.Base(filepath.Dir(path)) {
	case "Loadouts":
		return KindAILoadout, nil
	case "LootDrops":
		return KindAILootDrop, nil
	default:
		return "", fmt.Errorf("%s is not a supported Expansion AI config file", path)
	}
}

func validateAIFile(file AIFile, context validateContext) (string, error) {
	switch file.Kind {
	case KindAISettings:
		settings, err := ParseAISettingsFile(file.Path)
		if err != nil {
			return "", err
		}
		return "", ValidateAISettings(settings)
	case KindAIPatrols:
		patrols, err := ParseAIPatrolSettingsFile(file.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d patrols", len(patrols.Patrols)), ValidateAIPatrolSettings(patrols, context)
	case KindAILocations:
		locations, err := ParseAILocationSettingsFile(file.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d locations", len(locations.RoamingLocations)), ValidateAILocationSettings(locations)
	case KindAILoadout:
		loadout, err := ParseLoadoutFile(file.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d items", countPrefabItems(loadout)), ValidateLoadout(loadout, loadoutName(file.Path), context)
	case KindAILootDrop:
		lootDrop, err := ParseLootDropFile(file.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d entries", len(lootDrop)), ValidateLootDrop(lootDrop, context)
	default:
		return "", fmt.Errorf("unsupported Expansion AI file kind %q", file.Kind)
	}
}

func ParseAISettingsFile(path string) (AISettings, error) {
	var settings AISettings
	return settings, parseJSONFile(path, &settings)
}

func ParseAIPatrolSettingsFile(path string) (AIPatrolSettings, error) {
	var settings AIPatrolSettings
	return settings, parseJSONFile(path, &settings)
}

func ParseAILocationSettingsFile(path string) (AILocationSettings, error) {
	var settings AILocationSettings
	return settings, parseJSONFile(path, &settings)
}

func ParseLoadoutFile(path string) (PrefabObject, error) {
	var loadout PrefabObject
	return loadout, parseJSONFile(path, &loadout)
}

func ParseLootDropFile(path string) ([]PrefabObject, error) {
	var lootDrop []PrefabObject
	return lootDrop, parseJSONFile(path, &lootDrop)
}

func CountPrefabItems(object PrefabObject) int {
	return countPrefabItems(object)
}

func parseJSONFile(path string, target any) error {
	data, err := readFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := decodeStrictJSON(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("expected one JSON document")
	}
	return nil
}

func ValidateAISettings(settings AISettings) error {
	var errs validationErrors
	requirePercent(&errs, "AccuracyMin", settings.AccuracyMin)
	requirePercent(&errs, "AccuracyMax", settings.AccuracyMax)
	requireOrdered(&errs, "AccuracyMin", settings.AccuracyMin, "AccuracyMax", settings.AccuracyMax)
	requireNonNegative(&errs, "ThreatDistanceLimit", settings.ThreatDistanceLimit)
	requireNonNegative(&errs, "NoiseInvestigationDistanceLimit", settings.NoiseInvestigationDistanceLimit)
	requireNonNegative(&errs, "MaxFlankingDistance", settings.MaxFlankingDistance)
	requireNonNegative(&errs, "DamageMultiplier", settings.DamageMultiplier)
	requireNonNegative(&errs, "DamageReceivedMultiplier", settings.DamageReceivedMultiplier)
	requireNonNegative(&errs, "SniperProneDistanceThreshold", settings.SniperProneDistanceThreshold)
	requireIntRange(&errs, "MaxRecruitableAI", settings.MaxRecruitableAI, 0, 255)
	requireNonNegative(&errs, "FormationScale", settings.FormationScale)
	for key, value := range settings.LightingConfigMinNightVisibility {
		requireRange(&errs, fmt.Sprintf("LightingConfigMinNightVisibilityMeters[%d]", key), value, 0.000001, 1000)
	}
	return errs.err()
}

func ValidateAIPatrolSettings(settings AIPatrolSettings, context validateContext) error {
	var errs validationErrors
	requireInheritNonNegative(&errs, "FormationScale", settings.FormationScale, -1)
	requireInheritNonNegative(&errs, "DespawnTime", settings.DespawnTime, -1)
	requireInheritNonNegative(&errs, "RespawnTime", settings.RespawnTime, -1)
	requireInheritNonNegative(&errs, "MinDistRadius", settings.MinDistRadius, -1)
	requireInheritNonNegative(&errs, "MaxDistRadius", settings.MaxDistRadius, -1)
	requireNonNegative(&errs, "DespawnRadius", settings.DespawnRadius)
	requireInheritPercent(&errs, "AccuracyMin", settings.AccuracyMin)
	requireInheritPercent(&errs, "AccuracyMax", settings.AccuracyMax)
	requireOrderedUnlessInherited(&errs, "AccuracyMin", settings.AccuracyMin, "AccuracyMax", settings.AccuracyMax)
	requireInheritNonNegative(&errs, "ThreatDistanceLimit", settings.ThreatDistanceLimit, -1)
	requireInheritNonNegative(&errs, "NoiseInvestigationDistanceLimit", settings.NoiseInvestigationDistanceLimit, -1)
	requireInheritNonNegative(&errs, "MaxFlankingDistance", settings.MaxFlankingDistance, -1)
	requireInheritNonNegative(&errs, "DamageMultiplier", settings.DamageMultiplier, -1)
	requireInheritNonNegative(&errs, "DamageReceivedMultiplier", settings.DamageReceivedMultiplier, -1)
	validateLoadBalancingCategories(&errs, settings.LoadBalancingCategories)
	for index, patrol := range settings.Patrols {
		validatePatrol(&errs, patrol, index, settings.LoadBalancingCategories, context)
	}
	return errs.err()
}

func validateLoadBalancingCategories(errs *validationErrors, categories map[string][]LoadBalancingRule) {
	for category, rules := range categories {
		if len(rules) == 0 {
			errs.add("LoadBalancingCategories[%q] must contain at least one rule", category)
		}
		for index, rule := range rules {
			field := fmt.Sprintf("LoadBalancingCategories[%q][%d]", category, index)
			requireNonNegativeInt(errs, field+".MinPlayers", rule.MinPlayers)
			requireNonNegativeInt(errs, field+".MaxPlayers", rule.MaxPlayers)
			requireInheritNonNegativeInt(errs, field+".MaxPatrols", rule.MaxPatrols, -1)
			if rule.MaxPlayers < rule.MinPlayers {
				errs.add("%s.MaxPlayers must be greater than or equal to %s.MinPlayers", field, field)
			}
		}
	}
}

func validatePatrol(errs *validationErrors, patrol AIPatrol, index int, categories map[string][]LoadBalancingRule, context validateContext) {
	prefix := fmt.Sprintf("Patrols[%d]", index)
	requireEnum(errs, prefix+".Faction", patrol.Faction, documentedFactions(), false)
	requireEnum(errs, prefix+".Formation", patrol.Formation, documentedFormations(), true)
	requireInheritNonNegative(errs, prefix+".FormationScale", patrol.FormationScale, -1)
	requireNonNegative(errs, prefix+".FormationLooseness", patrol.FormationLooseness)
	validateLoadoutReference(errs, prefix+".Loadout", patrol.Loadout, context)
	requireEnum(errs, prefix+".Behaviour", patrol.Behaviour, waypointBehaviors(), false)
	requireBitmask(errs, prefix+".LootingBehaviour", patrol.LootingBehaviour, lootingBehaviors())
	requireEnum(errs, prefix+".Speed", patrol.Speed, movementSpeeds(), false)
	requireEnum(errs, prefix+".UnderThreatSpeed", patrol.UnderThreatSpeed, movementSpeeds(), false)
	requireEnum(errs, prefix+".DefaultStance", patrol.DefaultStance, stances(), true)
	requireRange(errs, prefix+".DefaultLookAngle", patrol.DefaultLookAngle, -180, 180)
	requireUnlimitedReload(errs, prefix+".UnlimitedReload", patrol.UnlimitedReload)
	requireNonNegative(errs, prefix+".SniperProneDistanceThreshold", patrol.SniperProneDistanceThreshold)
	requireInheritPercent(errs, prefix+".AccuracyMin", patrol.AccuracyMin)
	requireInheritPercent(errs, prefix+".AccuracyMax", patrol.AccuracyMax)
	requireOrderedUnlessInherited(errs, prefix+".AccuracyMin", patrol.AccuracyMin, prefix+".AccuracyMax", patrol.AccuracyMax)
	requireInheritNonNegative(errs, prefix+".ThreatDistanceLimit", patrol.ThreatDistanceLimit, -1)
	requireInheritNonNegative(errs, prefix+".NoiseInvestigationDistanceLimit", patrol.NoiseInvestigationDistanceLimit, -1)
	requireInheritNonNegative(errs, prefix+".MaxFlankingDistance", patrol.MaxFlankingDistance, -1)
	requireInheritNonNegative(errs, prefix+".DamageMultiplier", patrol.DamageMultiplier, -1)
	requireInheritNonNegative(errs, prefix+".DamageReceivedMultiplier", patrol.DamageReceivedMultiplier, -1)
	requireNonNegative(errs, prefix+".HeadshotResistance", patrol.HeadshotResistance)
	requireInheritNonNegative(errs, prefix+".MinDistRadius", patrol.MinDistRadius, -1)
	requireInheritNonNegative(errs, prefix+".MaxDistRadius", patrol.MaxDistRadius, -1)
	requireInheritNonNegative(errs, prefix+".DespawnRadius", patrol.DespawnRadius, -1)
	requireNonNegative(errs, prefix+".MinSpreadRadius", patrol.MinSpreadRadius)
	requireNonNegative(errs, prefix+".MaxSpreadRadius", patrol.MaxSpreadRadius)
	requireOrdered(errs, prefix+".MinSpreadRadius", patrol.MinSpreadRadius, prefix+".MaxSpreadRadius", patrol.MaxSpreadRadius)
	requirePercent(errs, prefix+".Chance", patrol.Chance)
	requireInheritNonNegative(errs, prefix+".DespawnTime", patrol.DespawnTime, -1)
	requireRespawnTime(errs, prefix+".RespawnTime", patrol.RespawnTime)
	requireEnum(errs, prefix+".WaypointInterpolation", patrol.WaypointInterpolation, waypointInterpolations(), true)
	validateLoadBalancingReference(errs, prefix+".LoadBalancingCategory", patrol.LoadBalancingCategory, categories)
	validateLootDropReference(errs, prefix+".LootDropOnDeath", patrol.LootDropOnDeath, context)
	if patrol.NumberOfAIMax > 0 && patrol.NumberOfAI > patrol.NumberOfAIMax {
		errs.add("%s.NumberOfAI must be less than or equal to %s.NumberOfAIMax", prefix, prefix)
	}
}

func ValidateAILocationSettings(settings AILocationSettings) error {
	var errs validationErrors
	for index, location := range settings.RoamingLocations {
		requireNonNegative(&errs, fmt.Sprintf("RoamingLocations[%d].Radius", index), location.Radius)
	}
	for index, area := range settings.NoGoAreas {
		requireNonNegative(&errs, fmt.Sprintf("NoGoAreas[%d].Radius", index), area.Radius)
		requireNonNegative(&errs, fmt.Sprintf("NoGoAreas[%d].Height", index), area.Height)
	}
	return errs.err()
}

func ValidateLoadout(loadout PrefabObject, name string, context validateContext) error {
	var errs validationErrors
	validatePrefabObject(&errs, loadout, name, "root", context)
	return errs.err()
}

func ValidateLootDrop(entries []PrefabObject, context validateContext) error {
	var errs validationErrors
	for index, entry := range entries {
		validatePrefabObject(&errs, entry, "", fmt.Sprintf("entries[%d]", index), context)
	}
	return errs.err()
}

func validatePrefabObject(errs *validationErrors, object PrefabObject, rootName string, path string, context validateContext) {
	requirePercent(errs, path+".Chance", object.Chance)
	requireMinMax(errs, path+".Quantity", object.Quantity, false)
	for index, health := range object.Health {
		requireMinMax(errs, fmt.Sprintf("%s.Health[%d]", path, index), MinMax{Min: health.Min, Max: health.Max}, true)
	}
	validateIncludeReference(errs, path+".Include", object.Include, rootName, context)
	for index, slot := range object.InventoryAttachments {
		for itemIndex, item := range slot.Items {
			validatePrefabObject(errs, item, rootName, fmt.Sprintf("%s.InventoryAttachments[%d].Items[%d]", path, index, itemIndex), context)
		}
	}
	for index, item := range object.InventoryCargo {
		validatePrefabObject(errs, item, rootName, fmt.Sprintf("%s.InventoryCargo[%d]", path, index), context)
	}
	for index, item := range object.Sets {
		validatePrefabObject(errs, item, rootName, fmt.Sprintf("%s.Sets[%d]", path, index), context)
	}
}

func validateLoadoutReference(errs *validationErrors, field string, value string, context validateContext) {
	if value == "" || len(context.loadoutNames) == 0 {
		return
	}
	if strings.EqualFold(filepath.Ext(value), ".json") {
		errs.add("%s must be a loadout name without .json", field)
		return
	}
	if _, ok := context.loadoutNames[value]; !ok {
		errs.add("%s references missing loadout %q", field, value)
	}
}

func validateLootDropReference(errs *validationErrors, field string, value string, context validateContext) {
	if value == "" || len(context.lootDrops) == 0 {
		return
	}
	if _, ok := context.lootDrops[value]; !ok {
		errs.add("%s references missing loot drop %q", field, value)
	}
}

func validateIncludeReference(errs *validationErrors, field string, value string, rootName string, context validateContext) {
	if value == "" || len(context.loadoutNames) == 0 {
		return
	}
	if strings.EqualFold(value, rootName) {
		errs.add("%s cannot include its own loadout %q", field, value)
		return
	}
	if _, ok := context.loadoutNames[value]; !ok {
		errs.add("%s references missing loadout %q", field, value)
	}
}

func validateLoadBalancingReference(errs *validationErrors, field string, value string, categories map[string][]LoadBalancingRule) {
	if value == "" {
		return
	}
	if _, ok := categories[value]; !ok {
		errs.add("%s references missing load balancing category %q", field, value)
	}
}

func requireEnum(errs *validationErrors, field string, value string, allowed map[string]struct{}, allowEmpty bool) {
	if value == "" && allowEmpty {
		return
	}
	if _, ok := allowed[value]; !ok {
		errs.add("%s has invalid value %q", field, value)
	}
}

func requireBitmask(errs *validationErrors, field string, value string, allowed map[string]struct{}) {
	if strings.TrimSpace(value) == "" {
		return
	}
	for _, part := range strings.Split(value, "|") {
		token := strings.TrimSpace(part)
		if _, ok := allowed[token]; !ok {
			errs.add("%s has invalid value %q", field, token)
		}
	}
}

func requirePercent(errs *validationErrors, field string, value float64) {
	requireRange(errs, field, value, 0, 1)
}

func requireInheritPercent(errs *validationErrors, field string, value float64) {
	if value == -1 {
		return
	}
	requirePercent(errs, field, value)
}

func requireRange(errs *validationErrors, field string, value float64, min float64, max float64) {
	if value < min || value > max {
		errs.add("%s must be between %g and %g", field, min, max)
	}
}

func requireNonNegative(errs *validationErrors, field string, value float64) {
	if value < 0 {
		errs.add("%s must be greater than or equal to 0", field)
	}
}

func requireInheritNonNegative(errs *validationErrors, field string, value float64, inherited float64) {
	if value == inherited {
		return
	}
	requireNonNegative(errs, field, value)
}

func requireNonNegativeInt(errs *validationErrors, field string, value int) {
	if value < 0 {
		errs.add("%s must be greater than or equal to 0", field)
	}
}

func requireInheritNonNegativeInt(errs *validationErrors, field string, value int, inherited int) {
	if value == inherited {
		return
	}
	requireNonNegativeInt(errs, field, value)
}

func requireIntRange(errs *validationErrors, field string, value int, min int, max int) {
	if value < min || value > max {
		errs.add("%s must be between %d and %d", field, min, max)
	}
}

func requireOrdered(errs *validationErrors, minField string, minValue float64, maxField string, maxValue float64) {
	if maxValue < minValue {
		errs.add("%s must be less than or equal to %s", minField, maxField)
	}
}

func requireOrderedUnlessInherited(errs *validationErrors, minField string, minValue float64, maxField string, maxValue float64) {
	if minValue == -1 || maxValue == -1 {
		return
	}
	requireOrdered(errs, minField, minValue, maxField, maxValue)
}

func requireRespawnTime(errs *validationErrors, field string, value float64) {
	if value == -2 || value == -1 {
		return
	}
	requireNonNegative(errs, field, value)
}

func requireUnlimitedReload(errs *validationErrors, field string, value int) {
	if value == 0 || value == 1 {
		return
	}
	const allowedMask = 2 | 4 | 8 | 16
	if value < 0 || value&^allowedMask != 0 {
		errs.add("%s must be 0, 1, or a bitmask composed of 2, 4, 8, and 16", field)
	}
}

func requireMinMax(errs *validationErrors, field string, value MinMax, percent bool) {
	if value.Max < value.Min {
		errs.add("%s.Min must be less than or equal to %s.Max", field, field)
	}
	if percent {
		requirePercent(errs, field+".Min", value.Min)
		requirePercent(errs, field+".Max", value.Max)
	}
}

func countPrefabItems(object PrefabObject) int {
	count := 1
	for _, slot := range object.InventoryAttachments {
		for _, item := range slot.Items {
			count += countPrefabItems(item)
		}
	}
	for _, item := range object.InventoryCargo {
		count += countPrefabItems(item)
	}
	for _, item := range object.Sets {
		count += countPrefabItems(item)
	}
	return count
}

func loadoutName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func documentedFactions() map[string]struct{} {
	return stringSet("West", "East", "Raiders", "Mercenaries", "Civilian", "Passive", "Guards", "InvincibleGuards", "Shamans", "Observers", "InvincibleObservers", "YeetBrigade", "InvincibleYeetBrigade", "Brawlers", "RANDOM")
}

func documentedFormations() map[string]struct{} {
	return stringSet("Column", "InvColumn", "File", "InvFile", "Vee", "InvVee", "Wall", "Star", "StarDot", "Circle", "CircleDot", "RANDOM")
}

func waypointBehaviors() map[string]struct{} {
	return stringSet("HALT", "LOOP", "ALTERNATE", "ONCE", "HALT_OR_LOOP", "HALT_OR_ALTERNATE", "LOOP_OR_ALTERNATE", "ROAMING", "ROAMING_LOCAL", "MIXED")
}

func movementSpeeds() map[string]struct{} {
	return stringSet("STATIC", "WALK", "JOG", "SPRINT", "RANDOM", "RANDOM_NONSTATIC")
}

func stances() map[string]struct{} {
	return stringSet("STANDING", "CROUCHED", "PRONE")
}

func lootingBehaviors() map[string]struct{} {
	return stringSet("WEAPONS_FIREARMS", "WEAPONS_LAUNCHERS", "WEAPONS_MELEE", "WEAPONS", "BANDAGES", "CLOTHING_ARMBAND", "CLOTHING_BACK_LARGE", "CLOTHING_BACK_MEDIUM", "CLOTHING_BACK_SMALL", "CLOTHING_BACK", "CLOTHING_BODY", "CLOTHING_EYEWEAR", "CLOTHING_FEET", "CLOTHING_GLOVES", "CLOTHING_HEADGEAR", "CLOTHING_HIPS", "CLOTHING_LEGS", "CLOTHING_MASK", "CLOTHING_MELEE", "CLOTHING_SHOULDER", "CLOTHING_VEST", "CLOTHING_SIMILAR", "CLOTHING_IDENTICAL", "CLOTHING", "FOOD", "UPGRADE", "DEFAULT", "ALL", "NONE")
}

func waypointInterpolations() map[string]struct{} {
	return stringSet("CatmullRom", "NaturalCubic", "UniformCubic")
}

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
