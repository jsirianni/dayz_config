package patrols

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

var writeFileMutation = expansion.WriteFileMutation
var getwd = os.Getwd

var patrolStringFlags = map[string]string{
	"set-name":                "Name",
	"faction":                 "Faction",
	"formation":               "Formation",
	"loadout":                 "Loadout",
	"behaviour":               "Behaviour",
	"looting-behaviour":       "LootingBehaviour",
	"speed":                   "Speed",
	"under-threat-speed":      "UnderThreatSpeed",
	"default-stance":          "DefaultStance",
	"loot-drop-on-death":      "LootDropOnDeath",
	"load-balancing-category": "LoadBalancingCategory",
	"object-class-name":       "ObjectClassName",
	"waypoint-interpolation":  "WaypointInterpolation",
}

var patrolFloatFlags = map[string]string{
	"formation-scale":                    "FormationScale",
	"formation-looseness":                "FormationLooseness",
	"default-look-angle":                 "DefaultLookAngle",
	"sniper-prone-distance-threshold":    "SniperProneDistanceThreshold",
	"accuracy-min":                       "AccuracyMin",
	"accuracy-max":                       "AccuracyMax",
	"threat-distance-limit":              "ThreatDistanceLimit",
	"noise-investigation-distance-limit": "NoiseInvestigationDistanceLimit",
	"max-flanking-distance":              "MaxFlankingDistance",
	"damage-multiplier":                  "DamageMultiplier",
	"damage-received-multiplier":         "DamageReceivedMultiplier",
	"headshot-resistance":                "HeadshotResistance",
	"shoryuken-chance":                   "ShoryukenChance",
	"shoryuken-damage-multiplier":        "ShoryukenDamageMultiplier",
	"min-dist-radius":                    "MinDistRadius",
	"max-dist-radius":                    "MaxDistRadius",
	"despawn-radius":                     "DespawnRadius",
	"min-spread-radius":                  "MinSpreadRadius",
	"max-spread-radius":                  "MaxSpreadRadius",
	"chance":                             "Chance",
	"despawn-time":                       "DespawnTime",
	"respawn-time":                       "RespawnTime",
}

var patrolIntFlags = map[string]string{
	"number-of-ai":     "NumberOfAI",
	"number-of-ai-max": "NumberOfAIMax",
	"unlimited-reload": "UnlimitedReload",
}

var patrolBoolIntFlags = map[string]string{
	"persist":                            "Persist",
	"can-be-looted":                      "CanBeLooted",
	"can-spawn-in-contaminated-area":     "CanSpawnInContaminatedArea",
	"can-be-triggered-by-ai":             "CanBeTriggeredByAI",
	"use-random-waypoint-as-start-point": "UseRandomWaypointAsStartPoint",
}

var patrolInheritBoolFlags = map[string]string{
	"enable-flanking-outside-combat": "EnableFlankingOutsideCombat",
}

func NewCreateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var file string
	command := &cobra.Command{
		Use:   "patrols <name>",
		Short: "Append a patrol definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetFile, err := resolvePatrolsFile(file)
			if err != nil {
				return err
			}
			patch, err := patrolPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			patch.Strings["Name"] = args[0]
			mutation, err := expansion.CreatePatrolFile(targetFile, patch)
			if err != nil {
				return err
			}
			return outputMutation(targetFile, "patrols", mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	addCommonFlags(command, &file, &dryRun)
	addPatrolFieldFlags(command, false)
	return command
}

func NewUpdateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var file string
	var index int
	var occurrence int
	command := &cobra.Command{
		Use:   "patrols [name]",
		Short: "Modify a patrol definition",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetFile, err := resolvePatrolsFile(file)
			if err != nil {
				return err
			}
			patch, err := patrolPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			selector := expansion.PatrolSelector{Index: index, Occurrence: occurrence}
			if len(args) == 1 {
				selector.Name = args[0]
			}
			mutation, err := expansion.UpdatePatrolFile(targetFile, expansion.PatrolUpdateOptions{
				Selector: selector,
				Patch:    patch,
			})
			if err != nil {
				return err
			}
			return outputMutation(targetFile, "patrols", mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	addCommonFlags(command, &file, &dryRun)
	command.Flags().IntVar(&index, "index", 0, "select a 1-based patrol index")
	command.Flags().IntVar(&occurrence, "occurrence", 0, "select a duplicate patrol name occurrence")
	addPatrolFieldFlags(command, true)
	return command
}

func NewDeleteCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var file string
	var index int
	var occurrence int
	command := &cobra.Command{
		Use:   "patrols [name]",
		Short: "Remove a patrol definition",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetFile, err := resolvePatrolsFile(file)
			if err != nil {
				return err
			}
			selector := expansion.PatrolSelector{Index: index, Occurrence: occurrence}
			if len(args) == 1 {
				selector.Name = args[0]
			}
			mutation, err := expansion.DeletePatrolFile(targetFile, selector)
			if err != nil {
				return err
			}
			return outputMutation(targetFile, "patrols", mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	addCommonFlags(command, &file, &dryRun)
	command.Flags().IntVar(&index, "index", 0, "select a 1-based patrol index")
	command.Flags().IntVar(&occurrence, "occurrence", 0, "select a duplicate patrol name occurrence")
	return command
}

func addCommonFlags(command *cobra.Command, file *string, dryRun *bool) {
	command.Flags().StringVar(file, "file", "", "AIPatrolSettings.json path")
	command.Flags().BoolVar(dryRun, "dry-run", false, "print modified JSON without writing")
}

func addPatrolFieldFlags(command *cobra.Command, includeRename bool) {
	flags := command.Flags()
	for flag := range patrolStringFlags {
		if flag != "set-name" || includeRename {
			flags.String(flag, "", "set "+flag)
		}
	}
	for flag := range patrolFloatFlags {
		flags.Float64(flag, 0, "set "+flag)
	}
	for flag := range patrolIntFlags {
		flags.Int(flag, 0, "set "+flag)
	}
	for flag := range patrolBoolIntFlags {
		flags.Int(flag, 0, "set "+flag+" as 0 or 1")
	}
	for flag := range patrolInheritBoolFlags {
		flags.Int(flag, 0, "set "+flag+" as -1, 0, or 1")
	}
	flags.StringArray("set-unit", nil, "replace Units with this class name")
	flags.StringArray("add-unit", nil, "add a unit class name")
	flags.StringArray("remove-unit", nil, "remove a unit class name")
	flags.Bool("clear-units", false, "remove all units")
	flags.StringArray("waypoint", nil, "replace Waypoints with x,y,z")
	flags.StringArray("add-waypoint", nil, "append waypoint x,y,z")
	flags.StringArray("set-waypoint", nil, "update waypoint as n=x,y,z")
	flags.IntSlice("remove-waypoint", nil, "remove 1-based waypoint index")
	flags.Bool("clear-waypoints", false, "remove all waypoints")
}

func patrolPatchFromCommand(cmd *cobra.Command) (expansion.PatrolPatch, error) {
	flags := cmd.Flags()
	patch := expansion.PatrolPatch{
		Strings:         map[string]string{},
		Floats:          map[string]float64{},
		Ints:            map[string]int{},
		BoolInts:        map[string]int{},
		InheritBools:    map[string]int{},
		UpdateWaypoints: map[int]expansion.Vector{},
	}
	for flag, field := range patrolStringFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetString(flag)
			patch.Strings[field] = value
		}
	}
	for flag, field := range patrolFloatFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetFloat64(flag)
			patch.Floats[field] = value
		}
	}
	for flag, field := range patrolIntFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetInt(flag)
			patch.Ints[field] = value
		}
	}
	for flag, field := range patrolBoolIntFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetInt(flag)
			patch.BoolInts[field] = value
		}
	}
	for flag, field := range patrolInheritBoolFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetInt(flag)
			patch.InheritBools[field] = value
		}
	}
	if flags.Changed("set-unit") {
		values, _ := flags.GetStringArray("set-unit")
		patch.SetUnits = &values
	}
	patch.AddUnits, _ = flags.GetStringArray("add-unit")
	patch.RemoveUnits, _ = flags.GetStringArray("remove-unit")
	patch.ClearUnits, _ = flags.GetBool("clear-units")
	if flags.Changed("waypoint") {
		values, _ := flags.GetStringArray("waypoint")
		waypoints, err := parseVectorsFlag(values)
		if err != nil {
			return expansion.PatrolPatch{}, err
		}
		patch.SetWaypoints = &waypoints
	}
	addWaypointValues, _ := flags.GetStringArray("add-waypoint")
	waypoints, err := parseVectorsFlag(addWaypointValues)
	if err != nil {
		return expansion.PatrolPatch{}, err
	}
	patch.AddWaypoints = waypoints
	updateWaypointValues, _ := flags.GetStringArray("set-waypoint")
	updates, err := parseWaypointUpdatesFlag(updateWaypointValues)
	if err != nil {
		return expansion.PatrolPatch{}, err
	}
	patch.UpdateWaypoints = updates
	patch.RemoveWaypoints, _ = flags.GetIntSlice("remove-waypoint")
	patch.ClearWaypoints, _ = flags.GetBool("clear-waypoints")
	return patch, nil
}

func resolvePatrolsFile(file string) (string, error) {
	if file != "" {
		return file, nil
	}
	wd, err := getwd()
	if err != nil {
		return "", err
	}
	return expansion.FindAIPatrolsFileNear(wd)
}

func parseVectorsFlag(values []string) ([]expansion.Vector, error) {
	waypoints := make([]expansion.Vector, 0, len(values))
	for _, value := range values {
		vector, err := parseVector(value)
		if err != nil {
			return nil, err
		}
		waypoints = append(waypoints, vector)
	}
	return waypoints, nil
}

func parseWaypointUpdatesFlag(values []string) (map[int]expansion.Vector, error) {
	updates := map[int]expansion.Vector{}
	for _, value := range values {
		left, right, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("set-waypoint must be n=x,y,z")
		}
		position, err := strconv.Atoi(left)
		if err != nil || position < 1 {
			return nil, fmt.Errorf("waypoint index must be greater than 0")
		}
		vector, err := parseVector(right)
		if err != nil {
			return nil, err
		}
		updates[position] = vector
	}
	return updates, nil
}

func parseVector(value string) (expansion.Vector, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return expansion.Vector{}, fmt.Errorf("waypoint must be x,y,z")
	}
	var vector expansion.Vector
	for index, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return expansion.Vector{}, fmt.Errorf("waypoint coordinate %q is not a number", part)
		}
		vector[index] = number
	}
	return vector, nil
}

func outputMutation(path string, kind string, mutation expansion.FileMutation, dryRun bool, stdout io.Writer) error {
	if dryRun {
		_, err := stdout.Write(mutation.Data)
		return err
	}
	if err := writeFileMutation(path, mutation); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s %s ok\n", kind, path)
	return nil
}
