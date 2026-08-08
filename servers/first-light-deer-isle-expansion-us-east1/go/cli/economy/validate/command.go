package validate

import (
	"fmt"
	"io"

	"dzcli/cli/validation"
	"dzcli/internal/economyconfig"

	"github.com/spf13/cobra"
)

func NewCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "economy <mission-root|cfgeconomycore.xml|economy-file>",
		Short: "Validate central economy files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ValidateEconomy(args[0], stdout)
		},
	}
}

func ValidateEconomy(path string, stdout io.Writer) error {
	statuses, err := economyconfig.InspectEconomy(path)
	if err != nil {
		return fmt.Errorf("economy: failed: %w", err)
	}
	return printStatuses(statuses, stdout)
}

func ValidateEconomyCore(path string, stdout io.Writer) error {
	statuses, err := economyconfig.InspectEconomyCore(path)
	if err != nil {
		return fmt.Errorf("cfgeconomycore: failed: %w", err)
	}
	return printStatuses(statuses, stdout)
}

func printStatuses(statuses []economyconfig.FileStatus, stdout io.Writer) error {
	allOK := true
	for _, status := range statuses {
		if status.Err != nil {
			allOK = false
			fmt.Fprintf(stdout, "%s %s failed: %v\n", status.Kind, status.Path, status.Err)
			printWarnings(stdout, status)
			continue
		}
		if status.TypeCount > 0 || status.Kind == "base-types" || status.Kind == "types" {
			fmt.Fprintf(stdout, "%s %s ok (%d types)\n", status.Kind, status.Path, status.TypeCount)
		} else {
			fmt.Fprintf(stdout, "%s %s ok\n", status.Kind, status.Path)
		}
		printWarnings(stdout, status)
	}
	if !allOK {
		return validation.ErrFailed
	}
	return nil
}

func printWarnings(stdout io.Writer, status economyconfig.FileStatus) {
	for _, warning := range status.Warnings {
		fmt.Fprintf(stdout, "%s %s warning: %s\n", status.Kind, status.Path, warning)
	}
}
