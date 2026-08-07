package xml

import (
	"io"

	"dzcli/cli/xml/validate"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "xml",
		Short: "Inspect XML files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	command.SetOut(stdout)
	command.AddCommand(validate.NewCommand(stdout))
	return command
}
