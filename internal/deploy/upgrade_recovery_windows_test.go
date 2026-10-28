//go:build windows

package deploy

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func upgradeRecoveryFixture(t *testing.T) (string, UpgradeRequest, upgradeRetirementRequest) {
	t.Helper()
	t.Setenv("ProgramData", t.TempDir())
	c := testConfig(t)
	req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40), Host: c}
	dir, err := protectedUpgradePath(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	r := upgradeRetirementRequest{Request: req, Manager: ProcessIdentity{PID: 41, Token: "1:41", Executable: filepath.Join(c.StateDir, "bin", "dzdeploy.exe")}, Game: ProcessIdentity{PID: 42, Token: "1:42", Executable: filepath.Join(c.InstallDir, "DayZServer_x64.exe")}}
	if err = WriteJSON(filepath.Join(dir, "retire-request.json"), r); err != nil {
		t.Fatal(err)
	}
	return dir, req, r
}

func TestUpgradeGuardianRecoversIntentBeforeProcessCreation(t *testing.T) {
	dir, req, r := upgradeRecoveryFixture(t)
	before, err := os.ReadFile(filepath.Join(dir, "retire-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	guardian := ProcessIdentity{PID: 43, Token: "1:43", Executable: filepath.Join(dir, "coordinator.exe")}
	starts := 0
	native := upgradeGuardianNative{alive: func(p ProcessIdentity) (bool, error) { return p == r.Manager || p == r.Game || p == guardian, nil }, retire: func(ProcessIdentity) error { t.Fatal("live game recovery retired a manager directly"); return nil }, start: func(got string) error {
		if got != dir {
			t.Fatal("guardian escaped original operation")
		}
		starts++
		return WriteJSON(filepath.Join(dir, "guardian.json"), guardian)
	}}
	ready, err := reconcileUpgradeGuardian(dir, req, native)
	if err != nil || ready || starts != 1 {
		t.Fatalf("intent recovery ready=%v starts=%d error=%v", ready, starts, err)
	}
	ready, err = reconcileUpgradeGuardian(dir, req, native)
	if err != nil || ready || starts != 1 {
		t.Fatalf("live guardian was duplicated: ready=%v starts=%d error=%v", ready, starts, err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "retire-request.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("guardian recovery changed the original request or operation ID")
	}
}

func TestUpgradeGuardianInfersRetirementAfterArmedCrash(t *testing.T) {
	dir, req, r := upgradeRecoveryFixture(t)
	guardian := ProcessIdentity{PID: 43, Token: "1:43", Executable: filepath.Join(dir, "coordinator.exe")}
	if err := WriteJSON(filepath.Join(dir, "guardian.json"), guardian); err != nil {
		t.Fatal(err)
	}
	native := upgradeGuardianNative{alive: func(p ProcessIdentity) (bool, error) { return p == r.Game, nil }, start: func(string) error { t.Fatal("crashed guardian restarted after manager was already gone"); return nil }, retire: func(ProcessIdentity) error {
		t.Fatal("retirement was attempted against a reused/gone manager PID")
		return nil
	}}
	ready, err := reconcileUpgradeGuardian(dir, req, native)
	if err != nil || !ready {
		t.Fatalf("armed guardian crash was not reconciled: ready=%v error=%v", ready, err)
	}
	retired, err := waitUpgradeRetirement(context.Background(), dir, req)
	if !retired || err == nil || !strings.Contains(err.Error(), "paused recovery") {
		t.Fatalf("inferred retirement did not require paused recovery: retired=%v error=%v", retired, err)
	}
}

func TestUpgradeGuardianRetiresOnlyVerifiedManagerWhenGameExited(t *testing.T) {
	dir, req, r := upgradeRecoveryFixture(t)
	retirements := 0
	native := upgradeGuardianNative{alive: func(p ProcessIdentity) (bool, error) { return p == r.Manager, nil }, start: func(string) error { t.Fatal("guardian restarted after game continuity was lost"); return nil }, retire: func(p ProcessIdentity) error {
		if p != r.Manager {
			t.Fatal("retirement targeted another process")
		}
		retirements++
		return nil
	}}
	ready, err := reconcileUpgradeGuardian(dir, req, native)
	if err != nil || !ready || retirements != 1 {
		t.Fatalf("game-exit recovery ready=%v retirements=%d error=%v", ready, retirements, err)
	}
	var result upgradeRetirementOutcome
	if err = ReadJSON(filepath.Join(dir, "retire-result.json"), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Retired || result.Error == "" {
		t.Fatal("game-exit recovery did not persist required paused recovery")
	}
}

func TestUpgradeGuardianInspectionFailureDoesNotInferRetirement(t *testing.T) {
	dir, req, _ := upgradeRecoveryFixture(t)
	blocked := errors.New("process access denied")
	native := upgradeGuardianNative{alive: func(ProcessIdentity) (bool, error) { return false, blocked }, start: func(string) error { t.Fatal("inspection failure launched a guardian"); return nil }, retire: func(ProcessIdentity) error { t.Fatal("inspection failure retired a process"); return nil }}
	if _, err := reconcileUpgradeGuardian(dir, req, native); !errors.Is(err, blocked) {
		t.Fatalf("native inspection failure was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "retire-result.json")); !os.IsNotExist(err) {
		t.Fatal("access failure was treated as proof of retirement")
	}
}

func TestUpgradeCoordinatorClaimsGateAfterIndependentLock(t *testing.T) {
	_, req, _ := upgradeRecoveryFixture(t)
	// Staged request/receipt artifacts alone leave ordinary supervision ungated.
	if _, err := os.Stat(filepath.Join(upgradeRoot(req.Target), "active.json")); !os.IsNotExist(err) {
		t.Fatal("staging created an active marker")
	}
	submitUnlock, err := lockHost(upgradeRoot(req.Target))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if unlock, err := lockUpgradeTarget(ctx, req); err == nil {
		unlock()
		submitUnlock()
		t.Fatal("coordinator did not wait for unfinished submission")
	}
	submitUnlock()
	unlock, err := lockUpgradeTarget(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err = claimUpgradeActive(req); err != nil {
		t.Fatal(err)
	}
	other := req
	other.ID = NewID()
	if err = claimUpgradeActive(other); err == nil {
		t.Fatal("competing operation replaced active marker")
	}
	active, err := readUpgradeActive(req.Target)
	if err != nil || active.ID != req.ID {
		t.Fatal("competing operation changed original marker")
	}
}

func TestUpgradeJournalRejectsForeignProtectedReceipts(t *testing.T) {
	_, req, _ := upgradeRecoveryFixture(t)
	base := upgradeJournal{Request: req, Result: UpgradeResult{Version: Protocol, ID: req.ID, Target: req.Target, Status: "running"}}
	for _, field := range []string{"version", "id", "target", "status"} {
		t.Run(field, func(t *testing.T) {
			j := base
			switch field {
			case "version":
				j.Result.Version++
			case "id":
				j.Result.ID = NewID()
			case "target":
				j.Result.Target = "nasdara"
			case "status":
				j.Result.Status = "corrupt"
			}
			if err := validateUpgradeJournal(&j, req); err == nil {
				t.Fatalf("foreign journal result %s accepted", field)
			}
		})
	}
}
