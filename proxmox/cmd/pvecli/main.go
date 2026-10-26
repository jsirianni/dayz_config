package main

import (
	"context"
	"github.com/jsirianni/dayz_config/proxmox/internal/cli"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	app := cli.App{}
	os.Exit(app.Execute(ctx, os.Args[1:]))
}
