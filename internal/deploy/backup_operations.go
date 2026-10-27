package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (a *Agent) backupPreflight(ctx context.Context, req Request) (*Bundle, bool, error) {
	if req.Action != "backup-create" && req.Action != "backup-restore" {
		return nil, false, errors.New("unknown backup action")
	}
	if err := validateExpectedState(a.Store.State, req); err != nil {
		return nil, false, err
	}
	if a.Store.State.Installed == nil {
		return nil, false, errors.New("no installed release")
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "journal.json")); !os.IsNotExist(err) {
		return nil, false, errors.New("configuration recovery blocks backup maintenance")
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "restore.json")); !os.IsNotExist(err) {
		return nil, false, errors.New("restore recovery is pending")
	}
	b, err := LoadBundle(packagePath(a.Store.Config.StateDir, a.Store.State.Installed.ID), req.Target)
	if err != nil {
		return nil, false, err
	}
	running := a.Game.Alive(a.Store.State)
	if running && !req.Stop {
		return nil, true, errors.New("DayZ is running; --stop is required for a consistent backup/restore")
	}
	if running && (a.Store.State.RestartRequired || a.Store.State.RunningCommit != a.Store.State.Installed.Commit) {
		return nil, true, errors.New("activation is pending or prior running release is unknown; resolve it before maintenance")
	}
	if _, ok := a.Game.(HardStopper); !ok {
		return nil, running, errors.New("guarded maintenance is unavailable")
	}
	return b, running, nil
}

func (a *Agent) processBackup(ctx context.Context, req Request) (r Result) {
	r = Result{Version: Protocol, ID: req.ID, Target: a.Store.Config.Target, Status: "running", Cause: req.Action, Updated: time.Now().UTC()}
	if !ValidID(req.ID) || req.Version != Protocol || req.Target != a.Store.Config.Target {
		r.Status = "failed"
		r.Error = "invalid backup request identity"
		return
	}
	var previous Result
	if ReadJSON(a.resultPath(req.ID), &previous) == nil && (previous.Status == "succeeded" || previous.Status == "failed") {
		return previous
	}
	b, running, err := a.backupPreflight(ctx, req)
	if err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		_ = WriteJSON(a.resultPath(req.ID), r)
		return
	}
	r.Commit = b.Manifest.Commit
	if req.Action == "backup-restore" {
		return a.restoreBackup(ctx, req, b, r, running)
	}
	if _, err = a.Store.snapshotSpace(ctx, 2); err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		_ = WriteJSON(a.resultPath(req.ID), r)
		return
	}
	provenance := a.Store.State.DZSA
	// Even an already stopped installation is checked for unmanaged processes.
	if err = a.pauseAndStop(ctx, &r, true); err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		_ = a.progress(&r, "failed")
		return
	}
	err = a.progress(&r, "backing-up")
	if err == nil {
		receipt, e := a.Store.createSnapshot(ctx, req.ID, false, provenance)
		err = e
		if e == nil {
			r.Backup = &receipt
			a.event("backup-created", req.Action, &r)
		}
	}
	backupErr := err
	if running {
		if e := a.start(ctx, b, &r, false); e != nil {
			err = errors.Join(err, fmt.Errorf("resume failed: %w", e))
			a.Store.State.Paused = true
		}
	} else {
		a.Store.State.Paused = true
		_ = a.Store.Save()
	}
	if err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		if backupErr == nil {
			r.Recovery = "verified backup retained; inspect game state before another restart"
		}
		_ = a.progress(&r, "failed")
		return
	}
	r.Status = "succeeded"
	a.Store.State.Error = ""
	if err = a.progress(&r, "complete"); err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		return r
	}
	if e := a.Store.retainSnapshots(); e != nil {
		r.Recovery = "backup completed; retention needs attention: " + e.Error()
		_ = WriteJSON(a.resultPath(req.ID), r)
	}
	return
}
