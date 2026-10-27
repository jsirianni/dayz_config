// Package snapshot creates and verifies streaming, content-addressed backup archives.
package snapshot

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const MaxEntries = 100000
const MaxDataBytes int64 = 1 << 40

type Entry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256,omitempty"`
	Directory bool   `json:"directory,omitempty"`
}
type Manifest struct {
	Version     int       `json:"version"`
	ID          string    `json:"id"`
	Target      string    `json:"target"`
	Created     time.Time `json:"created"`
	Revision    string    `json:"revision"`
	GameVersion string    `json:"gameVersion,omitempty"`
	Mods        []string  `json:"mods"`
	Managed     []string  `json:"managed"`
	Entries     []Entry   `json:"entries"`
}
type Rules struct {
	Check func(string, string) (string, error)
	Allow func(string) bool
}

func Scan(ctx context.Context, root string, roots []string, rules Rules) ([]Entry, int64, error) {
	var entries []Entry
	var bytes int64
	seen := map[string]bool{}
	for _, relative := range roots {
		start, err := rules.Check(root, relative)
		if err != nil {
			return nil, 0, err
		}
		if _, err = os.Lstat(start); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, 0, err
		}
		err = filepath.WalkDir(start, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			// Check links even at excluded paths: never silently follow a linked root.
			if _, e = rules.Check(root, rel); e != nil {
				return e
			}
			if !rules.Allow(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[strings.ToLower(rel)] {
				return fmt.Errorf("duplicate/case-colliding backup path %s", rel)
			}
			seen[strings.ToLower(rel)] = true
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("nonregular backup file %s", rel)
			}
			entries = append(entries, Entry{Path: rel, Size: info.Size(), Directory: info.IsDir()})
			if info.IsDir() {
				entries[len(entries)-1].Size = 0
			} else {
				if info.Size() < 0 || info.Size() > MaxDataBytes-bytes {
					return errors.New("backup data exceeds size bound")
				}
				bytes += info.Size()
			}
			if len(entries) > MaxEntries {
				return errors.New("backup entry limit exceeded")
			}
			return nil
		})
		if err != nil {
			return nil, 0, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, bytes, nil
}

func copyContext(ctx context.Context, w io.Writer, r io.Reader) (int64, error) {
	b := make([]byte, 128<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := r.Read(b)
		if n > 0 {
			written, e := w.Write(b[:n])
			total += int64(written)
			if e != nil {
				return total, e
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func Create(ctx context.Context, root, destination string, manifest Manifest, roots []string, rules Rules) (Manifest, error) {
	entries, _, err := Scan(ctx, root, roots, rules)
	if err != nil {
		return manifest, err
	}
	manifest.Entries = entries
	manifest.Version = 1
	f, err := os.CreateTemp(filepath.Dir(destination), ".snapshot-*")
	if err != nil {
		return manifest, err
	}
	defer os.Remove(f.Name())
	z := zip.NewWriter(f)
	for i := range manifest.Entries {
		entry := &manifest.Entries[i]
		source, e := rules.Check(root, entry.Path)
		if e != nil {
			err = e
			break
		}
		header := &zip.FileHeader{Name: entry.Path, Method: zip.Deflate}
		header.SetMode(0600)
		if entry.Directory {
			header.Name += "/"
			header.SetMode(os.ModeDir | 0700)
		}
		w, e := z.CreateHeader(header)
		if e != nil {
			err = e
			break
		}
		if entry.Directory {
			continue
		}
		input, e := os.Open(source)
		if e != nil {
			err = e
			break
		}
		hash := sha256.New()
		n, e := copyContext(ctx, io.MultiWriter(w, hash), io.LimitReader(input, entry.Size+1))
		input.Close()
		if e != nil {
			err = e
			break
		}
		if n != entry.Size {
			err = fmt.Errorf("file changed during backup: %s", entry.Path)
			break
		}
		entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	if err == nil {
		w, e := z.Create("manifest.json")
		if e == nil {
			e = json.NewEncoder(w).Encode(manifest)
		}
		err = e
	}
	err = errors.Join(err, z.Close(), f.Sync(), f.Close())
	if err != nil {
		return manifest, err
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		return manifest, errors.New("backup destination exists or cannot be inspected")
	}
	if err = os.Rename(f.Name(), destination); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func Hash(ctx context.Context, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = copyContext(ctx, h, f)
	return hex.EncodeToString(h.Sum(nil)), err
}
