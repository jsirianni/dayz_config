package ai

import (
	"io"

	"dzcli/cli/expansion/ai/validate"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "ai",
		Short: "Inspect DayZ Expansion AI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(validate.NewCommand(stdout))
	return command
}
