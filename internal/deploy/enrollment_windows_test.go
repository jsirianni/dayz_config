//go:build windows

package deploy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestVirtualServiceBootstrapOnDisposableRunner(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("creates a real Windows service only on disposable GitHub-hosted runners")
	}
	c := testConfig(t)
	m, err := mgr.Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Disconnect() })
	name := ServiceName(c.Target)
	if old, err := m.OpenService(name); err == nil {
		old.Close()
		t.Fatal("refusing to touch an existing service", name)
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s, err := m.OpenService(name)
		if err != nil {
			return
		}
		defer s.Close()
		_, _ = s.Control(svc.Stop)
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			status, err := s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err := s.Delete(); err != nil {
			t.Error(err)
		}
	})
	// Build a real service entry point, so SCM starts the agent rather than a
	// test binary. It never installs SteamCMD or launches the game.
	helper := filepath.Join(t.TempDir(), "service helper.exe")
	if out, err := exec.Command("go", "build", "-o", helper, "./testdata/service-bootstrap").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	config := filepath.Join(t.TempDir(), "host.json")
	if err := WriteJSON(config, c); err != nil {
		t.Fatal(err)
	}
	// Fail after the service is registered and permissions are applied. The
	// service must be removed, and retry must recover from the retained files.
	if err := os.MkdirAll(c.InstallDir, 0700); err != nil {
		t.Fatal(err)
	}
	obstruction := filepath.Join(c.InstallDir, "battleye")
	if err := os.WriteFile(obstruction, []byte("block directory creation"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(helper, config).CombinedOutput(); err == nil {
		t.Fatal("bootstrap unexpectedly succeeded with obstructed RCon directory", string(out))
	} else if strings.Contains(string(out), "No mapping") {
		t.Fatal("virtual account still could not be resolved", string(out))
	} else {
		t.Logf("Expected first-attempt failure: %s", out)
	}
	if s, err := m.OpenService(name); err == nil {
		s.Close()
		t.Fatal("failed bootstrap left a registered service")
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		t.Fatal(err)
	}
	if err := os.Remove(obstruction); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(helper, config).CombinedOutput(); err != nil {
		for _, p := range []string{c.StateDir, filepath.Join(c.StateDir, "state.json")} {
			acl, _ := exec.Command("icacls.exe", p).CombinedOutput()
			t.Logf("ACL for %s: %s", p, acl)
		}
		t.Fatal("fresh virtual-service bootstrap failed", err, string(out))
	}
	s, err := m.OpenService(name)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	settings, err := s.Config()
	if err != nil || settings.StartType != mgr.StartAutomatic || !settings.DelayedAutoStart || settings.ServiceStartName != `NT SERVICE\`+name {
		t.Fatal("wrong service settings", settings, err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var heartbeat struct{ Version int }
		if err := ReadJSON(filepath.Join(c.StateDir, "heartbeat.json"), &heartbeat); err == nil && heartbeat.Version == Protocol {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status, err := s.Query(); err == nil {
		t.Logf("Service status: %+v", status)
	}
	for _, name := range []string{"agent-error.txt", "agent-service-entry.txt"} {
		if data, err := os.ReadFile(filepath.Join(c.StateDir, "logs", name)); err == nil {
			t.Logf("%s: %s", name, data)
		}
	}
	t.Fatal("virtual account service did not publish its heartbeat")
}

func TestServiceAuthenticationPipeRelaysChallengesWithoutLeakingSecrets(t *testing.T) {
	t.Setenv("DZDEPLOY_STEAM_TEST_HELPER", "1")
	c := testConfig(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = atomicCopyContext(context.Background(), self, c.SteamCMD, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stop, err := startAuthBroker(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	conn, err := winio.DialPipeContext(ctx, authPipe(c))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var wire bytes.Buffer
	err = authConversation(io.TeeReader(conn, &wire), conn, func(f AuthFrame) (string, error) {
		if f.Type == "password" {
			second, err := winio.DialPipeContext(ctx, authPipe(c))
			if err != nil {
				return "", err
			}
			defer second.Close()
			if err = authConversation(second, second, nil); err == nil {
				return "", errors.New("concurrent authentication was accepted")
			}
			return "private-password", nil
		}
		if f.Type == "guard" {
			return "private-guard-code", nil
		}
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(wire.String(), "private-password") || strings.Contains(wire.String(), "private-guard-code") {
		t.Fatal("secret leaked into authentication output")
	}
}

func TestVirtualServiceSIDMatchesWindows(t *testing.T) {
	name := "dzdeploy-test-uninstalled"
	sid, err := virtualServiceSID(name)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sc.exe", "showsid", name).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if !strings.Contains(string(out), sid.String()) {
		t.Fatalf("computed SID does not match Windows: %s, %s", sid, out)
	}
}

func TestWindowsEnrollmentCommandHandlesSpaces(t *testing.T) {
	t.Setenv("DZDEPLOY_ARGS_TEST_HELPER", "1")
	root := filepath.Join(t.TempDir(), "directory with spaces")
	exe := filepath.Join(root, "agent.exe")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = atomicCopyContext(context.Background(), self, exe, 0700); err != nil {
		t.Fatal(err)
	}
	request := filepath.Join(root, "request with spaces.json")
	cmd := exec.Command("cmd.exe")
	prepareCommand(cmd)
	cmd.SysProcAttr.CmdLine = windowsRemoteCommand(exe, "enroll", "--request", request)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "enroll|--request|"+request {
		t.Fatalf("remote Windows quoting failed: %v %s", err, out)
	}
}

func TestUnsignedInstallerIsRejected(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyPublisher(self, "Microsoft Corporation"); err == nil {
		t.Fatal("unsigned executable accepted as Microsoft")
	}
}

func TestFirstEnrollmentUploadsAndExecutesVerifiedRequest(t *testing.T) {
	var calls atomic.Int32
	endpoint, root := sshExecFixture(t, func(command string, in io.Reader, out io.Writer) error {
		match := regexp.MustCompile(`^cmd.exe /d /s /c ""([^"]+)" enroll --request "([^"]+)""$`).FindStringSubmatch(command)
		if len(match) != 3 {
			t.Error("invalid remote command", command)
			return errors.New("invalid command")
		}
		var req EnrollmentRequest
		if err := ReadJSON(match[2], &req); err != nil {
			t.Error(err)
			return err
		}
		raw, err := os.ReadFile(match[1])
		if err != nil {
			t.Error(err)
			return err
		}
		if Digest(raw) != req.BinarySHA256 || req.Version != Protocol {
			t.Error("uploaded agent/request mismatch")
			return errors.New("hash mismatch")
		}
		if err = WriteJSON(enrollmentPath(req.Config), Enrollment{Version: Protocol, Target: req.Config.Target, ID: req.ID, Phase: "installing"}); err != nil {
			return err
		}
		if calls.Add(1) == 1 {
			return errors.New("simulate dropped SSH setup session")
		}
		if err = WriteJSON(filepath.Join(req.Config.StateDir, "state.json"), State{Version: Protocol, Target: req.Config.Target, Paused: true}); err != nil {
			return err
		}
		if err = WriteJSON(filepath.Join(req.Config.StateDir, "host.json"), req.Config); err != nil {
			return err
		}
		return WriteJSON(enrollmentPath(req.Config), Enrollment{Version: Protocol, Target: req.Config.Target, ID: req.ID, Phase: "service-ready"})
	})
	for _, p := range []string{"state.json", "heartbeat.json"} {
		if err := os.Remove(filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}
	c := testConfig(t)
	c.StateDir = root
	c.RConPasswordFile = filepath.Join(root, "secrets", "rcon.txt")
	endpoint.Host = &c
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b := testBundle(t, nil)
	if err = r.Enroll(context.Background(), b, io.Discard); err == nil {
		t.Fatal("interrupted enrollment reported success")
	}
	if needed, err := r.EnrollmentNeeded(c.Target); err != nil || !needed {
		t.Fatal("interrupted setup not resumable", needed, err)
	}
	if err = r.Enroll(context.Background(), b, io.Discard); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("enrollment not retried")
	}
}

// Optional read-only verification of real vendor downloads. Never executes an
// installer, creates a service, or alters the host; normal CI remains offline.
func TestOfficialInstallerSignatures(t *testing.T) {
	if os.Getenv("DZDEPLOY_CHECK_VENDOR_DOWNLOADS") != "1" {
		t.Skip("explicit network/download check only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	raw, err := downloadOfficial(ctx, steamInstallerURL, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := steamExecutable(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "steamcmd.exe")
	if err = AtomicWrite(p, exe, 0700); err != nil {
		t.Fatal(err)
	}
	if err = verifyPublisher(p, "Valve", "Valve Corp.", "Valve Corporation"); err != nil {
		t.Fatal(err)
	}
	raw, err = downloadOfficial(ctx, "https://aka.ms/vs/17/release/vc_redist.x64.exe", 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	p = filepath.Join(t.TempDir(), "vc_redist.x64.exe")
	if err = AtomicWrite(p, raw, 0700); err != nil {
		t.Fatal(err)
	}
	if err = verifyPublisher(p, "Microsoft Corporation"); err != nil {
		t.Fatal(err)
	}
}
