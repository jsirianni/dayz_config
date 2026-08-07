package limits

import (
	"fmt"
	"io"

	"dzcli/internal/economy"

	"github.com/spf13/cobra"
)

var writeFileMutation = economy.WriteFileMutation

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "limits",
		Short: "Modify DayZ central economy limits definition files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newBaseCommand("add", economy.LimitAdd, stdout))
	command.AddCommand(newBaseCommand("remove", economy.LimitRemove, stdout))
	command.AddCommand(newGroupCommand(stdout))
	return command
}

func newBaseCommand(name string, action economy.LimitAction, stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   name + " <category|tag|usage|value> <name>",
		Short: name + " a base limits definition",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, err := economy.UpdateLimitsFile(file, args[0], args[1], action)
			if err != nil {
				return err
			}
			return outputMutation(file, "limits", mutation, dryRun, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "cfglimitsdefinition.xml path")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified XML without writing")
	return command
}

func newGroupCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "group",
		Short: "Modify user limits groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newGroupAddCommand(stdout))
	command.AddCommand(newGroupRemoveCommand(stdout))
	command.AddCommand(newMemberCommand(stdout))
	return command
}

func newGroupAddCommand(stdout io.Writer) *cobra.Command {
	var file string
	var members []string
	var dryRun bool
	command := &cobra.Command{
		Use:   "add <usage|value> <group-name>",
		Short: "Add a user limits group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserGroupMutation(file, dryRun, stdout, economy.UserLimitGroupOptions{
				Kind:      args[0],
				GroupName: args[1],
				Members:   members,
				Action:    economy.UserGroupAdd,
			})
		},
	}
	addUserGroupFlags(command, &file, &dryRun)
	command.Flags().StringArrayVar(&members, "member", nil, "group member name")
	return command
}

func newGroupRemoveCommand(stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   "remove <usage|value> <group-name>",
		Short: "Remove a user limits group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserGroupMutation(file, dryRun, stdout, economy.UserLimitGroupOptions{
				Kind:      args[0],
				GroupName: args[1],
				Action:    economy.UserGroupRemove,
			})
		},
	}
	addUserGroupFlags(command, &file, &dryRun)
	return command
}

func newMemberCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "member",
		Short: "Modify user limits group members",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(newMemberMutationCommand("add", economy.UserGroupMemberAdd, stdout))
	command.AddCommand(newMemberMutationCommand("remove", economy.UserGroupMemberRemove, stdout))
	return command
}

func newMemberMutationCommand(name string, action economy.UserGroupAction, stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   name + " <usage|value> <group-name> <member-name>",
		Short: name + " a user limits group member",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserGroupMutation(file, dryRun, stdout, economy.UserLimitGroupOptions{
				Kind:      args[0],
				GroupName: args[1],
				Member:    args[2],
				Action:    action,
			})
		},
	}
	addUserGroupFlags(command, &file, &dryRun)
	return command
}

func addUserGroupFlags(command *cobra.Command, file *string, dryRun *bool) {
	command.Flags().StringVar(file, "file", "", "cfglimitsdefinitionuser.xml path")
	command.Flags().BoolVar(dryRun, "dry-run", false, "print modified XML without writing")
}

func runUserGroupMutation(file string, dryRun bool, stdout io.Writer, options economy.UserLimitGroupOptions) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	mutation, err := economy.UpdateUserLimitGroupFile(file, options)
	if err != nil {
		return err
	}
	return outputMutation(file, "limits", mutation, dryRun, stdout)
}

func outputMutation(path string, kind string, mutation economy.FileMutation, dryRun bool, stdout io.Writer) error {
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
