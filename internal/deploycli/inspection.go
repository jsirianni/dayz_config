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
)

func runInspection(args []string) error {
	if len(args) < 2 {
		return errors.New("health/history requires a server alias or all")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	since := f.Duration("since", 24*time.Hour, "history duration")
	timeout := f.Duration("timeout", 60*time.Second, "fleet inspection timeout")
	repo := f.String("repo", ".", "repository root")
	config := f.String("config", "", "client config")
	asJSON := f.Bool("json", false, "JSON report")
	if err := f.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 || *since <= 0 || *timeout <= 0 {
		return errors.New("unexpected arguments or nonpositive duration")
	}
	if *config == "" {
		*config = filepath.Join(*repo, "deploy.yaml")
	}
	cfg, err := deploy.LoadClient(*config)
	if err != nil {
		return err
	}
	targets := map[string]deploy.Endpoint{}
	if args[1] == "all" {
		all, _ := deploy.Targets(nil)
		for name := range all {
			targets[name] = cfg.Targets[name]
		}
	} else {
		if _, err := deploy.TargetFor(args[1]); err != nil {
			return err
		}
		targets[args[1]] = cfg.Targets[args[1]]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	start := time.Now().UTC().Add(-*since)
	if args[0] == "history" {
		return printHistory(ctx, targets, start, *asJSON)
	}
	reports := deploy.Fleet(ctx, targets, start)
	if *asJSON {
		if err := emit(reports, true); err != nil {
			return err
		}
	} else {
		if err := renderHealth(os.Stdout, reports); err != nil {
			return err
		}
	}
	for _, r := range reports {
		if len(r.Errors) > 0 {
			return reported{errors.New("one or more servers failed health checks")}
		}
	}
	return nil
}

func printHistory(ctx context.Context, targets map[string]deploy.Endpoint, since time.Time, asJSON bool) error {
	type item struct {
		Target  string               `json:"target"`
		History deploy.HistoryReport `json:"history"`
		Error   string               `json:"error,omitempty"`
	}
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	reports := make([]item, 0, len(names))
	failed := false
	for _, name := range names {
		entry := item{Target: name}
		r, err := deploy.Connect(ctx, targets[name])
		if err == nil {
			var host deploy.HostConfig
			err = r.ReadJSON("host.json", &host)
			if err == nil {
				err = host.Validate()
			}
			if err == nil && host.Target != name {
				err = errors.New("host target mismatch")
			}
			if err == nil {
				entry.History = r.History(name, host, since)
			}
			r.Close()
		}
		if err != nil {
			entry.Error = err.Error()
			failed = true
		}
		reports = append(reports, entry)
	}
	if asJSON {
		if err := emit(reports, true); err != nil {
			return err
		}
	} else {
		for _, r := range reports {
			fmt.Println(r.Target)
			if r.Error != "" {
				fmt.Println("  ERROR:", r.Error)
				continue
			}
			if r.History.Incomplete {
				fmt.Println("  Coverage incomplete: missing events do not prove an absence of restarts")
			}
			for _, cycle := range r.History.Cycles {
				fmt.Printf("  Confirmed %s relaunch: shutdown=%s ready=%s operation=%s\n", cycle.Cause, cycle.Shutdown.Format(time.RFC3339), cycle.Ready.Format(time.RFC3339), cycle.Operation)
			}
			for _, e := range r.History.Events {
				fmt.Printf("  %s %-18s cause=%s operation=%s source=%s\n", e.At.Format(time.RFC3339), e.Kind, e.Cause, e.Operation, e.Source)
			}
		}
	}
	if failed {
		return reported{errors.New("one or more history sources were unavailable")}
	}
	return nil
}
