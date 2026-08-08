package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"dzcli/cli/validation"
	"dzcli/cli/verbs"

	"github.com/spf13/cobra"
)

const (
	SuccessExitCode = 0
	FailureExitCode = 1
)

func Main(args []string, stdout io.Writer, stderr io.Writer, exit func(int)) {
	code := RunWithInput(args, os.Stdin, stdout, stderr)
	if code != SuccessExitCode {
		exit(code)
	}
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithInput(args, os.Stdin, stdout, stderr)
}

func RunWithInput(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	command := NewRootCommandWithInput(stdin, stdout, stderr)
	command.SetArgs(args)

	if err := command.Execute(); err != nil {
		if !errors.Is(err, validation.ErrFailed) {
			fmt.Fprintln(stderr, err)
		}
		return FailureExitCode
	}
	return SuccessExitCode
}

func NewRootCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	return NewRootCommandWithInput(os.Stdin, stdout, stderr)
}

func NewRootCommandWithInput(stdin io.Reader, stdout io.Writer, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:               "dzcli",
		Short:             "Tools for DayZ server configuration",
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(verbs.NewGetCommand(stdout))
	root.AddCommand(verbs.NewCreateCommand(stdout))
	root.AddCommand(verbs.NewUpdateCommand(stdin, stdout))
	root.AddCommand(verbs.NewDeleteCommand(stdin, stdout))
	root.AddCommand(verbs.NewValidateCommand(stdout))
	return root
}
