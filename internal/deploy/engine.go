package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Agent struct {
	Store *Store
	Game  Game
}

func (a *Agent) resultPath(id string) string {
	return filepath.Join(a.Store.Config.StateDir, "results", id+".json")
}
func (a *Agent) progress(r *Result, phase string) error {
	r.Phase = phase
	r.Updated = time.Now().UTC()
	a.Store.State.Phase = phase
	a.Store.State.LastOperation = r.ID
	if err := a.Store.Save(); err != nil {
		return err
	}
	return WriteJSON(a.resultPath(r.ID), r)
}
func (a *Agent) stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(a.Store.Config.ShutdownSeconds)*time.Second)
	defer cancel()
	if err := a.Game.Stop(ctx, a.Store.State); err != nil {
		return err
	}
	a.Store.State.PID = 0
	a.Store.State.ProcessToken = ""
	a.Store.State.RunningCommit = ""
	return a.Store.Save()
}
func (a *Agent) start(ctx context.Context, b *Bundle, r *Result, update bool) error {
	if a.Game.Alive(a.Store.State) {
		return errors.New("target game is still running; refusing duplicate launch")
	}
	if update {
		if err := a.progress(r, "updating"); err != nil {
			return err
		}
		updateCtx, cancel := context.WithTimeout(ctx, time.Duration(a.Store.Config.UpdateSeconds)*time.Second)
		err := a.Game.Update(updateCtx, b.Manifest.Launcher)
		cancel()
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.progress(r, "applying"); err != nil {
		return err
	}
	changed, deleted, err := a.Store.Apply(b, r.ID)
	if err != nil {
		return err
	}
	r.Changed = changed
	r.Deleted = deleted
	if err = a.progress(r, "starting"); err != nil {
		return err
	}
	pid, token, err := a.Game.Launch(b.Manifest.Launcher)
	// Persist returned process identity even if subsequent identity/health checks fail.
	a.Store.State.PID = pid
	a.Store.State.ProcessToken = token
	if saveErr := a.Store.Save(); saveErr != nil {
		return saveErr
	}
	if err != nil {
		return err
	}
	if err = a.progress(r, "checking"); err != nil {
		return err
	}
	healthCtx, cancel := context.WithTimeout(ctx, time.Duration(a.Store.Config.StartupSeconds)*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last error
	for {
		if !a.Game.Alive(a.Store.State) {
			return errors.New("game exited before becoming healthy")
		}
		if last = a.Game.Healthy(healthCtx, b.Manifest.Launcher); last == nil {
			break
		}
		select {
		case <-healthCtx.Done():
			return fmt.Errorf("startup health timed out: %v: %w", last, healthCtx.Err())
		case <-ticker.C:
		}
	}
	a.Store.State.RunningCommit = b.Manifest.Commit
	a.Store.State.RestartRequired = false
	a.Store.State.Paused = false
	a.Store.State.Error = ""
	return a.Store.Save()
}
func (a *Agent) Process(ctx context.Context, req Request) (r Result) {
	r = Result{Version: Protocol, ID: req.ID, Target: a.Store.Config.Target, Status: "running", Updated: time.Now().UTC()}
	if !ValidID(req.ID) {
		r.Status = "failed"
		r.Error = "invalid request ID"
		return
	}
	// Completed operation IDs are immutable: reconnect/repeated delivery cannot reapply.
	var old Result
	if ReadJSON(a.resultPath(req.ID), &old) == nil && (old.Status == "succeeded" || old.Status == "failed") {
		return old
	}
	fail := func(err error) {
		r.Status = "failed"
		r.Error = err.Error()
		a.Store.State.Error = err.Error()
		if _, journalErr := os.Stat(filepath.Join(a.Store.Config.StateDir, "journal.json")); journalErr == nil {
			a.Store.State.Paused = true
		}
		_ = a.progress(&r, "failed")
	}
	if req.Version != Protocol {
		fail(VersionError(req.Version))
		return
	}
	if req.Target != a.Store.Config.Target {
		fail(errors.New("request target mismatch"))
		return
	}
	if req.Action != "deploy" && req.Action != "restart" && req.Action != "rollback" {
		fail(errors.New("unsupported action"))
		return
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "journal.json")); err == nil {
		fail(errors.New("unrecovered transaction blocks operations; restart the agent after repairing recovery access"))
		return
	}
	if err := a.progress(&r, "validating"); err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		return
	}
	var b *Bundle
	var err error
	switch req.Action {
	case "deploy":
		p := filepath.Join(a.Store.Config.StateDir, "inbox", req.ID+".zip")
		var raw []byte
		fi, e := os.Stat(p)
		if e != nil {
			err = e
		} else if fi.Size() > MaxPackageBytes {
			err = errors.New("package too large")
		} else {
			raw, err = os.ReadFile(p)
		}
		if err == nil && Digest(raw) != req.PackageSHA256 {
			err = errors.New("package transfer hash mismatch")
		}
		if err == nil {
			b, err = Decode(raw, req.Target)
		}
		if err == nil && b.Manifest.ID != req.ID {
			err = errors.New("deployment/package ID mismatch")
		}
		if err == nil {
			dest := packagePath(a.Store.Config.StateDir, b.Manifest.ID)
			if existing, e := os.ReadFile(dest); e == nil && Digest(existing) != Digest(raw) {
				err = errors.New("immutable release ID already exists with different bytes")
			} else if e != nil && !os.IsNotExist(e) {
				err = e
			} else {
				err = AtomicWrite(dest, raw, 0600)
			}
		}
	case "rollback":
		if !ValidID(req.Deployment) {
			err = errors.New("rollback requires a valid deployment ID")
			break
		}
		var receipt Manifest
		err = ReadJSON(filepath.Join(a.Store.Config.StateDir, "releases", req.Deployment+".success.json"), &receipt)
		if err == nil {
			b, err = LoadBundle(packagePath(a.Store.Config.StateDir, req.Deployment), req.Target)
		}
		if err == nil && receipt.Commit != b.Manifest.Commit {
			err = errors.New("release receipt mismatch")
		}
	case "restart":
		if a.Store.State.Installed == nil {
			err = errors.New("no deployed release; deploy configuration first")
		} else {
			b, err = LoadBundle(packagePath(a.Store.Config.StateDir, a.Store.State.Installed.ID), req.Target)
		}
	}
	if err != nil {
		fail(err)
		return
	}
	r.Commit = b.Manifest.Commit
	if err = a.Store.Preflight(b); err != nil {
		fail(err)
		return
	}
	if err = ctx.Err(); err != nil {
		fail(err)
		return
	}
	previous := a.Store.State.Installed
	if req.Restart || req.Action == "restart" {
		wasPaused := a.Store.State.Paused
		a.Store.State.Paused = true
		if err = a.progress(&r, "stopping"); err == nil {
			err = a.stop(ctx)
		}
		if err != nil {
			a.Store.State.Paused = wasPaused
			fail(err)
			return
		}
		err = a.start(ctx, b, &r, true)
		if err != nil {
			original := err
			// Never launch over a stuck/new process. Recovery gets one clean stop attempt.
			if ctx.Err() == nil && previous != nil {
				if stopErr := a.stop(ctx); stopErr != nil {
					r.Recovery = "failed: " + stopErr.Error()
				} else {
					oldBundle, loadErr := LoadBundle(packagePath(a.Store.Config.StateDir, previous.ID), req.Target)
					if loadErr == nil {
						recovery := Result{Version: Protocol, ID: NewID(), Target: req.Target, Status: "running"}
						loadErr = a.start(ctx, oldBundle, &recovery, false)
					}
					if loadErr == nil {
						r.Recovery = "previous configuration recovered; DayZ/mod updates were not reverted"
					} else {
						r.Recovery = "failed: " + loadErr.Error()
					}
				}
			}
			if r.Recovery == "" || strings.HasPrefix(r.Recovery, "failed:") {
				a.Store.State.Paused = true
			}
			fail(original)
			return
		}
	} else {
		if err = a.progress(&r, "applying"); err == nil {
			r.Changed, r.Deleted, err = a.Store.Apply(b, req.ID)
		}
		if err != nil {
			fail(err)
			return
		}
		a.Store.State.RestartRequired = true
		r.RestartRequired = true
	}
	if err = WriteJSON(filepath.Join(a.Store.Config.StateDir, "releases", b.Manifest.ID+".success.json"), b.Manifest); err != nil {
		fail(err)
		return
	}
	r.Status = "succeeded"
	a.Store.State.Error = ""
	if err = a.progress(&r, "complete"); err != nil {
		r.Status = "failed"
		r.Error = err.Error()
		return
	}
	// A retention failure does not misreport a completed deployment as rolled back.
	if err = a.Store.Retain(); err != nil {
		r.Recovery = "deployment completed; retention cleanup needs attention: " + err.Error()
		_ = WriteJSON(a.resultPath(r.ID), r)
	}
	return
}
func RunAgent(ctx context.Context, c HostConfig, game Game) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(c.StateDir, 0700); err != nil {
		return err
	}
	unlock, err := lockHost(c.StateDir)
	if err != nil {
		return err
	}
	defer unlock()
	if err = os.MkdirAll(c.InstallDir, 0700); err != nil {
		return err
	}
	unlockInstall, err := lockHost(c.InstallDir)
	if err != nil {
		return fmt.Errorf("another agent owns this installation: %w", err)
	}
	defer unlockInstall()
	store, err := OpenStore(c)
	if err != nil {
		return err
	}
	if err = store.Recover(); err != nil {
		return fmt.Errorf("interrupted transaction recovery failed; launch blocked: %w", err)
	}
	if game == nil {
		game = &NativeGame{Config: store.Config}
	}
	a := &Agent{Store: store, Game: game}
	// Non-file phases (updates/startup) may have been interrupted too. Never blindly replay them.
	entries, err := os.ReadDir(filepath.Join(c.StateDir, "results"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		var r Result
		if !strings.HasSuffix(e.Name(), ".json") || ReadJSON(filepath.Join(c.StateDir, "results", e.Name()), &r) != nil {
			continue
		}
		if r.Status == "running" {
			r.Status = "failed"
			r.Phase = "interrupted"
			r.Error = "agent stopped during operation; inspect status and explicitly restart"
			r.Updated = time.Now().UTC()
			store.State.Paused = true
			if err = WriteJSON(a.resultPath(r.ID), r); err != nil {
				return err
			}
		}
	}
	if err = store.Save(); err != nil {
		return err
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan struct{})
	defer func() { cancel(); <-heartbeatDone }()
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			_ = WriteJSON(filepath.Join(c.StateDir, "heartbeat.json"), struct {
				Version int       `json:"version"`
				Updated time.Time `json:"updated"`
			}{Protocol, time.Now().UTC()})
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			_ = store.Save()
			stopCtx, cancel := context.WithTimeout(context.Background(), time.Duration(store.Config.ShutdownSeconds)*time.Second)
			err = a.stop(stopCtx)
			cancel()
			return err
		}
		requests, err := os.ReadDir(filepath.Join(c.StateDir, "inbox"))
		if err != nil {
			return err
		}
		sort.Slice(requests, func(i, j int) bool { return requests[i].Name() < requests[j].Name() })
		for _, entry := range requests {
			if ctx.Err() != nil {
				break
			}
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".json")
			if !ValidID(id) {
				continue
			}
			var existing Result
			if ReadJSON(a.resultPath(id), &existing) == nil && (existing.Status == "succeeded" || existing.Status == "failed") {
				if err = clearInbox(c.StateDir, id); err != nil {
					return err
				}
				continue
			}
			var req Request
			err = ReadJSON(filepath.Join(c.StateDir, "inbox", entry.Name()), &req)
			if err != nil || req.ID != id {
				result := Result{Version: Protocol, ID: id, Target: c.Target, Status: "failed", Phase: "rejected", Error: "malformed request or filename/ID mismatch", Updated: time.Now().UTC()}
				if err = WriteJSON(a.resultPath(id), result); err != nil {
					return err
				}
				continue
			}
			result := a.Process(ctx, req)
			if result.Status == "failed" {
				if err = WriteJSON(a.resultPath(id), result); err != nil {
					return err
				}
			}
			if _, journalErr := os.Stat(filepath.Join(c.StateDir, "journal.json")); journalErr == nil {
				return errors.New("transaction recovery incomplete; launches are blocked")
			}
			if err = clearInbox(c.StateDir, id); err != nil {
				return err
			}
		}
		if !store.State.Paused && store.State.Installed != nil && !game.Alive(store.State) {
			b, err := LoadBundle(packagePath(c.StateDir, store.State.Installed.ID), c.Target)
			r := Result{Version: Protocol, ID: NewID(), Target: c.Target, Status: "running", Commit: store.State.Installed.Commit}
			store.State.PID = 0
			store.State.ProcessToken = ""
			store.State.RunningCommit = ""
			if err == nil {
				err = a.start(ctx, b, &r, true)
			}
			if err != nil {
				store.State.Paused = true
				store.State.Error = err.Error()
				r.Status = "failed"
				r.Error = err.Error()
				_ = a.progress(&r, "failed")
			} else {
				r.Status = "succeeded"
				_ = a.progress(&r, "complete")
				_ = store.Retain()
			}
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

func clearInbox(root, id string) error {
	if !ValidID(id) {
		return errors.New("invalid inbox cleanup ID")
	}
	for _, ext := range []string{".json", ".zip"} {
		p, err := SafeJoin(root, "inbox/"+id+ext)
		if err != nil {
			return err
		}
		if err = os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
