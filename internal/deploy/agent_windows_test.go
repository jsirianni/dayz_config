//go:build windows

package deploy

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAtomicReplaceRetriesTransientWindowsReader(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := AtomicWrite(p, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- AtomicWrite(p, []byte("new"), 0600) }()
	time.Sleep(80 * time.Millisecond)
	windows.CloseHandle(handle)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil || string(raw) != "new" {
		t.Fatal("atomic replacement failed", err)
	}
}
