//go:build linux

package deploy

import (
	"context"
	"errors"
)

func ServiceName(alias string) string { return "dzdeploy-" + alias }

type BootstrapOptions struct {
	Config        HostConfig
	Account       string
	PasswordEnv   string
	PublicKeyFile string
	SSHSources    string
	SSHPort       int
	InstallSSH    bool
	Virtual       bool
}

func Bootstrap(context.Context, BootstrapOptions) (map[string]string, error) {
	return nil, errors.New("host bootstrap is Windows-only; copy the Windows build onto the host and run elevated")
}
func RunService(HostConfig, bool) error {
	return errors.New("host agent is Windows-only; the client supports Windows and Linux")
}
func ensureNoGame(string) error { return nil }
