package snapshot

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testRules() Rules {
	return Rules{Allow: func(s string) bool { return s == "profiles" || strings.HasPrefix(s, "profiles/") }, Check: func(root, rel string) (string, error) {
		if filepath.IsAbs(rel) || strings.ContainsAny(rel, "\\:") || filepath.ToSlash(filepath.Clean(rel)) != rel || strings.HasPrefix(rel, "../") {
			return "", errors.New("unsafe path")
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		for current := p; current != root; current = filepath.Dir(current) {
			if info, e := os.Lstat(current); e == nil && info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("link")
			}
		}
		return p, nil
	}}
}

func TestStreamingSnapshotRoundTripAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "profiles", "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	original := bytes.Repeat([]byte("world-data\x00"), 20000)
	if err := os.WriteFile(filepath.Join(root, "profiles", "world.bin"), original, 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.zip")
	manifest := Manifest{ID: "one", Target: "nasdara", Created: time.Now().UTC()}
	built, err := Create(context.Background(), root, archive, manifest, []string{"profiles"}, testRules())
	if err != nil || len(built.Entries) != 3 {
		t.Fatal(built, err)
	}
	if _, err = Verify(context.Background(), archive, "other", testRules()); err == nil {
		t.Fatal("wrong target accepted")
	}
	dest := t.TempDir()
	if _, err = Extract(context.Background(), archive, "nasdara", dest, testRules(), nil); err != nil {
		t.Fatal(err)
	}
	found, err := os.ReadFile(filepath.Join(dest, "profiles", "world.bin"))
	if err != nil || !bytes.Equal(found, original) {
		t.Fatal("world bytes changed", err)
	}
	if info, e := os.Stat(filepath.Join(dest, "profiles", "empty")); e != nil || !info.IsDir() {
		t.Fatal("empty directory lost", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Create(ctx, root, filepath.Join(t.TempDir(), "cancelled.zip"), manifest, []string{"profiles"}, testRules()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func maliciousArchive(t *testing.T, names []string, hash string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "unsafe.zip")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	m := Manifest{Version: 1, Target: "nasdara", ID: "unsafe"}
	for _, name := range names {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = w.Write([]byte("data"))
		m.Entries = append(m.Entries, Entry{Path: name, Size: 4, SHA256: hash})
	}
	w, _ := z.Create("manifest.json")
	_ = json.NewEncoder(w).Encode(m)
	_ = z.Close()
	_ = f.Close()
	return file
}

func TestRejectUnsafeArchivesAndBadContent(t *testing.T) {
	sum := sha256.Sum256([]byte("data"))
	hash := hex.EncodeToString(sum[:])
	for _, names := range [][]string{{"../escape"}, {"/profiles/absolute"}, {"profiles/a:stream"}, {"profiles\\escape"}, {"profiles/File", "profiles/file"}, {"profiles/one", "profiles/one"}, {"credentials.txt"}} {
		archive := maliciousArchive(t, names, hash)
		if _, err := Verify(context.Background(), archive, "nasdara", testRules()); err == nil {
			t.Fatal("accepted", names)
		}
	}
	archive := maliciousArchive(t, []string{"profiles/data"}, strings.Repeat("0", 64))
	if _, err := Verify(context.Background(), archive, "nasdara", testRules()); err == nil {
		t.Fatal("content hash mismatch accepted")
	}
	b, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(archive, b[:len(b)/2], 0600)
	if _, err = Verify(context.Background(), archive, "nasdara", testRules()); err == nil {
		t.Fatal("truncated archive accepted")
	}
}
