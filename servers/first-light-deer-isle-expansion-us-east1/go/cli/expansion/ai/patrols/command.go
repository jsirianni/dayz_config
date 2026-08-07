package patrols

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

var writeFileMutation = expansion.WriteFileMutation

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

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "patrols",
		Short: "Modify DayZ Expansion AI patrol definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newCreateCommand(stdout))
	command.AddCommand(newUpdateCommand(stdout))
	command.AddCommand(newDeleteCommand(stdout))
	return command
}

func newCreateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "create <AIPatrolSettings.json>",
		Short: "Append a patrol definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := patrolPatchFromCommand(cmd, "name")
			if err != nil {
				return err
			}
			mutation, err := expansion.CreatePatrolFile(args[0], patch)
			if err != nil {
				return err
			}
			return outputMutation(args[0], "patrols", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	addPatrolFieldFlags(command, "name")
	return command
}

func newUpdateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var index int
	var name string
	var occurrence int
	command := &cobra.Command{
		Use:   "update <AIPatrolSettings.json>",
		Short: "Modify a patrol definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := patrolPatchFromCommand(cmd, "set-name")
			if err != nil {
				return err
			}
			mutation, err := expansion.UpdatePatrolFile(args[0], expansion.PatrolUpdateOptions{
				Selector: expansion.PatrolSelector{Index: index, Name: name, Occurrence: occurrence},
				Patch:    patch,
			})
			if err != nil {
				return err
			}
			return outputMutation(args[0], "patrols", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	command.Flags().IntVar(&index, "index", 0, "select a 1-based patrol index")
	command.Flags().StringVar(&name, "name", "", "select a patrol by name")
	command.Flags().IntVar(&occurrence, "occurrence", 0, "select a duplicate patrol name occurrence")
	addPatrolFieldFlags(command, "set-name")
	return command
}

func newDeleteCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var index int
	var name string
	var occurrence int
	command := &cobra.Command{
		Use:   "delete <AIPatrolSettings.json>",
		Short: "Remove a patrol definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mutation, err := expansion.DeletePatrolFile(args[0], expansion.PatrolSelector{Index: index, Name: name, Occurrence: occurrence})
			if err != nil {
				return err
			}
			return outputMutation(args[0], "patrols", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	command.Flags().IntVar(&index, "index", 0, "select a 1-based patrol index")
	command.Flags().StringVar(&name, "name", "", "select a patrol by name")
	command.Flags().IntVar(&occurrence, "occurrence", 0, "select a duplicate patrol name occurrence")
	return command
}

func addPatrolFieldFlags(command *cobra.Command, nameFlag string) {
	flags := command.Flags()
	flags.String(nameFlag, "", "set patrol name")
	for flag := range patrolStringFlags {
		if flag != "set-name" {
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

func patrolPatchFromCommand(cmd *cobra.Command, nameFlag string) (expansion.PatrolPatch, error) {
	flags := cmd.Flags()
	patch := expansion.PatrolPatch{
		Strings:         map[string]string{},
		Floats:          map[string]float64{},
		Ints:            map[string]int{},
		BoolInts:        map[string]int{},
		InheritBools:    map[string]int{},
		UpdateWaypoints: map[int]expansion.Vector{},
	}
	if flags.Changed(nameFlag) {
		value, _ := flags.GetString(nameFlag)
		patch.Strings["Name"] = value
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
