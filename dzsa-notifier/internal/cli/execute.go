package cli

import (
	"context"
	"log/slog"

	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/notifier"
	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/service"
)

type executeFunc func(context.Context, mode, notifier.Config, string, *slog.Logger) error

func execute(ctx context.Context, mode mode, config notifier.Config, version string, log *slog.Logger) error {
	runner := notifier.NewRunner(config, notifier.NewClient(config.RequestTimeout, version), log)
	if mode == modeOnce {
		return runner.Cycle(ctx)
	}
	serviceLog := log.With("version", version, "interval", config.Interval.String())
	return service.New(config.MetricsListen, runner.Registry, runner.Run, serviceLog).Run(ctx)
}
