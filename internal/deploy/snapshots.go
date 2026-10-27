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

const BackupCapability = "backup.v1"

type BackupReceipt struct {
	ID       string            `json:"id"`
	Target   string            `json:"target"`
	Created  time.Time         `json:"created"`
	SHA256   string            `json:"sha256"`
	Bytes    int64             `json:"bytes"`
	Safety   bool              `json:"safety"`
	Manifest snapshot.Manifest `json:"manifest"`
}

func backupRoots(t Target, managed *Manifest) []string {
	roots := []string{"mpmissions/" + t.Mission, "profiles", "serverDZ.cfg"}
	if managed != nil {
		for _, file := range managed.Files {
			found := false
			for _, root := range roots {
				found = found || file.Destination == root || strings.HasPrefix(file.Destination, root+"/")
			}
			if !found {
				roots = append(roots, file.Destination)
			}
		}
	}
	sort.Strings(roots)
	return roots
}

func backupRules(t Target) snapshot.Rules {
	return snapshot.Rules{Check: SafeJoin, Allow: func(rel string) bool {
		if SafeRelative(rel) != nil {
			return false
		}
		if rel == "serverDZ.cfg" || rel == "dayz-service.bat" || rel == "dayz-service-manager.bat" {
			return true
		}
		if rel != "profiles" && !strings.HasPrefix(rel, "profiles/") && rel != "mpmissions/"+t.Mission && !strings.HasPrefix(rel, "mpmissions/"+t.Mission+"/") {
			return false
		}
		for _, part := range strings.Split(strings.ToLower(rel), "/") {
			switch part {
			case "logs", "log", "cache", "caches", "dumps", ".ssh", "steamapps", "workshop", ".git", ".deploy":
				return false
			}
		}
		switch strings.ToLower(path.Ext(rel)) {
		case ".log", ".rpt", ".adm", ".mdmp", ".dmp", ".tmp":
			return false
		}
		return true
	}}
}

func snapshotPath(c HostConfig, id string, safety bool) (string, error) {
	if !ValidID(id) {
		return "", errors.New("invalid backup ID")
	}
	prefix := "snapshots/"
	if safety {
		prefix += "safety/"
	}
	return SafeJoin(c.StateDir, prefix+id+".zip")
}
func receiptPath(c HostConfig, id string, safety bool) (string, error) {
	p, e := snapshotPath(c, id, safety)
	return strings.TrimSuffix(p, ".zip") + ".json", e
}

func (s *Store) createSnapshot(ctx context.Context, id string, safety bool, provenance *DZSAReport) (BackupReceipt, error) {
	var receipt BackupReceipt
	if s.State.Installed == nil {
		return receipt, errors.New("no installed release")
	}
	target := s.State.Installed.Launcher
	roots := backupRoots(target, s.State.Installed)
	archive, err := snapshotPath(s.Config, id, safety)
	if err != nil {
		return receipt, err
	}
	if err = os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		return receipt, err
	}
	if err = protectPrivateDirectory(filepath.Dir(archive)); err != nil {
		return receipt, err
	}
	manifest := snapshot.Manifest{ID: id, Target: s.Config.Target, Created: time.Now().UTC(), Revision: s.State.Installed.Commit}
	for _, mod := range target.Mods {
		manifest.Mods = append(manifest.Mods, mod.ID)
	}
	for _, file := range s.State.Installed.Files {
		manifest.Managed = append(manifest.Managed, file.Destination)
	}
	if provenance != nil {
		manifest.GameVersion = provenance.Server.Version
	}
	if manifest, err = snapshot.Create(ctx, s.Config.InstallDir, archive, manifest, roots, backupRules(target)); err != nil {
		return receipt, err
	}
	if _, err = snapshot.Verify(ctx, archive, s.Config.Target, backupRules(target)); err != nil {
		return receipt, err
	}
	hash, err := snapshot.Hash(ctx, archive)
	if err != nil {
		return receipt, err
	}
	info, err := os.Stat(archive)
	if err != nil {
		return receipt, err
	}
	receipt = BackupReceipt{ID: id, Target: s.Config.Target, Created: manifest.Created, SHA256: hash, Bytes: info.Size(), Safety: safety, Manifest: manifest}
	metadata, _ := receiptPath(s.Config, id, safety)
	err = WriteJSON(metadata, receipt)
	return receipt, err
}

func (s *Store) verifiedSnapshot(ctx context.Context, id string, safety bool) (BackupReceipt, string, error) {
	var receipt BackupReceipt
	metadata, err := receiptPath(s.Config, id, safety)
	if err != nil {
		return receipt, "", err
	}
	if err = ReadJSON(metadata, &receipt); err != nil {
		return receipt, "", err
	}
	if receipt.Target != s.Config.Target || receipt.ID != id || receipt.Safety != safety {
		return receipt, "", errors.New("backup receipt identity mismatch")
	}
	archive, err := snapshotPath(s.Config, id, safety)
	if err != nil {
		return receipt, "", err
	}
	hash, err := snapshot.Hash(ctx, archive)
	if err != nil {
		return receipt, "", err
	}
	if hash != receipt.SHA256 {
		return receipt, "", errors.New("backup archive checksum mismatch")
	}
	target, _ := TargetFor(s.Config.Target)
	if s.State.Installed != nil {
		target = s.State.Installed.Launcher
	}
	manifest, err := snapshot.Verify(ctx, archive, s.Config.Target, backupRules(target))
	if err != nil {
		return receipt, "", err
	}
	if manifest.ID != receipt.ID || manifest.Revision != receipt.Manifest.Revision {
		return receipt, "", errors.New("backup manifest identity mismatch")
	}
	receipt.Manifest = manifest
	return receipt, archive, nil
}

func (s *Store) snapshotSpace(ctx context.Context, multiplier uint64) (uint64, error) {
	if s.State.Installed == nil {
		return 0, errors.New("no installed release")
	}
	_, size, err := snapshot.Scan(ctx, s.Config.InstallDir, backupRoots(s.State.Installed.Launcher, s.State.Installed), backupRules(s.State.Installed.Launcher))
	if err != nil {
		return 0, err
	}
	required := uint64(size)*multiplier + 64<<20
	for _, dir := range []string{s.Config.StateDir, s.Config.InstallDir} {
		available, e := s.availableBackupSpace(dir)
		if e != nil {
			return required, e
		}
		if available < required {
			return required, fmt.Errorf("insufficient disk space for backup/restore staging (need %d bytes in %s)", required, dir)
		}
	}
	return required, nil
}

func (s *Store) availableBackupSpace(dir string) (uint64, error) {
	if s.AvailableSpace != nil {
		return s.AvailableSpace(dir)
	}
	return freeSpace(dir)
}

func (s *Store) retainSnapshots() error {
	dir, err := SafeJoin(s.Config.StateDir, "snapshots")
	if err != nil {
		return err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var receipts []BackupReceipt
	for _, file := range files {
		id := strings.TrimSuffix(file.Name(), ".json")
		if !ValidID(id) || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		var r BackupReceipt
		metadata, e := SafeJoin(s.Config.StateDir, "snapshots/"+file.Name())
		if e != nil {
			return e
		}
		if ReadJSON(metadata, &r) == nil && !r.Safety && r.ID == id && r.Target == s.Config.Target && r.Manifest.Version == 1 && r.Manifest.ID == id {
			receipts = append(receipts, r)
		}
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].Created.After(receipts[j].Created) })
	for i, receipt := range receipts {
		if i < 10 {
			continue
		}
		id := receipt.ID
		for _, ext := range []string{".zip", ".json"} {
			p, e := SafeJoin(s.Config.StateDir, "snapshots/"+id+ext)
			if e != nil {
				return e
			}
			if e = os.Remove(p); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}
