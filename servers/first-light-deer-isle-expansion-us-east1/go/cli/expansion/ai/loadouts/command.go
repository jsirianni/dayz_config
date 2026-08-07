package loadouts

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

var writeFileMutation = expansion.WriteFileMutation
var deleteLoadoutFile = expansion.DeleteLoadoutFile

var prefabStringFlags = map[string]string{
	"class-name": "ClassName",
	"include":    "Include",
}

var prefabFloatFlags = map[string]string{
	"chance": "Chance",
}

func NewCommand(stdout io.Writer) *cobra.Command {
	return NewCommandWithInput(os.Stdin, stdout)
}

func NewCommandWithInput(stdin io.Reader, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "loadouts",
		Short: "Modify DayZ Expansion AI loadout files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newCreateCommand(stdout))
	command.AddCommand(newUpdateCommand(stdout))
	command.AddCommand(newDeleteCommand(stdin, stdout))
	command.AddCommand(newItemCommand(stdout))
	return command
}

func newCreateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "create <loadout.json>",
		Short: "Create a loadout file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := prefabPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			mutation, err := expansion.CreateLoadoutFile(args[0], patch)
			if err != nil {
				return err
			}
			return outputMutation(args[0], "loadouts", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	addPrefabFieldFlags(command)
	return command
}

func newUpdateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "update <loadout.json>",
		Short: "Modify the root object in a loadout file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := prefabPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			mutation, err := expansion.UpdateLoadoutFile(args[0], patch)
			if err != nil {
				return err
			}
			return outputMutation(args[0], "loadouts", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	addPrefabFieldFlags(command)
	return command
}

func newDeleteCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool
	var patrolsFile string
	command := &cobra.Command{
		Use:   "delete <loadout.json>",
		Short: "Delete a loadout file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := expansion.PlanLoadoutDelete(args[0], patrolsFile)
			if err != nil {
				return err
			}
			if dryRun {
				printDeletePlan(stdout, plan)
				return nil
			}
			if !force {
				confirmed, err := confirmDelete(stdin, stdout, plan)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintf(stdout, "loadouts %s not deleted\n", args[0])
					return nil
				}
			}
			if err := deleteLoadoutFile(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "loadouts %s ok\n", args[0])
			return nil
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print planned delete without deleting")
	command.Flags().BoolVar(&force, "force", false, "delete without prompting")
	command.Flags().StringVar(&patrolsFile, "patrols-file", "", "AIPatrolSettings.json path for reference checks")
	return command
}

func newItemCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "item",
		Short: "Modify nested loadout items",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newItemAddCommand(stdout))
	command.AddCommand(newItemUpdateCommand(stdout))
	command.AddCommand(newItemRemoveCommand(stdout))
	return command
}

func newItemAddCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var parent string
	var container string
	var slot string
	command := &cobra.Command{
		Use:   "add <loadout.json>",
		Short: "Add a nested loadout item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if parent == "" {
				return fmt.Errorf("--parent is required")
			}
			if container == "" {
				return fmt.Errorf("--container is required")
			}
			patch, err := prefabPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			mutation, err := expansion.AddLoadoutItemFile(args[0], expansion.LoadoutItemAddOptions{
				ParentPath: parent,
				Container:  container,
				Slot:       slot,
				Patch:      patch,
			})
			if err != nil {
				return err
			}
			return outputMutation(args[0], "loadouts", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	command.Flags().StringVar(&parent, "parent", "", "parent item path")
	command.Flags().StringVar(&container, "container", "", "attachment, cargo, or set")
	command.Flags().StringVar(&slot, "slot", "", "attachment slot name")
	addPrefabFieldFlags(command)
	return command
}

func newItemUpdateCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "update <loadout.json> <path>",
		Short: "Modify a nested loadout item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := prefabPatchFromCommand(cmd)
			if err != nil {
				return err
			}
			mutation, err := expansion.UpdateLoadoutItemFile(args[0], args[1], patch)
			if err != nil {
				return err
			}
			return outputMutation(args[0], "loadouts", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	addPrefabFieldFlags(command)
	return command
}

func newItemRemoveCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "remove <loadout.json> <path>",
		Short: "Remove a nested loadout item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mutation, err := expansion.RemoveLoadoutItemFile(args[0], args[1])
			if err != nil {
				return err
			}
			return outputMutation(args[0], "loadouts", mutation, dryRun, stdout)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	return command
}

func addPrefabFieldFlags(command *cobra.Command) {
	flags := command.Flags()
	for flag := range prefabStringFlags {
		flags.String(flag, "", "set "+flag)
	}
	for flag := range prefabFloatFlags {
		flags.Float64(flag, 0, "set "+flag)
	}
	flags.Float64("quantity-min", 0, "set Quantity.Min")
	flags.Float64("quantity-max", 0, "set Quantity.Max")
	flags.StringArray("health", nil, "set health as zone=min,max; use =min,max for global")
	flags.StringArray("remove-health", nil, "remove health by zone")
	flags.Bool("clear-health", false, "remove all health entries")
	flags.StringArray("set-construction-part", nil, "replace ConstructionPartsBuilt")
	flags.StringArray("add-construction-part", nil, "add a construction part")
	flags.StringArray("remove-construction-part", nil, "remove a construction part")
	flags.Bool("clear-construction-parts", false, "remove all construction parts")
}

func prefabPatchFromCommand(cmd *cobra.Command) (expansion.PrefabObjectPatch, error) {
	flags := cmd.Flags()
	patch := expansion.PrefabObjectPatch{
		Strings:   map[string]string{},
		Floats:    map[string]float64{},
		SetHealth: map[string]expansion.MinMax{},
	}
	for flag, field := range prefabStringFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetString(flag)
			patch.Strings[field] = value
		}
	}
	for flag, field := range prefabFloatFlags {
		if flags.Changed(flag) {
			value, _ := flags.GetFloat64(flag)
			patch.Floats[field] = value
		}
	}
	if flags.Changed("quantity-min") {
		value, _ := flags.GetFloat64("quantity-min")
		patch.QuantityMin = &value
	}
	if flags.Changed("quantity-max") {
		value, _ := flags.GetFloat64("quantity-max")
		patch.QuantityMax = &value
	}
	healthValues, _ := flags.GetStringArray("health")
	for _, value := range healthValues {
		zone, minMax, err := parseHealth(value)
		if err != nil {
			return expansion.PrefabObjectPatch{}, err
		}
		patch.SetHealth[zone] = minMax
	}
	patch.RemoveHealth, _ = flags.GetStringArray("remove-health")
	patch.ClearHealth, _ = flags.GetBool("clear-health")
	if flags.Changed("set-construction-part") {
		values, _ := flags.GetStringArray("set-construction-part")
		patch.ConstructionParts.Set = &values
	}
	patch.ConstructionParts.Add, _ = flags.GetStringArray("add-construction-part")
	patch.ConstructionParts.Remove, _ = flags.GetStringArray("remove-construction-part")
	patch.ConstructionParts.Clear, _ = flags.GetBool("clear-construction-parts")
	return patch, nil
}

func parseHealth(value string) (string, expansion.MinMax, error) {
	zone, rangeText, ok := strings.Cut(value, "=")
	if !ok {
		return "", expansion.MinMax{}, fmt.Errorf("health must be zone=min,max")
	}
	parts := strings.Split(rangeText, ",")
	if len(parts) != 2 {
		return "", expansion.MinMax{}, fmt.Errorf("health must be zone=min,max")
	}
	min, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return "", expansion.MinMax{}, fmt.Errorf("health min %q is not a number", parts[0])
	}
	max, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return "", expansion.MinMax{}, fmt.Errorf("health max %q is not a number", parts[1])
	}
	return zone, expansion.MinMax{Min: min, Max: max}, nil
}

func confirmDelete(stdin io.Reader, stdout io.Writer, plan expansion.LoadoutDeletePlan) (bool, error) {
	if plan.References > 0 {
		fmt.Fprintf(stdout, "Delete loadout %s referenced by %d patrols? [y/N]: ", plan.Path, plan.References)
	} else {
		fmt.Fprintf(stdout, "Delete loadout %s? [y/N]: ", plan.Path)
	}
	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

func printDeletePlan(stdout io.Writer, plan expansion.LoadoutDeletePlan) {
	if plan.References > 0 {
		fmt.Fprintf(stdout, "loadouts %s would delete (%d patrol references)\n", plan.Path, plan.References)
		return
	}
	fmt.Fprintf(stdout, "loadouts %s would delete\n", plan.Path)
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
