package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveContainsExecutableDocsAndChecksums(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pvecli.zip")
	files := map[string][]byte{"pvecli": []byte("binary"), "README.md": []byte("docs"), "examples/config.example.yaml": []byte("example")}
	if e := writeArchive(path, files, "pvecli"); e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	manifest := ""
	found := map[string]bool{}
	for _, f := range z.File {
		found[f.Name] = true
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if f.Name == "SHA256SUMS" {
			manifest = string(b)
		}
		if f.Name == "pvecli" && f.Mode().Perm() != 0755 {
			t.Fatal("executable bit lost")
		}
	}
	for k, b := range files {
		if !found[k] {
			t.Fatal(k)
		}
		if k != "SHA256SUMS" && !strings.Contains(manifest, fmt.Sprintf("%x  %s", sha256.Sum256(b), k)) {
			t.Fatal("missing checksum", k)
		}
	}
}
