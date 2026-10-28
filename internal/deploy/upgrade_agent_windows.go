//go:build windows

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func checkHandoffDirectory(dir string) error {
	if !filepath.IsAbs(dir) || isReparse(dir) {
		return errors.New("invalid protected upgrade directory")
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner.String() != "S-1-5-18" && owner.String() != "S-1-5-32-544" {
		return errors.New("upgrade directory must be owned by SYSTEM or Administrators")
	}
	return nil
}

func verifyHandoffGame(c HostConfig, expected ProcessIdentity) error {
	if expected.PID == 0 {
		return ensureNoGame(c.InstallDir)
	}
	exe := filepath.Join(c.InstallDir, "DayZServer_x64.exe")
	if expected.Token == "" || !strings.EqualFold(filepath.Clean(expected.Executable), filepath.Clean(exe)) {
		return errors.New("invalid handoff game identity")
	}
	token, err := processToken(expected.PID, exe)
	if err != nil || token != expected.Token {
		return errors.New("game continuity lost during agent handoff")
	}
	return nil
}

// awaitAgentUpgrade runs before RunAgent: even opening the store can recover a
// journal or overwrite state, so a candidate must acknowledge without doing so.
func awaitAgentUpgrade(ctx context.Context, c HostConfig, args []string) error {
	requestPath := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "dzdeploy-upgrade=") {
			if requestPath != "" {
				return errors.New("duplicate upgrade start argument")
			}
			requestPath = strings.TrimPrefix(arg, "dzdeploy-upgrade=")
		}
	}
	active := filepath.Join(upgradeRoot(c.Target), "active.json")
	if requestPath == "" {
		if _, err := os.Stat(active); err == nil {
			return errors.New("agent upgrade in progress; coordinator must authorize service startup")
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var req UpgradeRequest
	if err := ReadJSON(requestPath, &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	want := filepath.Join(upgradeRoot(c.Target), req.ID, "request.json")
	if req.Target != c.Target || !strings.EqualFold(filepath.Clean(want), filepath.Clean(requestPath)) {
		return errors.New("handoff request escaped protected operation directory")
	}
	for _, dir := range []string{filepath.Dir(upgradeRoot(c.Target)), upgradeRoot(c.Target), filepath.Dir(want)} {
		if err := checkHandoffDirectory(dir); err != nil {
			return err
		}
	}
	var hold UpgradeHold
	dir := filepath.Dir(want)
	if err := ReadJSON(filepath.Join(dir, "hold.json"), &hold); err != nil {
		return err
	}
	if hold.Version != Protocol || hold.ID != req.ID || hold.Target != c.Target {
		return errors.New("handoff hold identity mismatch")
	}
	self := CurrentAgentIdentity()
	if self.Protocol != Protocol || self.SHA256 != hold.CandidateSHA256 || self.PID != os.Getpid() || self.Token == "" {
		return errors.New("handoff executable identity mismatch")
	}
	if err := verifyHandoffGame(c, hold.Game); err != nil {
		return err
	}
	ack := UpgradeAcknowledgement{Version: Protocol, ID: req.ID, Target: c.Target, Agent: self, Game: hold.Game, Updated: time.Now().UTC()}
	raw, err := json.Marshal(ack)
	if err != nil {
		return err
	}
	// Only this precreated file is writable by the service account. The parent,
	// journal, candidate, hold, and commit decision stay administrator controlled.
	f, err := os.OpenFile(filepath.Join(dir, "ack.json"), os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var decision UpgradeDecision
		if err := ReadJSON(filepath.Join(dir, "decision.json"), &decision); err == nil {
			if decision.Version != Protocol || decision.ID != req.ID || decision.Target != c.Target || decision.Agent.PID != self.PID || decision.Agent.Token != self.Token {
				return errors.New("handoff decision identity mismatch")
			}
			if decision.Decision != "commit" {
				return errors.New("agent handoff aborted")
			}
			return verifyHandoffGame(c, hold.Game)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read handoff decision: %w", err)
		}
		if err := verifyHandoffGame(c, hold.Game); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func agentUpgradeActive(c HostConfig) error {
	_, err := os.Stat(filepath.Join(upgradeRoot(c.Target), "active.json"))
	if err == nil {
		return errors.New("agent upgrade in progress; authentication and submissions are blocked")
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}
