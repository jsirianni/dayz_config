package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/cli"
)

var version = "dev"

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], version, os.Stdout, os.Stderr)
}
