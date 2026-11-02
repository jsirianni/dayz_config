//go:build windows

package deploy

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAdministrativeCommandPreservesErrorOutput(t *testing.T) {
	t.Setenv("DZDEPLOY_CAPTURE_HELPER", "fail")
	cfg := HostConfig{StateDir: t.TempDir()}
	err := adminCommand(context.Background(), cfg, os.Args[0], "-test.run=^TestCaptureHelper$")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 || !strings.Contains(err.Error(), "final stdout") || !strings.Contains(err.Error(), "final stderr") {
		t.Fatal(err)
	}
}

func TestLogLeaseAllowsProvisioningACLReset(t *testing.T) {
	cfg := HostConfig{StateDir: t.TempDir()}
	if err := protectPrivateDirectory(cfg.StateDir); err != nil {
		t.Fatal(err)
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs, _ := processLogSession(context.Background(), cfg)
	defer logs.Close()
	// Both icacls commands run while the shared output file and writer lease are
	// open, exactly as during enrollment. Only this disposable directory changes.
	if err := protectServiceTree(ctx, cfg, cfg.StateDir, user.User.Sid.String()); err != nil {
		t.Fatal(err)
	}
}
