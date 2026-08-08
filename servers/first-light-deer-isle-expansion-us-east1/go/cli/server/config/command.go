package config

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"dzcli/cli/validation"
	"dzcli/internal/serverconfig"

	"github.com/spf13/cobra"
)

var writeFileMutation = serverconfig.WriteFileMutation

func NewGetCommand(stdout io.Writer) *cobra.Command {
	var file string
	command := &cobra.Command{
		Use:   "server [field]",
		Short: "List serverDZ.cfg fields",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 1 {
				filter = serverconfig.NormalizeField(args[0])
			}
			return ListConfig(file, filter, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "serverDZ.cfg path")
	return command
}

func NewValidateCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "server <serverDZ.cfg>",
		Short: "Validate serverDZ.cfg",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateConfig(args[0], stdout)
		},
	}
	command.SetOut(stdout)
	return command
}

func NewUpdateCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	var file string
	var values []string
	var force bool
	var dryRun bool
	var allowUnknown bool
	command := &cobra.Command{
		Use:   "server <field>",
		Short: "Set a serverDZ.cfg field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, existed, err := serverconfig.UpdateFieldFile(file, args[0], values, serverconfig.UpdateFieldOptions{
				AllowUnknown: allowUnknown,
			})
			if err != nil {
				return err
			}
			if existed && !force && !dryRun {
				confirmed, err := confirmOverwrite(stdin, stdout, serverconfig.NormalizeField(args[0]), file)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintf(stdout, "server %s not modified\n", file)
					return nil
				}
			}
			return outputMutation(file, mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "serverDZ.cfg path")
	command.Flags().StringArrayVar(&values, "value", nil, "field value; repeat for motd")
	command.Flags().BoolVar(&force, "force", false, "overwrite without prompting")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified config without writing")
	command.Flags().BoolVar(&allowUnknown, "allow-unknown", false, "allow inserting an undocumented field")
	return command
}

func NewDeleteCommand(stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   "server <field>",
		Short: "Remove a serverDZ.cfg field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, err := serverconfig.DeleteFieldFile(file, args[0])
			if err != nil {
				return err
			}
			return outputMutation(file, mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "serverDZ.cfg path")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified config without writing")
	return command
}

func ValidateConfig(path string, stdout io.Writer) error {
	if err := serverconfig.ValidateFile(path); err != nil {
		fmt.Fprintf(stdout, "server %s failed: %v\n", path, err)
		return validation.ErrFailed
	}
	fmt.Fprintf(stdout, "server %s ok\n", path)
	return nil
}

func ListConfig(file string, filter string, stdout io.Writer) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	values, err := serverconfig.ListFieldsFile(file)
	if err != nil {
		return err
	}
	values = serverconfig.SortFieldValues(values)
	var rows [][]string
	for _, value := range values {
		if filter == "" || value.Field == filter {
			rows = append(rows, []string{value.Field, value.Value})
		}
	}
	if len(rows) == 0 {
		if filter != "" {
			return fmt.Errorf("server config field %q not found", filter)
		}
		return fmt.Errorf("no server config fields found")
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

func outputMutation(path string, mutation serverconfig.FileMutation, dryRun bool, stdout io.Writer) error {
	if dryRun {
		_, err := stdout.Write(mutation.Data)
		return err
	}
	if err := writeFileMutation(path, mutation); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "server %s ok\n", path)
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
