package gameplay

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"dzcli/cli/validation"
	"dzcli/internal/gameplayconfig"

	"github.com/spf13/cobra"
)

var writeFileMutation = gameplayconfig.WriteFileMutation

func NewGetCommand(stdout io.Writer) *cobra.Command {
	var file string
	command := &cobra.Command{
		Use:   "gameplay [field]",
		Short: "List cfggameplay.json fields",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 1 {
				filter = gameplayconfig.NormalizeField(args[0])
			}
			return ListGameplay(file, filter, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "cfggameplay.json path")
	return command
}

func NewValidateCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "gameplay <cfggameplay.json>",
		Short: "Validate cfggameplay.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateGameplay(args[0], stdout)
		},
	}
	command.SetOut(stdout)
	return command
}

func NewUpdateCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	var file string
	var values []string
	var clear bool
	var force bool
	var dryRun bool
	command := &cobra.Command{
		Use:   "gameplay <field>",
		Short: "Set a cfggameplay.json field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, existed, err := gameplayconfig.UpdateFieldFile(file, args[0], values, gameplayconfig.UpdateFieldOptions{Clear: clear})
			if err != nil {
				return err
			}
			if existed && !force && !dryRun {
				confirmed, err := confirmOverwrite(stdin, stdout, gameplayconfig.NormalizeField(args[0]), file)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintf(stdout, "gameplay %s not modified\n", file)
					return nil
				}
			}
			return outputMutation(file, mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "cfggameplay.json path")
	command.Flags().StringArrayVar(&values, "value", nil, "field value; repeat for arrays")
	command.Flags().BoolVar(&clear, "clear", false, "set an array field to []")
	command.Flags().BoolVar(&force, "force", false, "overwrite without prompting")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	return command
}

func NewDeleteCommand(stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   "gameplay <field>",
		Short: "Remove a cfggameplay.json field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, err := gameplayconfig.DeleteFieldFile(file, args[0])
			if err != nil {
				return err
			}
			return outputMutation(file, mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "cfggameplay.json path")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified JSON without writing")
	return command
}

func ValidateGameplay(path string, stdout io.Writer) error {
	if err := gameplayconfig.ValidateFile(path); err != nil {
		fmt.Fprintf(stdout, "gameplay %s failed: %v\n", path, err)
		return validation.ErrFailed
	}
	fmt.Fprintf(stdout, "gameplay %s ok\n", path)
	return nil
}

func ListGameplay(file string, filter string, stdout io.Writer) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	var rows [][]string
	if filter != "" {
		value, ok, err := gameplayconfig.GetFieldFile(file, filter)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("gameplay field %q not found", filter)
		}
		rows = append(rows, []string{value.Field, value.Value})
	} else {
		values, err := gameplayconfig.ListFieldsFile(file)
		if err != nil {
			return err
		}
		for _, value := range values {
			rows = append(rows, []string{value.Field, value.Value})
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no gameplay fields found")
	}
	printTable(stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func confirmOverwrite(stdin io.Reader, stdout io.Writer, field string, file string) (bool, error) {
	fmt.Fprintf(stdout, "Overwrite %s in %s? [y/N]: ", field, file)
	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

func outputMutation(path string, mutation gameplayconfig.FileMutation, dryRun bool, stdout io.Writer) error {
	if dryRun {
		_, err := stdout.Write(mutation.Data)
		return err
	}
	if err := writeFileMutation(path, mutation); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "gameplay %s ok\n", path)
	return nil
}

func printTable(stdout io.Writer, headers []string, rows [][]string) {
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	_ = writer.Flush()
}
