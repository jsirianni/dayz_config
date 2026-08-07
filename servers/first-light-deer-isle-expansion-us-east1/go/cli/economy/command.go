package economy

import (
	"io"

	"dzcli/cli/economy/limits"
	"dzcli/cli/economy/types"
	"dzcli/cli/economy/validate"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "economy",
		Short: "Inspect DayZ central economy configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(limits.NewCommand(stdout))
	command.AddCommand(types.NewCommand(stdout))
	command.AddCommand(validate.NewCommand(stdout))
	return command
}
