package types

import (
	"io"

	"dzcli/cli/economy/types/update"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "types",
		Short: "Modify DayZ types XML files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(update.NewCommand(stdout))
	return command
}
