package weather

import (
	"fmt"
	"io"

	"dzcli/cli/validation"
	"dzcli/internal/weatherconfig"

	"github.com/spf13/cobra"
)

func NewValidateCommand(stdout io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "weather <cfgweather.xml>",
		Short: "Validate cfgweather.xml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateWeather(args[0], stdout)
		},
	}
	command.SetOut(stdout)
	return command
}

func ValidateWeather(path string, stdout io.Writer) error {
	if err := weatherconfig.ValidateFile(path); err != nil {
		fmt.Fprintf(stdout, "weather %s failed: %v\n", path, err)
		return validation.ErrFailed
	}
	fmt.Fprintf(stdout, "weather %s ok\n", path)
	return nil
}
