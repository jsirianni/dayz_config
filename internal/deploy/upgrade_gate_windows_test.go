//go:build windows

package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestUpgradeInboxGatePreservesInspectionButRejectsUploads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inbox")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := upgradeSetDACL(dir, before.String()); e != nil {
			t.Error(e)
		}
	})
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if err = gateUpgradeInbox(dir, user.User.Sid.String()); err != nil {
		t.Fatal(err)
	}
	if entries, e := os.ReadDir(dir); e != nil || len(entries) != 0 {
		t.Fatal("gate denied read-only quiescence inspection", entries, e)
	}
	if err = os.WriteFile(filepath.Join(dir, "new-request.json"), []byte("{}"), 0600); err == nil {
		t.Fatal("gate allowed an upload")
	}
	if err = upgradeSetDACL(dir, before.String()); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "new-request.json"), []byte("{}"), 0600); err != nil {
		t.Fatal("restored policy still blocks uploads", err)
	}
}
