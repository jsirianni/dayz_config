package expansion

import (
	"io"

	"dzcli/cli/expansion/ai"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "expansion",
		Short: "Inspect DayZ Expansion mod configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(ai.NewCommand(stdout))
	return command
}
