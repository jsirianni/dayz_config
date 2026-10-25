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
)

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
