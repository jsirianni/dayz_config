//go:build windows

package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestUpgradeDebugEventABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("legacy handoff supports Windows amd64 only")
	}
	if unsafe.Offsetof(upgradeDebugEvent{}.Data) != 16 || unsafe.Sizeof(upgradeDebugEvent{}) != 176 {
		t.Fatal("DEBUG_EVENT does not match the Windows amd64 SDK layout")
	}
}

func TestUpgradeDebugRequiresCompleteDistinctIdentities(t *testing.T) {
	for _, identities := range [][2]ProcessIdentity{
		{},
		{{PID: 1, Token: "x", Executable: "manager"}, {PID: 1, Token: "x", Executable: "game"}},
		{{PID: 1, Executable: "manager"}, {PID: 2, Token: "y", Executable: "game"}},
	} {
		if frozen, err := freezeUpgradeManager(context.Background(), identities[0], identities[1]); err == nil || frozen != nil {
			t.Fatalf("invalid identities accepted: %+v", identities)
		}
	}
}

// This fixture is deliberately self-contained: the manager and its child are
// copies of the test program, never any enrolled service or game installation.
func TestUpgradeDebugFixture(t *testing.T) {
	role := os.Getenv("DZDEPLOY_DEBUG_FIXTURE_ROLE")
	if role == "" {
		return
	}
	dir := os.Getenv("DZDEPLOY_DEBUG_FIXTURE_DIR")
	if role == "debugger" || role == "debugger-armed" {
		var identities [2]ProcessIdentity
		if ReadJSON(filepath.Join(dir, "identities.json"), &identities) != nil {
			os.Exit(6)
		}
		frozen, err := freezeUpgradeManagerPrepared(context.Background(), identities[0], identities[1], func() (func(), error) { return func() {}, nil }, debugFixtureOwnedOpen(identities[0], identities[1]))
		if err != nil || frozen == nil {
			os.Exit(7)
		}
		if role == "debugger-armed" {
			if err = frozen.ArmCrashRetirement(); err != nil {
				os.Exit(9)
			}
		}
		if os.WriteFile(filepath.Join(dir, "debugger.ready"), []byte("ready"), 0600) != nil {
			os.Exit(8)
		}
		// Intentionally no Close: the parent forcibly exits this sacrificial
		// debugger to verify DebugSetProcessKillOnExit(false).
	}
	if role == "manager" {
		child := exec.Command(os.Args[0], "-test.run=^TestUpgradeDebugFixture$")
		child.Env = append(os.Environ(), "DZDEPLOY_DEBUG_FIXTURE_ROLE=game")
		child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.pid"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			os.Exit(4)
		}
	}
	for {
		f, err := os.OpenFile(filepath.Join(dir, role+".tick"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(5)
		}
		_, err = fmt.Fprintln(f, time.Now().UnixNano())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			os.Exit(5)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestUpgradeLegacyDebuggerPreservesChild(t *testing.T) {
	if os.Getenv("DZDEPLOY_UPGRADE_DEBUG_INTEGRATION") != "1" {
		t.Skip("set DZDEPLOY_UPGRADE_DEBUG_INTEGRATION=1 on a disposable elevated Windows runner")
	}
	for _, action := range []string{"resume", "retire", "child-exit", "debugger-crash", "debugger-crash-armed"} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			manager := exec.Command(os.Args[0], "-test.run=^TestUpgradeDebugFixture$")
			manager.Env = append(os.Environ(), "DZDEPLOY_DEBUG_FIXTURE_ROLE=manager", "DZDEPLOY_DEBUG_FIXTURE_DIR="+dir)
			// Keep this fixture outside any test-runner job, just like an SCM
			// service. Unsupported breakaway is a failure on the explicit runner.
			manager.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_BREAKAWAY_FROM_JOB}
			if err := manager.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = manager.Process.Kill(); _ = manager.Wait() }()
			var childPID int
			waitDebugFixture(t, func() bool {
				raw, err := os.ReadFile(filepath.Join(dir, "game.pid"))
				if err != nil {
					return false
				}
				childPID, err = strconv.Atoi(string(raw))
				return err == nil && childPID > 0
			})
			child, err := os.FindProcess(childPID)
			if err != nil {
				t.Fatal(err)
			}
			defer child.Kill()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			managerToken, err := processToken(manager.Process.Pid, exe)
			if err != nil {
				t.Fatal(err)
			}
			childToken, err := processToken(childPID, exe)
			if err != nil {
				t.Fatal(err)
			}
			// Publishing the child PID precedes the manager's first pulse. Do
			// not freeze before there is an observable baseline to compare.
			_ = debugFixtureTick(t, filepath.Join(dir, "manager.tick"))
			_ = debugFixtureTick(t, filepath.Join(dir, "game.tick"))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			managerIdentity := ProcessIdentity{PID: manager.Process.Pid, Token: managerToken, Executable: exe}
			gameIdentity := ProcessIdentity{PID: childPID, Token: childToken, Executable: exe}
			if action == "debugger-crash" || action == "debugger-crash-armed" {
				if err = WriteJSON(filepath.Join(dir, "identities.json"), [2]ProcessIdentity{managerIdentity, gameIdentity}); err != nil {
					t.Fatal(err)
				}
				debugger := exec.Command(os.Args[0], "-test.run=^TestUpgradeDebugFixture$")
				role := "debugger"
				if action == "debugger-crash-armed" {
					role = "debugger-armed"
				}
				debugger.Env = append(os.Environ(), "DZDEPLOY_DEBUG_FIXTURE_ROLE="+role, "DZDEPLOY_DEBUG_FIXTURE_DIR="+dir)
				debugger.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
				if err = debugger.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = debugger.Process.Kill(); _ = debugger.Wait() }()
				waitDebugFixture(t, func() bool { _, e := os.Stat(filepath.Join(dir, "debugger.ready")); return e == nil })
				managerTick := debugFixtureTick(t, filepath.Join(dir, "manager.tick"))
				childTick := debugFixtureTick(t, filepath.Join(dir, "game.tick"))
				time.Sleep(180 * time.Millisecond)
				if debugFixtureTick(t, filepath.Join(dir, "manager.tick")) != managerTick || debugFixtureTick(t, filepath.Join(dir, "game.tick")) <= childTick {
					t.Fatal("sacrificial debugger did not isolate the manager")
				}
				if err = debugger.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				if action == "debugger-crash-armed" {
					waitDebugFixture(t, func() bool { _, e := processToken(manager.Process.Pid, exe); return e != nil })
				} else {
					waitDebugFixture(t, func() bool { return debugFixtureTick(t, filepath.Join(dir, "manager.tick")) > managerTick })
				}
				if token, e := processToken(childPID, exe); e != nil || token != childToken {
					t.Fatalf("debugger exit killed or replaced child: %s, %v", token, e)
				}
				waitDebugFixture(t, func() bool { return debugFixtureTick(t, filepath.Join(dir, "game.tick")) > childTick })
				return
			}
			// The real debug event engine can operate on our own disposable
			// processes without SeDebugPrivilege. Production still requires it.
			privilege := func() (func(), error) { return func() {}, nil }
			frozen, err := freezeUpgradeManagerPrepared(ctx, managerIdentity, gameIdentity, privilege, upgradeDebugOpen)
			if err != nil && strings.Contains(err.Error(), "job membership blocks") {
				t.Log("production preflight correctly rejected test-runner job membership; exercising debugger engine with owned fixture handles")
				// Sandboxes may assign an unbreakable outer job. The test owns
				// that job's lifetime, so retiring this fixture manager cannot
				// close its last handle. This seam exercises actual Windows
				// debugger APIs while leaving the production job guard intact.
				frozen, err = freezeUpgradeManagerPrepared(ctx, managerIdentity, gameIdentity, privilege, debugFixtureOwnedOpen(managerIdentity, gameIdentity))
			}
			if err != nil {
				t.Fatal(err)
			}
			defer frozen.Close()
			managerTick := debugFixtureTick(t, filepath.Join(dir, "manager.tick"))
			childTick := debugFixtureTick(t, filepath.Join(dir, "game.tick"))
			time.Sleep(180 * time.Millisecond)
			if got := debugFixtureTick(t, filepath.Join(dir, "manager.tick")); got != managerTick {
				t.Fatal("frozen manager continued executing")
			}
			if got := debugFixtureTick(t, filepath.Join(dir, "game.tick")); got <= childTick {
				t.Fatal("debugger suspended the child")
			}
			switch action {
			case "resume":
				if err = frozen.Resume(); err != nil {
					t.Fatal(err)
				}
				waitDebugFixture(t, func() bool { return debugFixtureTick(t, filepath.Join(dir, "manager.tick")) > managerTick })
			case "retire":
				if err = frozen.Retire(); err != nil {
					t.Fatal(err)
				}
			case "child-exit":
				if err = child.Kill(); err != nil {
					t.Fatal(err)
				}
				waitDebugFixture(t, func() bool { _, e := processToken(childPID, exe); return e != nil })
				if err = frozen.Resume(); err == nil || !strings.Contains(err.Error(), "refusing to resume") {
					t.Fatalf("manager resumed after independent child exit: %v", err)
				}
				if err = frozen.Close(); err == nil || !strings.Contains(err.Error(), "recovery must remain paused") {
					t.Fatalf("child exit not reported by safe cleanup: %v", err)
				}
				return
			}
			if token, err := processToken(childPID, exe); err != nil || token != childToken {
				t.Fatalf("child continuity lost: %s, %v", token, err)
			}
		})
	}
}

func debugFixtureOwnedOpen(manager, game ProcessIdentity) func(ProcessIdentity, uint32) (windows.Handle, error) {
	return func(identity ProcessIdentity, access uint32) (windows.Handle, error) {
		if identity != manager && identity != game {
			return 0, fmt.Errorf("unexpected fixture identity")
		}
		h, err := windows.OpenProcess(access|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(identity.PID))
		if err == nil {
			err = upgradeDebugIdentity(h, identity)
			if err != nil {
				windows.CloseHandle(h)
			}
		}
		return h, err
	}
}

func waitDebugFixture(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for isolated debugger fixture")
}

func debugFixtureTick(t *testing.T, path string) int64 {
	t.Helper()
	var tick int64
	waitDebugFixture(t, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines := strings.Split(string(raw), "\n")
		// Ignore a last partial record if the manager was frozen mid-write.
		for i := len(lines) - 2; i >= 0; i-- {
			tick, err = strconv.ParseInt(lines[i], 10, 64)
			if err == nil && tick > 0 {
				return true
			}
		}
		return false
	})
	if tick <= 0 {
		t.Fatal(fmt.Sprintf("invalid fixture tick %d", tick))
	}
	return tick
}
