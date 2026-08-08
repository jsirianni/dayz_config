package limits

import (
	"fmt"
	"io"

	"dzcli/internal/economy"

	"github.com/spf13/cobra"
)

var writeFileMutation = economy.WriteFileMutation

func NewCreateCommand(stdout io.Writer) *cobra.Command {
	return newCommand(economy.LimitAdd, economy.UserGroupAdd, economy.UserGroupMemberAdd, stdout)
}

func NewDeleteCommand(stdout io.Writer) *cobra.Command {
	return newCommand(economy.LimitRemove, economy.UserGroupRemove, economy.UserGroupMemberRemove, stdout)
}

func newCommand(baseAction economy.LimitAction, groupAction economy.UserGroupAction, memberAction economy.UserGroupAction, stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   "limits <category|tag|usage|value> <name>",
		Short: verb(baseAction) + " a base limits definition",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			mutation, err := economy.UpdateLimitsFile(file, args[0], args[1], baseAction)
			if err != nil {
				return err
			}
			return outputMutation(file, "limits", mutation, dryRun, stdout)
		},
	}
	command.SetOut(stdout)
	command.Flags().StringVar(&file, "file", "", "cfglimitsdefinition.xml path")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "print modified XML without writing")
	command.AddCommand(newGroupCommand(groupAction, memberAction, stdout))
	return command
}

func newGroupCommand(groupAction economy.UserGroupAction, memberAction economy.UserGroupAction, stdout io.Writer) *cobra.Command {
	var file string
	var members []string
	var dryRun bool
	command := &cobra.Command{
		Use:   "group <usage|value> <group-name>",
		Short: verb(groupAction) + " a user limits group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserGroupMutation(file, dryRun, stdout, economy.UserLimitGroupOptions{
				Kind:      args[0],
				GroupName: args[1],
				Members:   members,
				Action:    groupAction,
			})
		},
	}
	command.SetOut(stdout)
	addUserGroupFlags(command, &file, &dryRun)
	if groupAction == economy.UserGroupAdd {
		command.Flags().StringArrayVar(&members, "member", nil, "group member name")
	}
	command.AddCommand(newMemberCommand(memberAction, stdout))
	return command
}

func newMemberCommand(action economy.UserGroupAction, stdout io.Writer) *cobra.Command {
	var file string
	var dryRun bool
	command := &cobra.Command{
		Use:   "member <usage|value> <group-name> <member-name>",
		Short: verb(action) + " a user limits group member",
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
	command.SetOut(stdout)
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

func verb(action any) string {
	switch action {
	case economy.LimitAdd, economy.UserGroupAdd, economy.UserGroupMemberAdd:
		return "create"
	case economy.LimitRemove, economy.UserGroupRemove, economy.UserGroupMemberRemove:
		return "delete"
	default:
		return "modify"
	}
}
