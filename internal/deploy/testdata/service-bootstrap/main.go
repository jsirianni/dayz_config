//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func main() {
	if len(os.Args) == 4 && os.Args[1] == "agent" && os.Args[2] == "--host-config" {
		log := filepath.Join(filepath.Dir(os.Args[3]), "logs", "agent-service-entry.txt")
		_ = os.WriteFile(log, []byte("service entry reached"), 0600)
		c, err := deploy.LoadHost(os.Args[3])
		if err == nil {
			err = deploy.RunService(c, false)
		}
		if err != nil {
			_ = os.WriteFile(log, []byte(err.Error()), 0600)
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
