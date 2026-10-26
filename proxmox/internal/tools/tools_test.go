package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecHelper(t *testing.T) {
	if os.Getenv("PVECLI_PROCESS_HELPER") != "1" {
		return
	}
	if os.Getenv("PVECLI_PROCESS_SLEEP") == "1" {
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}
func TestManagedProcessAndCancellation(t *testing.T) {
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	env := append(os.Environ(), "PVECLI_PROCESS_HELPER=1")
	if e = (Exec{}).Run(ctx, Invocation{Binary: binary, Args: []string{"-test.run=^TestExecHelper$"}, Env: env}, &bytes.Buffer{}, &bytes.Buffer{}); e != nil {
		t.Fatal(e)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	env = append(env, "PVECLI_PROCESS_SLEEP=1")
	start := time.Now()
	if e = (Exec{}).Run(ctx, Invocation{Binary: binary, Args: []string{"-test.run=^TestExecHelper$"}, Env: env}, &bytes.Buffer{}, &bytes.Buffer{}); e == nil {
		t.Fatal("cancellation succeeded unexpectedly")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("process cancellation stalled")
	}
}

func TestChecksumsRejectCorruption(t *testing.T) {
	b := []byte("valid binary")
	h := sha256.Sum256(b)
	want := hex.EncodeToString(h[:])
	got, e := Checksum([]byte(want+"  packer.zip\n"), "packer.zip")
	if e != nil || got != want {
		t.Fatal(got, e)
	}
	if e = Verify(b, got); e != nil {
		t.Fatal(e)
	}
	if e = Verify([]byte("tampered"), got); e == nil {
		t.Fatal("accepted corruption")
	}
	if _, e = Checksum([]byte("abc  packer.zip"), "packer.zip"); e == nil {
		t.Fatal("invalid hash")
	}
}
func TestExtractOnlyExpectedBinary(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("../escape")
	_, _ = f.Write([]byte("bad"))
	_ = z.Close()
	dir := t.TempDir()
	if e := extractBinary(b.Bytes(), "packer", filepath.Join(dir, "packer")); e == nil {
		t.Fatal("accepted wrong archive")
	}
	if _, e := os.Stat(filepath.Join(filepath.Dir(dir), "escape")); !os.IsNotExist(e) {
		t.Fatal("zip traversal")
	}
}
func TestBuildEnvironmentDoesNotInheritPackerOverrides(t *testing.T) {
	t.Setenv("PACKER_LOG", "1")
	t.Setenv("PKR_VAR_token", "bad")
	t.Setenv("PROXMOX_TOKEN", "bad")
	env := CleanEnv(map[string]string{"PROXMOX_TOKEN": "selected"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "PKR_VAR_token") || strings.Contains(joined, "PACKER_LOG=1") || !strings.Contains(joined, "PROXMOX_TOKEN=selected") {
		t.Fatal("unsafe inherited overrides")
	}
}
