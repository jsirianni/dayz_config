package validate

import (
	"fmt"
	"io"

	"dzcli/cli/validation"
	"dzcli/internal/dayzinit"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "init <init.c>",
		Short: "Validate DayZ mission init.c",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateInit(args[0], stdout)
		},
	}
	command.SetOut(stdout)
	return command
}

func ValidateInit(path string, stdout io.Writer) error {
	if err := dayzinit.ValidateFile(path); err != nil {
		fmt.Fprintf(stdout, "init %s failed: %v\n", path, err)
		return validation.ErrFailed
	}
	fmt.Fprintf(stdout, "init %s ok\n", path)
	return nil
}
