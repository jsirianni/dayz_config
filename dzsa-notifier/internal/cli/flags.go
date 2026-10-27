package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/notifier"
)

type mode uint8

const (
	modeDaemon mode = iota
	modeOnce
	modeCheckConfig
	modeVersion
)

type options struct {
	configPath     string
	explicitConfig bool
	mode           mode
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	var once, check, version bool
	flags := flag.NewFlagSet("dzsa-notifier", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.configPath, "config", notifier.DefaultConfig, "YAML configuration path")
	flags.BoolVar(&check, "check-config", false, "validate configuration without network requests")
	flags.BoolVar(&once, "once", false, "notify once; exit nonzero on any failure or deferral")
	flags.BoolVar(&version, "version", false, "print version")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() != 0 {
		return opts, usageError(flags, stderr, "unexpected positional arguments")
	}
	selected := 0
	for _, selection := range []struct {
		enabled bool
		mode    mode
	}{{once, modeOnce}, {check, modeCheckConfig}, {version, modeVersion}} {
		if selection.enabled {
			selected++
			opts.mode = selection.mode
		}
	}
	if selected > 1 {
		return opts, usageError(flags, stderr, "--once, --check-config, and --version are mutually exclusive")
	}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			opts.explicitConfig = true
		}
	})
	return opts, nil
}

func usageError(flags *flag.FlagSet, stderr io.Writer, message string) error {
	err := errors.New(message)
	fmt.Fprintln(stderr, err)
	flags.Usage()
	return err
}
