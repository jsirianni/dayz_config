// Package cli owns command-line parsing, mode selection, and process exit codes.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/notifier"
)

const (
	exitFailure    = 1
	exitUsage      = 64
	exitPermission = 77
)

type command struct {
	version string
	stdout  io.Writer
	stderr  io.Writer
}

// Run executes one invocation without global flags or process exits. It returns
// 64 for usage/configuration errors, 77 for root execution, and 1 for runtime
// failures. Help, version, offline validation, and successful execution return 0.
func Run(ctx context.Context, args []string, version string, stdout, stderr io.Writer) int {
	cmd := command{version: version, stdout: stdout, stderr: stderr}
	return cmd.run(ctx, args, os.Geteuid(), execute)
}

func (c command) run(ctx context.Context, args []string, uid int, execute executeFunc) int {
	opts, err := parseFlags(args, c.stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return exitUsage
	}
	if opts.mode == modeVersion {
		if _, err := fmt.Fprintln(c.stdout, c.version); err != nil {
			fmt.Fprintf(c.stderr, "write version: %v\n", err)
			return exitFailure
		}
		return 0
	}
	log := slog.New(slog.NewJSONHandler(c.stdout, nil))
	config, err := notifier.Load(opts.configPath, opts.explicitConfig, os.LookupEnv)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		return exitUsage
	}
	return c.dispatch(ctx, opts.mode, config, uid, log, execute)
}

func (c command) dispatch(ctx context.Context, mode mode, config notifier.Config, uid int, log *slog.Logger, execute executeFunc) int {
	if mode == modeCheckConfig {
		log.Info("configuration valid", "endpoints", len(config.Endpoints))
		return 0
	}
	if uid == 0 {
		log.Error("run dzsa-notifier as an unprivileged user")
		return exitPermission
	}
	if err := execute(ctx, mode, config, c.version, log); err != nil {
		log.Error("notifier failed", "error", err)
		return exitFailure
	}
	return 0
}
