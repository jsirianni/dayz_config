package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jsirianni/dayz_config/internal/snapshot"
)

func (r *Remote) ListBackups(alias string) ([]BackupReceipt, error) {
	if _, err := TargetFor(alias); err != nil {
		return nil, err
	}
	if err := r.rejectRemoteLinks(r.remote("snapshots")); err != nil {
		return nil, err
	}
	var receipts []BackupReceipt
	for _, sub := range []string{"snapshots", "snapshots/safety"} {
		if err := r.rejectRemoteLinks(r.remote(sub)); err != nil {
			return nil, err
		}
		entries, err := r.SFTP.ReadDir(r.remote(sub))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(entries) > 20000 {
			return nil, errors.New("backup inventory exceeds limit")
		}
		for _, file := range entries {
			id := strings.TrimSuffix(file.Name(), ".json")
			if !ValidID(id) || !strings.HasSuffix(file.Name(), ".json") || !file.Mode().IsRegular() {
				continue
			}
			var receipt BackupReceipt
			if err = r.ReadJSON(sub+"/"+file.Name(), &receipt); err != nil {
				return nil, err
			}
			if receipt.ID != id || receipt.Target != alias || receipt.Safety != (sub == "snapshots/safety") {
				return nil, errors.New("backup receipt identity mismatch")
			}
			receipts = append(receipts, receipt)
		}
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].Created.After(receipts[j].Created) })
	return receipts, nil
}

func (r *Remote) backupReceipt(alias, id string) (BackupReceipt, string, error) {
	var receipt BackupReceipt
	if !ValidID(id) {
		return receipt, "", errors.New("invalid backup ID")
	}
	rel := "snapshots/" + id
	if err := r.ReadJSON(rel+".json", &receipt); os.IsNotExist(err) {
		rel = "snapshots/safety/" + id
		if err = r.ReadJSON(rel+".json", &receipt); err != nil {
			return receipt, "", err
		}
	} else if err != nil {
		return receipt, "", err
	}
	if receipt.ID != id || receipt.Target != alias || receipt.Safety != strings.Contains(rel, "/safety/") {
		return receipt, "", errors.New("backup receipt identity mismatch")
	}
	if err := r.rejectRemoteLinks(r.remote("snapshots")); err != nil {
		return receipt, "", err
	}
	return receipt, r.remote(rel + ".zip"), nil
}

func (r *Remote) VerifyBackup(ctx context.Context, alias, id string) (BackupReceipt, error) {
	receipt, file, err := r.backupReceipt(alias, id)
	if err != nil {
		return receipt, err
	}
	info, err := r.SFTP.Lstat(file)
	if err != nil {
		return receipt, err
	}
	if !info.Mode().IsRegular() || info.Size() != receipt.Bytes {
		return receipt, errors.New("backup archive size/type mismatch")
	}
	f, err := r.SFTP.Open(file)
	if err != nil {
		return receipt, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, contextReader{ctx, io.NewSectionReader(f, 0, info.Size())}); err != nil {
		return receipt, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != receipt.SHA256 {
		return receipt, errors.New("backup archive checksum mismatch")
	}
	target, _ := TargetFor(alias)
	rules := backupRules(target)
	// Remote verification validates logical paths; extraction on Windows uses SafeJoin.
	rules.Check = func(root, rel string) (string, error) {
		if err := SafeRelative(rel); err != nil {
			return "", err
		}
		return filepath.Join(root, filepath.FromSlash(rel)), nil
	}
	manifest, err := snapshot.VerifyReader(ctx, f, info.Size(), alias, rules)
	if err != nil {
		return receipt, err
	}
	if manifest.ID != id {
		return receipt, errors.New("backup manifest ID mismatch")
	}
	receipt.Manifest = manifest
	return receipt, nil
}

func (r *Remote) DownloadBackup(ctx context.Context, alias, id, destination string) (BackupReceipt, error) {
	receipt, err := r.VerifyBackup(ctx, alias, id)
	if err != nil {
		return receipt, err
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		return receipt, errors.New("download destination exists or cannot be inspected")
	}
	directory := filepath.Dir(destination)
	if err = os.MkdirAll(directory, 0700); err != nil {
		return receipt, err
	}
	temp, err := os.CreateTemp(directory, ".backup-download-*")
	if err != nil {
		return receipt, err
	}
	defer os.Remove(temp.Name())
	if err = protectPrivateFile(temp.Name()); err != nil {
		temp.Close()
		return receipt, err
	}
	_, source, err := r.backupReceipt(alias, id)
	if err != nil {
		temp.Close()
		return receipt, err
	}
	f, err := r.SFTP.Open(source)
	if err != nil {
		temp.Close()
		return receipt, err
	}
	defer f.Close()
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(temp, hash), contextReader{ctx, io.LimitReader(f, receipt.Bytes+1)})
	err = errors.Join(copyErr, temp.Sync(), temp.Close())
	if err != nil {
		return receipt, err
	}
	if n != receipt.Bytes || hex.EncodeToString(hash.Sum(nil)) != receipt.SHA256 {
		return receipt, errors.New("download checksum mismatch")
	}
	// Link publishes exclusively on both supported client platforms; it cannot
	// overwrite a file created after the initial destination check.
	if err = os.Link(temp.Name(), destination); err != nil {
		return receipt, err
	}
	return receipt, nil
}
