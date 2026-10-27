package deploycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
	"github.com/jsirianni/dayz_config/internal/logreader"
)

func runLogs(args []string) error {
	if len(args) < 2 {
		return errors.New("logs/diagnose requires a target")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	config := f.String("config", "", "client configuration")
	repo := f.String("repo", ".", "repository root")
	kind := f.String("kind", "console", "log kind")
	lines := f.Int("tail", 200, "last lines (1..10000)")
	follow := f.Bool("follow", false, "follow rotation")
	since := f.Duration("since", 24*time.Hour, "diagnostic history duration")
	output := f.String("output", filepath.Join(".deploy", "diagnostics"), "local diagnostic directory")
	asJSON := f.Bool("json", false, "JSON output")
	timeout := f.Duration("timeout", 5*time.Minute, "command timeout")
	if err := f.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 || *lines < 1 || *lines > 10000 || *since <= 0 || *timeout <= 0 {
		return errors.New("invalid log limits or unexpected arguments")
	}
	if !deploy.ValidLogKind(*kind) {
		return errors.New("invalid log kind")
	}
	if args[0] == "logs" && (args[1] == "all" || *asJSON) {
		return errors.New("logs requires one target and emits redacted text")
	}
	if args[0] == "diagnose" && *follow {
		return errors.New("diagnose cannot follow logs")
	}
	if *config == "" {
		*config = filepath.Join(*repo, "deploy.yaml")
	}
	cfg, err := deploy.LoadClient(*config)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if !*follow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	if args[0] == "logs" {
		if _, err = deploy.TargetFor(args[1]); err != nil {
			return err
		}
		r, e := deploy.Connect(ctx, cfg.Targets[args[1]])
		if e != nil {
			return e
		}
		defer r.Close()
		var host deploy.HostConfig
		if e = r.ReadJSON("host.json", &host); e != nil {
			return e
		}
		if e = host.Validate(); e != nil {
			return e
		}
		if host.Target != args[1] {
			return errors.New("host target mismatch")
		}
		e = r.Logs(ctx, host, *kind, *lines, *follow, os.Stdout)
		if ctx.Err() == context.Canceled {
			return nil
		}
		return e
	}
	names := []string{args[1]}
	if args[1] == "all" {
		names = nil
		targets, _ := deploy.Targets(nil)
		for name := range targets {
			names = append(names, name)
		}
		sort.Strings(names)
	} else {
		if _, err = deploy.TargetFor(args[1]); err != nil {
			return err
		}
	}
	type report struct {
		Target      string              `json:"target"`
		Bundle      string              `json:"bundle,omitempty"`
		Diagnostics *deploy.Diagnostics `json:"diagnostics,omitempty"`
		Error       string              `json:"error,omitempty"`
	}
	var reports []report
	failed := false
	for _, name := range names {
		entry := report{Target: name}
		r, e := deploy.Connect(ctx, cfg.Targets[name])
		if e == nil {
			result, collectErr := r.Diagnose(ctx, name, time.Now().UTC().Add(-*since), *output)
			r.Close()
			e = collectErr
			for i := range result.Logs {
				result.Logs[i].Text = ""
			}
			entry.Bundle = result.Bundle
			entry.Diagnostics = &result
		}
		if e != nil {
			entry.Error = logreader.CleanText(e.Error())
			failed = true
		}
		reports = append(reports, entry)
	}
	if *asJSON {
		if err = emit(reports, true); err != nil {
			return err
		}
	} else {
		for _, r := range reports {
			if r.Error != "" {
				fmt.Printf("%s ERROR: %s\n", r.Target, r.Error)
			} else {
				fmt.Printf("%s: %s (%d issue groups)\n", r.Target, r.Bundle, len(r.Diagnostics.Issues))
				for _, warning := range r.Diagnostics.Warnings {
					fmt.Println("  WARNING:", warning)
				}
			}
		}
	}
	if failed {
		return reported{errors.New("some diagnostic sources could not be collected")}
	}
	return nil
}
