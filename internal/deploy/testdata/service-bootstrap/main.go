//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func main() {
	if len(os.Args) == 4 && os.Args[1] == "agent" && os.Args[2] == "--host-config" {
		c, err := deploy.LoadHost(os.Args[3])
		if err == nil {
			err = deploy.RunService(c, false)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || len(os.Args) != 2 {
		panic("service integration helper requires a disposable GitHub-hosted runner")
	}
	c, err := deploy.LoadHost(os.Args[1])
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, err = deploy.Bootstrap(ctx, deploy.BootstrapOptions{Config: c, Virtual: true})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
