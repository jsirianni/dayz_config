package deploy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Bundle struct {
	Manifest Manifest
	Content  map[string][]byte
}

func Build(ctx context.Context, repo, alias, commit string) (*Bundle, error) {
	// Both launcher settings and file bytes come from the verified commit, not the binary's build tree.
	config, err := Git(ctx, repo, "show", commit+":internal/deploy/targets.json")
	if err != nil {
		return nil, err
	}
	ts, err := Targets(config)
	if err != nil {
		return nil, err
	}
	t, ok := ts[alias]
	if !ok {
		return nil, errors.New("unknown target")
	}
	canonical, err := TargetFor(alias)
	if err != nil {
		return nil, err
	}
	if t.Folder != canonical.Folder || t.Mission != canonical.Mission {
		return nil, errors.New("target source/mission identity cannot be remapped")
	}
	prefix := "servers/current/" + t.Folder + "/"
	archive, err := Git(ctx, repo, "archive", "--format=zip", commit, "--", strings.TrimSuffix(prefix, "/"))
	if err != nil {
		return nil, err
	}
	if len(archive) > MaxPackageBytes {
		return nil, errors.New("source archive exceeds size limit")
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	b := &Bundle{Manifest: Manifest{Version: Protocol, ID: NewID(), Target: alias, Commit: commit, Created: time.Now().UTC(), Launcher: t}, Content: map[string][]byte{}}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !strings.HasPrefix(f.Name, prefix) {
			return nil, errors.New("archive outside server root")
		}
		source := strings.TrimPrefix(f.Name, prefix)
		dest, include := Destination(t, source)
		if !include {
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("managed file is not regular: %s", source)
		}
		data, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		b.Content[source] = data
		b.Manifest.Files = append(b.Manifest.Files, File{Source: source, Destination: dest, SHA256: Digest(data), Size: int64(len(data))})
	}
	sort.Slice(b.Manifest.Files, func(i, j int) bool { return b.Manifest.Files[i].Destination < b.Manifest.Files[j].Destination })
	return b, b.Verify(alias)
}
func readZipFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > MaxFileBytes {
		return nil, errors.New("zip member exceeds size limit")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if len(b) > MaxFileBytes {
		return nil, errors.New("zip member exceeds size limit")
	}
	return b, err
}
func (b *Bundle) Verify(alias string) error {
	m := b.Manifest
	if m.Version != Protocol {
		return VersionError(m.Version)
	}
	if !ValidID(m.ID) || m.Target != alias || !regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(m.Commit) {
		return errors.New("invalid manifest identity")
	}
	canonical, err := TargetFor(alias)
	if err != nil {
		return err
	}
	if m.Launcher.Folder != canonical.Folder || m.Launcher.Mission != canonical.Mission {
		return errors.New("manifest remaps target identity")
	}
	if err = m.Launcher.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	sources := map[string]bool{}
	var total int64
	for _, f := range m.Files {
		dest, ok := Destination(m.Launcher, f.Source)
		if !ok || dest != f.Destination || SafeRelative(dest) != nil {
			return fmt.Errorf("file outside ownership policy: %s", f.Source)
		}
		key := strings.ToLower(dest)
		if seen[key] || sources[f.Source] {
			return fmt.Errorf("duplicate/colliding destination: %s", dest)
		}
		seen[key] = true
		sources[f.Source] = true
		data, ok := b.Content[f.Source]
		if !ok || int64(len(data)) != f.Size || f.Size > MaxFileBytes || Digest(data) != f.SHA256 {
			return fmt.Errorf("file integrity failed: %s", f.Source)
		}
		total += f.Size
	}
	if total > MaxPackageBytes || len(b.Content) != len(m.Files) {
		return errors.New("package size/content mismatch")
	}
	if !sources["serverDZ.cfg"] {
		return errors.New("package missing serverDZ.cfg")
	}
	return nil
}
func (b *Bundle) Encode() ([]byte, error) {
	if err := b.Verify(b.Manifest.Target); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	data, err := json.Marshal(b.Manifest)
	if err != nil {
		return nil, err
	}
	w, err := zw.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err = w.Write(data); err != nil {
		return nil, err
	}
	for _, f := range b.Manifest.Files {
		w, err = zw.Create("files/" + f.Source)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(b.Content[f.Source]); err != nil {
			return nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	if out.Len() > MaxPackageBytes {
		return nil, errors.New("encoded package too large")
	}
	return out.Bytes(), nil
}
func Decode(data []byte, alias string) (*Bundle, error) {
	if len(data) > MaxPackageBytes {
		return nil, errors.New("package too large")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	b := &Bundle{Content: map[string][]byte{}}
	names := map[string]bool{}
	var total int64
	for _, f := range zr.File {
		if names[f.Name] || !f.Mode().IsRegular() {
			return nil, errors.New("duplicate or non-regular zip member")
		}
		names[f.Name] = true
		raw, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		total += int64(len(raw))
		if total > MaxPackageBytes {
			return nil, errors.New("expanded package too large")
		}
		if f.Name == "manifest.json" {
			if err = json.Unmarshal(raw, &b.Manifest); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(f.Name, "files/") {
			p := strings.TrimPrefix(f.Name, "files/")
			if err = SafeRelative(p); err != nil {
				return nil, err
			}
			b.Content[p] = raw
		} else {
			return nil, errors.New("unexpected zip member")
		}
	}
	if err = b.Verify(alias); err != nil {
		return nil, err
	}
	return b, nil
}
func (b *Bundle) Extract(root string) error {
	for _, f := range b.Manifest.Files {
		p, err := SafeJoin(root, f.Source)
		if err != nil {
			return err
		}
		if err = AtomicWrite(p, b.Content[f.Source], 0600); err != nil {
			return err
		}
	}
	return nil
}
func LoadBundle(p, alias string) (*Bundle, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxPackageBytes {
		return nil, errors.New("package too large")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return Decode(data, alias)
}
func packagePath(stateDir, id string) string { return filepath.Join(stateDir, "releases", id+".zip") }
