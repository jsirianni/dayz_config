//go:build windows

package deploy

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestUpgradeProtectedPathRequiresAbsoluteProgramData(t *testing.T) {
	req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: "nasdara-staging", CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40), Host: testConfig(t)}
	t.Setenv("ProgramData", "relative-directory")
	if _, err := protectedUpgradePath(req); err == nil {
		t.Fatal("relative ProgramData was accepted for privileged coordinator artifacts")
	}
	root := t.TempDir()
	t.Setenv("ProgramData", root)
	got, err := protectedUpgradePath(req)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "dzdeploy", "upgrades", req.Target, req.ID); got != want {
		t.Fatalf("protected operation path = %s, want %s", got, want)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatal("resolving protected operation path mutated the filesystem")
	}
}

func TestUpgradeProtectedPathRejectsRuntimeOverlapBeforeMutation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramData", root)
	for _, field := range []string{"state", "install", "workshop", "steam", "ancestor"} {
		t.Run(field, func(t *testing.T) {
			c := testConfig(t)
			protected := filepath.Join(root, "dzdeploy")
			switch field {
			case "state":
				c.StateDir = filepath.Join(protected, "runtime")
				c.RConPasswordFile = filepath.Join(c.StateDir, "secret")
			case "install":
				c.InstallDir = filepath.Join(protected, "runtime")
			case "workshop":
				c.WorkshopDir = filepath.Join(protected, "workshop")
			case "steam":
				c.SteamCMD = filepath.Join(protected, "steam", "steamcmd.exe")
			case "ancestor":
				c.WorkshopDir = root
			}
			req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40), Host: c}
			if _, err := protectedUpgradePath(req); err == nil || !strings.Contains(err.Error(), "overlaps") {
				t.Fatalf("runtime overlap accepted or rejected for unrelated reason: %v", err)
			}
			if _, err := os.Stat(protected); !os.IsNotExist(err) {
				t.Fatal("overlap validation created protected directories")
			}
		})
	}
}

func TestUpgradeServiceBuiltinAccountSID(t *testing.T) {
	for account, want := range map[string]string{"LocalSystem": "S-1-5-18", `NT AUTHORITY\SYSTEM`: "S-1-5-18", `NT AUTHORITY\LocalService`: "S-1-5-19", "NetworkService": "S-1-5-20"} {
		got, err := upgradeServiceSID(mgr.Config{ServiceStartName: account})
		if err != nil || got != want {
			t.Fatalf("%s SID=%q, %v", account, got, err)
		}
	}
}

func TestUpgradeRecoveryRequiresPublishedProcessBoundCommit(t *testing.T) {
	dir := t.TempDir()
	c := testConfig(t)
	req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40), Host: c}
	agent := AgentIdentity{Protocol: Protocol, Revision: req.Revision, SHA256: req.CandidateSHA256, GOOS: "windows", GOARCH: "amd64", PID: 451, Token: "71:82"}
	game := ProcessIdentity{PID: 452, Token: "71:83", Executable: filepath.Join(c.InstallDir, "DayZServer_x64.exe")}
	j := upgradeJournal{Request: req, Committed: true, Result: UpgradeResult{Candidate: &agent, Game: game}}
	if _, ok := committedUpgradeAgent(dir, &j); ok {
		t.Fatal("commit intent was treated as a published decision")
	}
	ack := UpgradeAcknowledgement{Version: Protocol, ID: req.ID, Target: req.Target, Agent: agent, Game: game}
	decision := UpgradeDecision{Version: Protocol, ID: req.ID, Target: req.Target, Decision: "commit", Agent: ProcessIdentity{PID: agent.PID, Token: agent.Token, Executable: filepath.Join(c.StateDir, "bin", "dzdeploy.exe")}}
	if err := WriteJSON(filepath.Join(dir, "ack.json"), ack); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(filepath.Join(dir, "decision.json"), decision); err != nil {
		t.Fatal(err)
	}
	if got, ok := committedUpgradeAgent(dir, &j); !ok || got.PID != agent.PID {
		t.Fatal("complete matching commit was not recovered")
	}
	decision.Agent.Token = "reused-pid"
	if err := WriteJSON(filepath.Join(dir, "decision.json"), decision); err != nil {
		t.Fatal(err)
	}
	if _, ok := committedUpgradeAgent(dir, &j); ok {
		t.Fatal("stale commit authorized another process creation")
	}
}

func TestUpgradeTerminalReplayRepublishesReceiptAndRemovesGate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramData", root)
	for _, phase := range []string{"complete", "installed-awaiting-startup", "aborted", "rejected", "rolled-back", "recovery-required"} {
		t.Run(phase, func(t *testing.T) {
			c := testConfig(t)
			req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40), Host: c}
			dir, err := protectedUpgradePath(req)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			active := filepath.Join(upgradeRoot(c.Target), "active.json")
			if err = WriteJSON(active, map[string]any{"version": Protocol, "id": req.ID, "target": req.Target}); err != nil {
				t.Fatal(err)
			}
			status := "failed"
			if phase == "complete" || phase == "installed-awaiting-startup" {
				status = "succeeded"
			}
			j := upgradeJournal{Request: req, Result: UpgradeResult{Version: Protocol, ID: req.ID, Target: req.Target, Status: status, Phase: phase}}
			// Reproduce a crash after the terminal journal was flushed, before the
			// result receipt and active-marker cleanup could complete.
			if err = WriteJSON(filepath.Join(dir, "journal.json"), j); err != nil {
				t.Fatal(err)
			}
			if err = finalizeTerminalUpgrade(dir, &j, nil); err != nil {
				t.Fatal(err)
			}
			var result UpgradeResult
			if err = ReadJSON(filepath.Join(dir, "result.json"), &result); err != nil {
				t.Fatal(err)
			}
			if err = result.Validate(req.Target, req.ID); err != nil {
				t.Fatal(err)
			}
			_, gateErr := os.Stat(active)
			if phase == "recovery-required" {
				if gateErr != nil {
					t.Fatal("unsafe recovery-required gate was removed")
				}
			} else if !os.IsNotExist(gateErr) {
				t.Fatal("terminal replay left active marker behind")
			}
			if err = finalizeTerminalUpgrade(dir, &j, nil); err != nil {
				t.Fatalf("terminal cleanup was not idempotent: %v", err)
			}
		})
	}
}

func TestUpgradeRecoveryPausePreservesGameDeploymentState(t *testing.T) {
	c := testConfig(t)
	target, _ := TargetFor(c.Target)
	before := State{Version: Protocol, Target: c.Target, PID: 4101, ProcessToken: "17:42", RunningCommit: strings.Repeat("a", 40), Phase: "complete", LastOperation: NewID(), Error: "existing diagnostic", Updated: time.Now().Add(-time.Minute).UTC(), RestartRequired: true, Installed: &Manifest{Version: Protocol, ID: NewID(), Target: c.Target, Commit: strings.Repeat("b", 40), Launcher: target}}
	path := filepath.Join(c.StateDir, "state.json")
	if err := WriteJSON(path, before); err != nil {
		t.Fatal(err)
	}
	if err := pauseUpgradeState(c); err != nil {
		t.Fatal(err)
	}
	var after State
	if err := ReadJSON(path, &after); err != nil {
		t.Fatal(err)
	}
	if !after.Paused || !after.Updated.After(before.Updated) {
		t.Fatal("recovery did not persist a fresh paused state")
	}
	before.Paused, before.Updated = true, after.Updated
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("manager-only recovery changed game/deployment state:\nbefore %+v\nafter %+v", before, after)
	}
	c.Target = "nasdara"
	raw, _ := os.ReadFile(path)
	if err := pauseUpgradeState(c); err == nil {
		t.Fatal("recovery paused a foreign target state")
	}
	if current, _ := os.ReadFile(path); !bytes.Equal(raw, current) {
		t.Fatal("foreign target rejection changed the saved state")
	}
}

func TestUpgradeBinaryReplacementPreservesSecurityAndChecksums(t *testing.T) {
	dir := t.TempDir()
	dest, source := filepath.Join(dir, "installed.exe"), filepath.Join(dir, "candidate.exe")
	if err := os.WriteFile(dest, []byte("previous executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("candidate executable"), 0600); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if err = upgradeSetDACL(dest, "D:P(A;;FA;;;"+user.User.Sid.String()+")(A;;FA;;;SY)"); err != nil {
		t.Fatal(err)
	}
	flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION)
	before, err := windows.GetNamedSecurityInfo(dest, windows.SE_FILE_OBJECT, flags)
	if err != nil {
		t.Fatal(err)
	}
	acl := before.String()
	if !strings.Contains(acl, "O:") || !strings.Contains(acl, "G:") {
		t.Fatal("security snapshot omitted owner/group")
	}
	if err = replaceUpgradeBinary(source, dest, acl, strings.Repeat("0", 64)); err == nil {
		t.Fatal("replacement accepted the wrong candidate checksum")
	}
	if raw, _ := os.ReadFile(dest); string(raw) != "previous executable" {
		t.Fatal("checksum rejection changed the installed executable")
	}
	if err = replaceUpgradeBinary(source, dest, acl, Digest([]byte("candidate executable"))); err != nil {
		t.Fatalf("restore binary security for current user %s (%s): %v", user.User.Sid.String(), acl, err)
	}
	if raw, _ := os.ReadFile(dest); string(raw) != "candidate executable" {
		t.Fatal("replacement did not install the verified bytes")
	}
	after, err := windows.GetNamedSecurityInfo(dest, windows.SE_FILE_OBJECT, flags)
	if err != nil {
		t.Fatal(err)
	}
	if after.String() != acl {
		t.Fatalf("replacement changed ACL, owner, or group:\n%s\n%s", acl, after.String())
	}
	if paths, _ := filepath.Glob(dest + ".upgrade-*"); len(paths) != 0 {
		t.Fatalf("replacement left staging executables: %v", paths)
	}
}

func TestUpgradeAdministrativeArtifactOwnerPreservesInheritance(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("requires an elevated disposable Windows runner")
	}
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if err = upgradeAdministrativeOwner(path); err != nil {
		t.Fatal(err)
	}
	after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	beforeDACL, err := captureUpgradeDACL(before)
	if err != nil {
		t.Fatal(err)
	}
	afterDACL, err := captureUpgradeDACL(after)
	if err != nil {
		t.Fatal(err)
	}
	if beforeDACL != afterDACL {
		t.Fatalf("artifact owner normalization changed DACL or inheritance:\nDACL before=%+v\nDACL after=%+v\nfull before=%s\nfull after=%s", beforeDACL, afterDACL, before.String(), after.String())
	}
	security, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := security.Owner()
	if err != nil {
		t.Fatal(err)
	}
	group, _, err := security.Group()
	if err != nil {
		t.Fatal(err)
	}
	if owner.String() != "S-1-5-32-544" || group.String() != "S-1-5-32-544" {
		t.Fatal("protected artifact is not owned by Administrators")
	}
	if err = upgradeAdministrativeOwner(path); err != nil {
		t.Fatalf("owner normalization was not idempotent: %v", err)
	}
}

func TestUpgradeDACLComparisonExcludesOwnerAndKeepsAccessAndInheritance(t *testing.T) {
	views := make([]upgradeDACLView, 0, 4)
	for _, sddl := range []string{"O:SYG:SYD:AI(A;ID;FR;;;AU)", "O:BAG:BAD:AI(A;ID;FR;;;AU)", "O:BAG:BAD:AI(A;ID;FW;;;AU)", "O:BAG:BAD:P(A;ID;FR;;;AU)"} {
		sd, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			t.Fatal(err)
		}
		view, err := captureUpgradeDACL(sd)
		if err != nil {
			t.Fatal(err)
		}
		views = append(views, view)
	}
	if views[0] != views[1] {
		t.Fatalf("owner/group change affected DACL comparison: %+v %+v", views[0], views[1])
	}
	if views[1] == views[2] {
		t.Fatal("DACL comparison ignored an actual access-mask change")
	}
	if views[1] == views[3] {
		t.Fatal("DACL comparison ignored changed inheritance protection")
	}
}

// This opens a system service with QUERY_CONFIG only. The function under test
// must remain usable without any right to modify services or their policies.
func TestUpgradeServicePolicySnapshotNeedsReadAccessOnly(t *testing.T) {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		t.Skipf("SCM read access unavailable: %v", err)
	}
	defer windows.CloseServiceHandle(m)
	name, _ := windows.UTF16PtrFromString("EventLog")
	h, err := windows.OpenService(m, name, windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		t.Skipf("EventLog policy read access unavailable: %v", err)
	}
	s := &mgr.Service{Name: "EventLog", Handle: h}
	defer s.Close()
	before, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	p, err := snapshotUpgradePolicy(s, before)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || p.StartType != before.StartType || p.Delayed != before.DelayedAutoStart {
		t.Fatal("read-only policy snapshot changed service configuration or recorded startup incorrectly")
	}
	again, err := snapshotUpgradePolicy(s, after)
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatalf("policy snapshot changed across consecutive reads: %+v, %+v (%v)", p, again, err)
	}
}
