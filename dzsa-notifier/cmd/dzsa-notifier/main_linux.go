package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/notifier"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var version = "dev"

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	path := flag.String("config", notifier.DefaultConfig, "YAML configuration path")
	check := flag.Bool("check-config", false, "validate configuration without network requests")
	once := flag.Bool("once", false, "notify once; exit nonzero on any failure or deferral")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return 0
	}
	if flag.NArg() != 0 || (*check && *once) {
		log.Error("invalid command arguments")
		return 64
	}
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicit = true
		}
	})
	c, err := notifier.Load(*path, explicit, os.LookupEnv)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		return 64
	}
	if *check {
		log.Info("configuration valid", "endpoints", len(c.Endpoints))
		return 0
	}
	if os.Geteuid() == 0 {
		log.Error("run dzsa-notifier as an unprivileged user")
		return 77
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runner := notifier.NewRunner(c, notifier.NewClient(c.RequestTimeout, version), log)
	if *once {
		if err := runner.Cycle(ctx); err != nil {
			return 1
		}
		return 0
	}
	listener, err := net.Listen("tcp", c.MetricsListen)
	if err != nil {
		log.Error("metrics listen failed", "error", err)
		return 1
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(runner.Registry, promhttp.HandlerOpts{}))
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		log.Error("systemd readiness notification failed", "error", err)
		_ = server.Close()
		return 1
	}
	log.Info("notifier started", "version", version, "interval", c.Interval.String(), "metrics_listen", listener.Addr().String())
	done := make(chan struct{})
	go func() { defer close(done); runner.Run(ctx) }()
	code := 0
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server failed", "error", err)
			code = 1
		}
		stop()
	}
	stop()
	<-done
	_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
	}
	log.Info("notifier stopped")
	return code
}
