package ai

import (
	"io"
	"os"

	"dzcli/cli/expansion/ai/loadouts"
	"dzcli/cli/expansion/ai/patrols"
	"dzcli/cli/expansion/ai/validate"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	return NewCommandWithInput(os.Stdin, stdout)
}

func NewCommandWithInput(stdin io.Reader, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "ai",
		Short: "Inspect DayZ Expansion AI configuration",
		Long: `Inspect, validate, and mutate DayZ Expansion AI configuration.

Commands accept explicit file paths and work the same from a git repo checkout
or a live server directory. Nearby path discovery is only used for optional
reference validation when enough context exists.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(loadouts.NewCommandWithInput(stdin, stdout))
	command.AddCommand(patrols.NewCommand(stdout))
	command.AddCommand(validate.NewCommand(stdout))
	return command
}
