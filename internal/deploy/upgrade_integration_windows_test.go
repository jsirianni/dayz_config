//go:build windows

package deploy

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestAgentUpgradeServiceIntegration(t *testing.T) {
	if os.Getenv("DZDEPLOY_UPGRADE_INTEGRATION") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("requires disposable Windows CI runner")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Fatal("disposable runner must be elevated")
	}
	programData := filepath.Clean(os.Getenv("ProgramData"))
	root, err := os.MkdirTemp(programData, "dzdeploy-upgrade-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(root) != programData || !strings.HasPrefix(filepath.Base(root), "dzdeploy-upgrade-integration-") {
			t.Error("unsafe fixture cleanup")
			return
		}
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source")
	if err = os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		raw, e := os.ReadFile(filepath.Join(repo, name))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(source, name), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"internal", filepath.Join("cmd", "dzdeploy")} {
		if err = os.CopyFS(filepath.Join(source, name), os.DirFS(filepath.Join(repo, name))); err != nil {
			t.Fatal(err)
		}
	}
	run := func(directory string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = directory
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("fixture %s failed: %v\n%s", args[0], e, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(source, "git", "init")
	run(source, "git", "config", "user.name", "Disposable upgrade fixture")
	run(source, "git", "config", "user.email", "fixture@example.invalid")
	run(source, "git", "add", ".")
	run(source, "git", "commit", "-m", "fixture clean upgrade source")
	buildCandidate := func(name string) (string, AgentIdentity) {
		t.Helper()
		dest := filepath.Join(root, name)
		run(source, "go", "build", "-buildvcs=true", "-trimpath", "-o", dest, "./cmd/dzdeploy")
		identity, e := ReadAgentIdentity(dest)
		if e != nil {
			t.Fatal(e)
		}
		revision := run(source, "git", "rev-parse", "HEAD")
		if e = ValidateUpgradeCandidate(identity, revision); e != nil {
			t.Fatal(identity, e)
		}
		return dest, identity
	}
	candidate, candidateIdentity := buildCandidate("candidate.exe")
	if err = os.WriteFile(filepath.Join(source, "internal", "deploy", "upgrade_fixture_revision.go"), []byte("package deploy\nconst upgradeFixtureRevision = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(source, "git", "add", "internal/deploy/upgrade_fixture_revision.go")
	run(source, "git", "commit", "-m", "fixture successive cooperative upgrade")
	secondCandidate, secondIdentity := buildCandidate("second-candidate.exe")
	legacy := filepath.Join(root, "legacy.exe")
	run(repo, "go", "build", "-buildvcs=false", "-o", legacy, "./internal/deploy/testdata/upgrade-host")
	// The failure candidate retains the real coordinator but refuses its agent
	// startup. Its changed source is committed before building, preserving real
	// provenance checks instead of weakening them for the fixture.
	mainFile := filepath.Join(source, "cmd", "dzdeploy", "main.go")
	mainRaw, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	// Give the coordinator a deterministic interruption window after replacement
	// while the new manager has not yet connected to SCM or acknowledged a hold.
	slowMain := strings.Replace(string(mainRaw), "package main", "package main\nimport \"time\"", 1)
	slowMain = strings.Replace(slowMain, "func main() {", "func main() { if len(os.Args)>1 && os.Args[1]==\"agent\" { time.Sleep(20*time.Second) };", 1)
	if err = os.WriteFile(mainFile, []byte(slowMain), 0600); err != nil {
		t.Fatal(err)
	}
	run(source, "git", "add", "cmd/dzdeploy/main.go")
	run(source, "git", "commit", "-m", "fixture interrupted manager startup")
	slowCandidate, slowIdentity := buildCandidate("slow-candidate.exe")
	mainRaw = []byte(strings.Replace(string(mainRaw), "func main() {", "func main() { if len(os.Args)>1 && os.Args[1]==\"agent\" { os.Exit(7) };", 1))
	if err = os.WriteFile(mainFile, mainRaw, 0600); err != nil {
		t.Fatal(err)
	}
	run(source, "git", "add", "cmd/dzdeploy/main.go")
	run(source, "git", "commit", "-m", "fixture candidate startup failure")
	badCandidate, badIdentity := buildCandidate("bad-candidate.exe")
	for _, test := range []struct {
		name, candidate string
		identity        AgentIdentity
		fail            bool
		interrupt       bool
	}{{"legacy and cooperative continuity", candidate, candidateIdentity, false, false}, {"replacement startup rollback", badCandidate, badIdentity, true, false}, {"coordinator interruption recovery", slowCandidate, slowIdentity, true, true}} {
		t.Run(test.name, func(t *testing.T) {
			runAgentUpgradeFixture(t, filepath.Join(root, strings.ReplaceAll(test.name, " ", "-")), legacy, test.candidate, test.identity, test.fail, test.interrupt, secondCandidate, secondIdentity)
		})
	}
}

func runAgentUpgradeFixture(t *testing.T, root, legacy, candidate string, candidateIdentity AgentIdentity, wantFailure, interrupt bool, secondCandidate string, secondIdentity AgentIdentity) {
	t.Helper()
	c := testConfig(t)
	c.InstallDir = filepath.Join(root, "install")
	c.StateDir = filepath.Join(root, "state")
	c.SteamCMD = filepath.Join(root, "steam", "steamcmd.exe")
	c.WorkshopDir = filepath.Join(root, "workshop")
	c.RConPasswordFile = filepath.Join(c.StateDir, "secret")
	rcon, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c.RConAddress = rcon.LocalAddr().String()
	t.Cleanup(func() {
		defer rcon.Close()
		_ = rcon.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		packet := make([]byte, 4096)
		if n, _, e := rcon.ReadFrom(packet); e == nil || n != 0 {
			t.Error("agent upgrade attempted an RCon call")
		}
	})
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(c.StateDir, "bin"), filepath.Dir(c.SteamCMD), c.WorkshopDir} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err = WriteJSON(filepath.Join(c.StateDir, "host.json"), c); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(c.StateDir, "bin", "dzdeploy.exe")
	if err = AtomicCopy(legacy, installed, 0700); err != nil {
		t.Fatal(err)
	}
	if err = AtomicCopy(legacy, filepath.Join(c.InstallDir, "DayZServer_x64.exe"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = AtomicCopy(legacy, c.SteamCMD, 0700); err != nil {
		t.Fatal(err)
	}
	configurationPath := filepath.Join(c.InstallDir, "serverDZ.cfg")
	configuration := []byte("fixture configuration must remain byte-identical\n")
	if err = os.WriteFile(configurationPath, configuration, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if raw, e := os.ReadFile(configurationPath); e != nil || string(raw) != string(configuration) {
			t.Error("upgrade changed game configuration", e)
		}
		if _, e := os.Stat(filepath.Join(filepath.Dir(c.SteamCMD), "steam-called.txt")); !os.IsNotExist(e) {
			t.Error("upgrade executed SteamCMD", e)
		}
		if raw, e := os.ReadFile(filepath.Join(c.InstallDir, "game-launches.txt")); e != nil || strings.Count(string(raw), "launch\n") != 1 {
			t.Error("upgrade launched an extra game process", string(raw), e)
		}
	})
	oldRaw, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	hostRaw, err := os.ReadFile(filepath.Join(c.StateDir, "host.json"))
	if err != nil {
		t.Fatal(err)
	}
	oldACL, err := windows.GetNamedSecurityInfo(installed, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(upgradeRoot(c.Target)); !os.IsNotExist(err) {
		t.Fatal("refusing preexisting coordinator target directory", err)
	}
	t.Cleanup(func() {
		path := upgradeRoot(c.Target)
		if filepath.Base(path) != c.Target || filepath.Base(filepath.Dir(path)) != "upgrades" {
			t.Error("unsafe coordinator cleanup")
			return
		}
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	m, err := mgr.Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Disconnect() })
	name := ServiceName(c.Target)
	if old, e := m.OpenService(name); e == nil {
		old.Close()
		t.Fatal("refusing preexisting managed service")
	} else if !errors.Is(e, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		t.Fatal(e)
	}
	account := "LocalSystem"
	if !wantFailure {
		account = `NT SERVICE\` + name
	}
	service, err := m.CreateService(name, installed, mgr.Config{StartType: mgr.StartManual, ServiceStartName: account, SidType: windows.SERVICE_SID_TYPE_UNRESTRICTED, DisplayName: "Disposable dzdeploy upgrade fixture"}, "agent", "--host-config", filepath.Join(c.StateDir, "host.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !wantFailure {
		sid, e := virtualServiceSID(name)
		if e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{c.StateDir, c.InstallDir, filepath.Dir(c.SteamCMD), c.WorkshopDir} {
			if e = protectServiceTree(context.Background(), path, sid.String()); e != nil {
				t.Fatal(e)
			}
		}
		// Capture the installed file security after provisioning the actual
		// virtual service account; replacement must preserve this exact policy.
		oldACL, err = windows.GetNamedSecurityInfo(installed, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
	}
	var game ProcessIdentity
	var req UpgradeRequest
	var operations []UpgradeRequest
	t.Cleanup(func() {
		if game.PID > 0 {
			if p, e := os.FindProcess(game.PID); e == nil {
				_ = p.Kill()
			}
		}
		_ = service.ResetRecoveryActions()
		_, _ = service.Control(svc.Stop)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		_ = waitUpgradeService(ctx, service, svc.Stopped)
		cancel()
		_ = service.Delete()
		service.Close()
		for _, operation := range operations {
			if coordinator, e := m.OpenService(upgradeCoordinatorName(operation)); e == nil {
				_ = coordinator.ResetRecoveryActions()
				status, _ := coordinator.Query()
				if status.State != svc.Stopped && status.ProcessId != 0 {
					p := ProcessIdentity{PID: int(status.ProcessId), Executable: filepath.Join(upgradeRoot(c.Target), operation.ID, "coordinator.exe")}
					p.Token, _ = processToken(p.PID, p.Executable)
					_ = terminateUpgradeManager(p)
				}
				_ = coordinator.Delete()
				coordinator.Close()
			}
		}
	})
	if err = service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: time.Second}}, 300); err != nil {
		t.Fatal(err)
	}
	if err = service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		t.Fatal(err)
	}
	policyConfig, err := service.Config()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := snapshotUpgradePolicy(service, policyConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		var state State
		if ReadJSON(filepath.Join(c.StateDir, "state.json"), &state) == nil && state.PID > 0 {
			game = ProcessIdentity{PID: state.PID, Token: state.ProcessToken, Executable: filepath.Join(c.InstallDir, "DayZServer_x64.exe")}
			if upgradeIdentityAlive(game) == nil {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("legacy manager did not launch its ticking child")
		}
		time.Sleep(100 * time.Millisecond)
	}
	beforeTicks, err := os.ReadFile(filepath.Join(c.InstallDir, "ticks.txt"))
	if err != nil {
		t.Fatal(err)
	}
	id := NewID()
	stage := filepath.Join(c.StateDir, "setup", id)
	if err = os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if err = AtomicCopy(candidate, filepath.Join(stage, "dzdeploy.exe"), 0700); err != nil {
		t.Fatal(err)
	}
	req = UpgradeRequest{Version: Protocol, ID: id, Target: c.Target, CandidateSHA256: candidateIdentity.SHA256, Revision: candidateIdentity.Revision, Host: c}
	operations = append(operations, req)
	requestPath := filepath.Join(stage, "request.json")
	if err = WriteJSON(requestPath, req); err != nil {
		t.Fatal(err)
	}
	if err = SubmitAgentUpgrade(context.Background(), requestPath, io.Discard); err != nil {
		t.Fatal(err)
	}
	var result UpgradeResult
	interrupted := false
	deadline = time.Now().Add(90 * time.Second)
	for {
		if ReadJSON(filepath.Join(upgradeRoot(c.Target), id, "result.json"), &result) == nil && result.Status != "running" {
			break
		}
		if interrupt && !interrupted && result.Phase == "starting" {
			managed, e := service.Query()
			if e == nil && managed.ProcessId != 0 {
				coordinator, e := m.OpenService(upgradeCoordinatorName(req))
				if e != nil {
					t.Fatal(e)
				}
				status, e := coordinator.Query()
				coordinator.Close()
				if e != nil {
					t.Fatal(e)
				}
				p := ProcessIdentity{PID: int(status.ProcessId), Executable: filepath.Join(upgradeRoot(c.Target), id, "coordinator.exe")}
				p.Token, e = processToken(p.PID, p.Executable)
				if e != nil {
					t.Fatal(e)
				}
				if e = terminateUpgradeManager(p); e != nil {
					t.Fatal(e)
				}
				interrupted = true
			}
		}
		if time.Now().After(deadline) {
			journal, _ := os.ReadFile(filepath.Join(upgradeRoot(c.Target), id, "journal.json"))
			t.Fatalf("upgrade timed out: %+v\n%s", result, journal)
		}
		if err = upgradeIdentityAlive(game); err != nil {
			t.Fatal("game continuity lost during upgrade", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if interrupt && !interrupted {
		t.Fatal("coordinator interruption window was not exercised")
	}
	if err = result.Validate(c.Target, id); err != nil {
		t.Fatal(err)
	}
	if result.Continuity != true || !upgradeSameGame(result.Game, game) {
		t.Fatal("receipt lost exact child identity", result)
	}
	if err = upgradeIdentityAlive(game); err != nil {
		t.Fatal("original game no longer alive", err)
	}
	afterTicks, err := os.ReadFile(filepath.Join(c.InstallDir, "ticks.txt"))
	if err != nil || string(afterTicks) == string(beforeTicks) {
		t.Fatal("child did not keep ticking", err)
	}
	gotPolicyConfig, err := service.Config()
	if err != nil {
		t.Fatal(err)
	}
	gotPolicy, err := snapshotUpgradePolicy(service, gotPolicyConfig)
	if err != nil || !reflect.DeepEqual(policy, gotPolicy) {
		t.Fatal("service policy changed", policy, gotPolicy, err)
	}
	newHost, err := os.ReadFile(filepath.Join(c.StateDir, "host.json"))
	if err != nil || string(hostRaw) != string(newHost) {
		t.Fatal("host settings changed", err)
	}
	newACL, err := windows.GetNamedSecurityInfo(installed, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || newACL.String() != oldACL.String() {
		t.Fatal("binary ACL changed", err)
	}
	gotBinary, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if wantFailure {
		status, e := service.Query()
		if e != nil || status.State != svc.Stopped || result.Status != "failed" || !result.StartupPending || Digest(gotBinary) != Digest(oldRaw) {
			t.Fatal("failed candidate did not restore safe legacy state", result, status, e)
		}
	} else {
		if result.Status != "succeeded" || result.StartupPending || Digest(gotBinary) != candidateIdentity.SHA256 {
			t.Fatal("upgrade did not commit candidate", result)
		}
		var heartbeat Heartbeat
		if err = ReadJSON(filepath.Join(c.StateDir, "heartbeat.json"), &heartbeat); err != nil || heartbeat.Agent == nil || heartbeat.Agent.SHA256 != candidateIdentity.SHA256 {
			t.Fatal("replacement identity missing from heartbeat", heartbeat, err)
		}
		// Repeating the same operation must read its terminal receipt, never retire
		// the replacement manager or create a second child.
		if err = SubmitAgentUpgrade(context.Background(), requestPath, io.Discard); err != nil {
			t.Fatal(err)
		}
		if err = upgradeIdentityAlive(game); err != nil {
			t.Fatal(err)
		}
		firstManager := heartbeat.Agent.PID
		secondReq := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, CandidateSHA256: secondIdentity.SHA256, Revision: secondIdentity.Revision, Host: c}
		operations = append(operations, secondReq)
		secondStage := filepath.Join(c.StateDir, "setup", secondReq.ID)
		if err = os.MkdirAll(secondStage, 0700); err != nil {
			t.Fatal(err)
		}
		if err = AtomicCopy(secondCandidate, filepath.Join(secondStage, "dzdeploy.exe"), 0700); err != nil {
			t.Fatal(err)
		}
		secondRequestPath := filepath.Join(secondStage, "request.json")
		if err = WriteJSON(secondRequestPath, secondReq); err != nil {
			t.Fatal(err)
		}
		if err = SubmitAgentUpgrade(context.Background(), secondRequestPath, io.Discard); err != nil {
			t.Fatal(err)
		}
		deadline = time.Now().Add(90 * time.Second)
		for {
			if err = upgradeIdentityAlive(game); err != nil {
				t.Fatal("successive upgrade lost game", err)
			}
			if ReadJSON(filepath.Join(upgradeRoot(c.Target), secondReq.ID, "result.json"), &result) == nil && result.Status != "running" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("cooperative upgrade timed out", result)
			}
			time.Sleep(50 * time.Millisecond)
		}
		if result.Status != "succeeded" || !result.Continuity || result.Previous == nil || result.Previous.PID != firstManager {
			t.Fatal("cooperative upgrade failed", result)
		}
		var proof AdoptionProof
		if err = ReadJSON(filepath.Join(upgradeRoot(c.Target), "adoption.json"), &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Game != game || proof.Parent == firstManager || proof.Agent.PID == firstManager {
			t.Fatal("successive upgrade lost original ancestry", proof)
		}
		if err = ReadJSON(filepath.Join(c.StateDir, "heartbeat.json"), &heartbeat); err != nil || heartbeat.Agent == nil || heartbeat.Agent.SHA256 != secondIdentity.SHA256 {
			t.Fatal("cooperative heartbeat missing", heartbeat, err)
		}
	}
}
