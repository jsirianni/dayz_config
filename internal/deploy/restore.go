package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/snapshot"
)

type RestoreChange struct {
	SourcePath string `json:"sourcePath,omitempty"`
	Directory  bool   `json:"directory,omitempty"`
	AfterHash  string `json:"afterHash,omitempty"`
	Path       string `json:"path"`
	Before     bool   `json:"before"`
	After      bool   `json:"after"`
}
type RestoreJournal struct {
	Version        int             `json:"version"`
	Operation      string          `json:"operation"`
	Target         string          `json:"target"`
	Safety         string          `json:"safety"`
	Before         State           `json:"before"`
	Changes        []RestoreChange `json:"changes"`
	Committed      bool            `json:"committed"`
	StopAuthorized bool            `json:"stopAuthorized"`
}

func (s *Store) restoreStage(id string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("invalid restore operation")
	}
	return SafeJoin(s.Config.StateDir, "restore-stage/"+id)
}

func (s *Store) stageRestore(ctx context.Context, req Request, b *Bundle) (string, BackupReceipt, error) {
	receipt, archive, err := s.verifiedSnapshot(ctx, req.Backup, req.BackupSafety)
	if err != nil {
		return "", receipt, err
	}
	stage, err := s.restoreStage(req.ID)
	if err != nil {
		return "", receipt, err
	}
	if _, err = os.Stat(stage); !os.IsNotExist(err) {
		return "", receipt, errors.New("restore staging already exists; inspect operation")
	}
	if err = os.MkdirAll(stage, 0700); err != nil {
		return "", receipt, err
	}
	if err = protectPrivateDirectory(stage); err != nil {
		return stage, receipt, err
	}
	skip := map[string]bool{}
	for _, name := range receipt.Manifest.Managed {
		skip[name] = true
	}
	for _, file := range b.Manifest.Files {
		skip[file.Destination] = true
	}
	if _, err = snapshot.Extract(ctx, archive, s.Config.Target, stage, backupRules(b.Manifest.Launcher), skip); err != nil {
		return stage, receipt, err
	}
	// Archived managed settings are evidence only. Current desired bytes win.
	for _, file := range b.Manifest.Files {
		dest, e := SafeJoin(stage, file.Destination)
		if e != nil {
			return stage, receipt, e
		}
		if e = AtomicWrite(dest, b.Content[file.Source], 0600); e != nil {
			return stage, receipt, e
		}
	}
	return stage, receipt, nil
}

func (s *Store) planRestore(ctx context.Context, stage string, safety BackupReceipt, b *Bundle) ([]RestoreChange, error) {
	after, _, err := snapshot.Scan(ctx, stage, backupRoots(b.Manifest.Launcher, &b.Manifest), backupRules(b.Manifest.Launcher))
	if err != nil {
		return nil, err
	}
	beforeMap := map[string]snapshot.Entry{}
	afterMap := map[string]snapshot.Entry{}
	for _, entry := range safety.Manifest.Entries {
		beforeMap[strings.ToLower(entry.Path)] = entry
	}
	for _, entry := range after {
		if entry.Directory {
			afterMap[strings.ToLower(entry.Path)] = entry
			continue
		}
		file, e := SafeJoin(stage, entry.Path)
		if e != nil {
			return nil, e
		}
		entry.SHA256, e = snapshot.Hash(ctx, file)
		if e != nil {
			return nil, e
		}
		afterMap[strings.ToLower(entry.Path)] = entry
	}
	names := map[string]bool{}
	for name := range beforeMap {
		names[name] = true
	}
	for name := range afterMap {
		names[name] = true
	}
	var changes []RestoreChange
	for name := range names {
		before, beforeOK := beforeMap[name]
		after, afterOK := afterMap[name]
		if beforeOK && afterOK && before.Directory != after.Directory {
			return nil, fmt.Errorf("file/directory type conflict at %s", name)
		}
		if before.Directory {
			continue
		}
		if beforeOK && afterOK && before.SHA256 == after.SHA256 {
			continue
		}
		destination := before.Path
		if !beforeOK {
			destination = existingPathCase(after.Path, beforeMap)
		}
		changes = append(changes, RestoreChange{Path: destination, SourcePath: after.Path, Before: beforeOK, After: afterOK, Directory: after.Directory, AfterHash: after.SHA256})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

// Windows paths are case-insensitive. Preserve the live spelling and use a
// separate staged source so a case-only change cannot become copy-then-delete.
func existingPathCase(relative string, before map[string]snapshot.Entry) string {
	for parent := relative; parent != "."; parent = path.Dir(parent) {
		if entry, ok := before[strings.ToLower(parent)]; ok {
			if parent == relative {
				return entry.Path
			}
			return entry.Path + "/" + strings.TrimPrefix(relative, parent+"/")
		}
	}
	return relative
}

func (s *Store) applyRestore(ctx context.Context, stage string, j RestoreJournal) error {
	for _, change := range j.Changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		dest, err := SafeJoin(s.Config.InstallDir, change.Path)
		if err != nil {
			return err
		}
		if s.BeforeWrite != nil {
			if err = s.BeforeWrite(change.Path); err != nil {
				return err
			}
		}
		if change.Directory {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return err
			}
			continue
		}
		if change.After {
			sourcePath := change.SourcePath
			if sourcePath == "" {
				sourcePath = change.Path
			}
			source, e := SafeJoin(stage, sourcePath)
			if e != nil {
				return e
			}
			if e = atomicCopyContext(ctx, source, dest, 0600); e != nil {
				return e
			}
			if hash, e := snapshot.Hash(ctx, dest); e != nil || hash != change.AfterHash {
				return errors.New("restored file content verification failed")
			}
		} else if err = os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (a *Agent) restoreBackup(ctx context.Context, req Request, b *Bundle, r Result, running bool) Result {
	fail := func(err error) Result {
		r.Status = "failed"
		r.Error = err.Error()
		_ = a.progress(&r, "failed")
		return r
	}
	receipt, _, err := a.Store.verifiedSnapshot(ctx, req.Backup, req.BackupSafety)
	if err != nil {
		return fail(err)
	}
	var archivedBytes uint64
	for _, entry := range receipt.Manifest.Entries {
		archivedBytes += uint64(entry.Size)
	}
	if required, e := a.Store.snapshotSpace(ctx, 2); e != nil {
		return fail(e)
	} else {
		for _, dir := range []string{a.Store.Config.StateDir, a.Store.Config.InstallDir} {
			available, e := a.Store.availableBackupSpace(dir)
			if e != nil {
				return fail(e)
			}
			if available < required+archivedBytes*2 {
				return fail(errors.New("insufficient space for staged archive and restore safety data"))
			}
		}
	}
	stage, _, err := a.Store.stageRestore(ctx, req, b)
	if err != nil {
		return fail(err)
	}
	// Staging is confined to stateDir and validated before any recursive cleanup.
	defer func() {
		checked, e := a.Store.restoreStage(req.ID)
		if e == nil && checked == stage {
			_ = os.RemoveAll(checked)
		}
	}()
	before := a.Store.State
	if err = a.pauseAndStop(ctx, &r, true); err != nil {
		return fail(err)
	}
	if err = a.progress(&r, "safety-backup"); err != nil {
		return fail(err)
	}
	safety, err := a.Store.createSnapshot(ctx, req.ID, true, before.DZSA)
	if err != nil {
		if running {
			if e := a.start(ctx, b, &r, false); e != nil {
				a.Store.State.Paused = true
				err = errors.Join(err, e)
			}
		}
		return fail(err)
	}
	changes, err := a.Store.planRestore(ctx, stage, safety, b)
	if err != nil {
		if running {
			_ = a.start(ctx, b, &r, false)
		}
		return fail(err)
	}
	j := RestoreJournal{Version: Protocol, Operation: req.ID, Target: req.Target, Safety: safety.ID, Before: before, Changes: changes, StopAuthorized: req.Stop}
	journalPath := filepath.Join(a.Store.Config.StateDir, "restore.json")
	if err = WriteJSON(journalPath, j); err != nil {
		if running {
			_ = a.start(ctx, b, &r, false)
		}
		return fail(err)
	}
	if err = a.progress(&r, "restoring"); err == nil {
		err = a.Store.applyRestore(ctx, stage, j)
	}
	if err == nil && running {
		err = a.start(ctx, b, &r, false)
	}
	if err != nil {
		original := err
		recoverCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if e := a.recoverRestore(recoverCtx); e != nil {
			a.Store.State.Paused = true
			r.Recovery = "restore rollback blocked: " + e.Error()
			return fail(errors.Join(original, e))
		}
		r.Recovery = "pre-restore data recovered; safety backup retained"
		if running {
			if e := a.start(recoverCtx, b, &r, false); e != nil {
				a.Store.State.Paused = true
				r.Recovery += "; prior game resume failed: " + e.Error()
			}
		}
		return fail(original)
	}
	j.Committed = true
	if err = WriteJSON(journalPath, j); err != nil {
		a.Store.State.Paused = true
		return fail(fmt.Errorf("restore commit requires recovery: %w", err))
	}
	if err = os.Remove(journalPath); err != nil {
		return fail(err)
	}
	if !running {
		a.Store.State.Paused = true
	}
	a.Store.State.Error = ""
	r.Status = "succeeded"
	r.Backup = &receipt
	r.Recovery = "restore safety backup: " + safety.ID
	if err = a.progress(&r, "complete"); err != nil {
		return fail(err)
	}
	return r
}
