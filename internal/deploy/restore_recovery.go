package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/snapshot"
)

func (a *Agent) recoverRestore(ctx context.Context) error {
	file := filepath.Join(a.Store.Config.StateDir, "restore.json")
	var j RestoreJournal
	if err := ReadJSON(file, &j); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if j.Version != 1 || !ValidID(j.Operation) || j.Safety != j.Operation || j.Target != a.Store.Config.Target || j.Before.Installed == nil {
		return errors.New("invalid restore journal")
	}
	if err := checkOwnership(*j.Before.Installed, j.Target); err != nil {
		return err
	}
	if j.Committed {
		return os.Remove(file)
	}
	receipt, archive, err := a.Store.verifiedSnapshot(ctx, j.Safety, true)
	if err != nil {
		return fmt.Errorf("restore safety archive is unavailable or corrupt: %w", err)
	}
	entries := map[string]bool{}
	for _, entry := range receipt.Manifest.Entries {
		entries[entry.Path] = !entry.Directory
	}
	rules := backupRules(j.Before.Installed.Launcher)
	seen := map[string]bool{}
	for _, change := range j.Changes {
		key := strings.ToLower(change.Path)
		if !rules.Allow(change.Path) || seen[key] {
			return errors.New("unsafe or duplicate restore recovery path")
		}
		seen[key] = true
		if _, err = SafeJoin(a.Store.Config.InstallDir, change.Path); err != nil {
			return err
		}
		if change.Before && !entries[change.Path] {
			return errors.New("restore preimage is missing from verified safety archive")
		}
	}
	// Verify every preimage before stopping a game or writing any live file.
	recovery, err := SafeJoin(a.Store.Config.StateDir, "restore-recovery/"+j.Operation)
	if err != nil {
		return err
	}
	if err = os.RemoveAll(recovery); err != nil {
		return err
	}
	if err = os.MkdirAll(recovery, 0700); err != nil {
		return err
	}
	defer func() {
		if checked, e := SafeJoin(a.Store.Config.StateDir, "restore-recovery/"+j.Operation); e == nil && checked == recovery {
			_ = os.RemoveAll(checked)
		}
	}()
	if _, err = snapshot.Extract(ctx, archive, j.Target, recovery, rules, nil); err != nil {
		return err
	}
	a.Store.State.Paused = true
	if err = a.Store.Save(); err != nil {
		return err
	}
	if a.Game.Alive(a.Store.State) && !j.StopAuthorized {
		return errors.New("running game blocks restore recovery without original stop authorization")
	}
	if err = a.stopMode(ctx, true); err != nil {
		return err
	}
	// A managed-config transaction may have been interrupted during restored startup.
	if err = a.Store.Recover(); err != nil {
		return err
	}
	for i := len(j.Changes) - 1; i >= 0; i-- {
		change := j.Changes[i]
		dest, e := SafeJoin(a.Store.Config.InstallDir, change.Path)
		if e != nil {
			return e
		}
		if change.Before {
			source, e := SafeJoin(recovery, change.Path)
			if e != nil {
				return e
			}
			if e = atomicCopyContext(ctx, source, dest, 0600); e != nil {
				return e
			}
		} else if e = os.Remove(dest); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	a.Store.State = j.Before
	a.Store.State.PID = 0
	a.Store.State.ProcessToken = ""
	a.Store.State.RunningCommit = ""
	a.Store.State.DZSA = nil
	a.Store.State.Paused = true
	a.Store.State.Phase = "recovered"
	a.Store.State.Error = "interrupted restore recovered; inspect status and explicitly restart"
	if err = a.Store.Save(); err != nil {
		return err
	}
	result := Result{Version: Protocol, ID: j.Operation, Target: j.Target, Status: "failed", Phase: "recovered", Error: a.Store.State.Error, Cause: "backup-restore", Updated: time.Now().UTC()}
	if err = WriteJSON(a.resultPath(j.Operation), result); err != nil {
		return err
	}
	return os.Remove(file)
}
