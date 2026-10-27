package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type startupLogGame struct {
	*hardFake
	logPath   string
	failNext  bool
	interrupt bool
}

func (g *startupLogGame) Healthy(ctx context.Context, target Target) (*DZSAReport, error) {
	if g.failNext {
		g.failNext = false
		if err := os.WriteFile(g.logPath, []byte("startup diagnostic retained"), 0600); err != nil {
			return nil, err
		}
		if g.interrupt {
			panic("fixture agent interrupted during restored startup")
		}
		g.alive = false
		return nil, errors.New("fixture readiness failed after writing a startup log")
	}
	return g.hardFake.Healthy(ctx, target)
}

func TestInterruptedRestoreRecoveryPreservesStartupLogWithoutLaunching(t *testing.T) {
	a, g, _ := backupFixture(t)
	root := a.Store.Config.InstallDir
	writeTest(t, root, "profiles/newmod/runtime.json", "archived mod data")
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" || created.Backup == nil {
		t.Fatal(created)
	}
	for _, name := range []string{"profiles/newmod/runtime.json", "profiles/newmod"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	writeTest(t, root, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "pre-interruption world")
	a.Game = &startupLogGame{hardFake: g, logPath: filepath.Join(root, "profiles/newmod/runtime.log"), failNext: true, interrupt: true}
	launches := g.launches
	req := backupRequest(a, "backup-restore", created.Backup.ID, true)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("agent interruption was not reached")
			}
		}()
		_ = a.Process(context.Background(), req)
	}()
	if err := a.recoverRestore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if value := readTest(t, root, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "pre-interruption world" {
		t.Fatal("world preimage was not recovered", value)
	}
	if value := readTest(t, root, "profiles/newmod/runtime.log"); value != "startup diagnostic retained" {
		t.Fatal("excluded startup log changed", value)
	}
	if _, err := os.Stat(filepath.Join(root, "profiles/newmod/runtime.json")); !os.IsNotExist(err) {
		t.Fatal("introduced runtime data remains", err)
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "restore.json")); !os.IsNotExist(err) {
		t.Fatal("journal was not recovered", err)
	}
	if g.alive || !a.Store.State.Paused || a.Store.State.PID != 0 || g.launches != launches+1 {
		t.Fatal("interrupted recovery unexpectedly launched a game", a.Store.State)
	}
	if err := a.recoverRestore(context.Background()); err != nil || g.launches != launches+1 {
		t.Fatal("repeated recovery launched a duplicate", err)
	}
}

func TestRestoreRollbackPreservesStartupLogsAndRecoversAllPreimages(t *testing.T) {
	a, g, _ := backupFixture(t)
	root := a.Store.Config.InstallDir
	writeTest(t, root, "profiles/newmod/nested/runtime.json", "archived mod data")
	if err := os.MkdirAll(filepath.Join(root, "profiles/newempty/nested"), 0700); err != nil {
		t.Fatal(err)
	}
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" || created.Backup == nil {
		t.Fatal(created)
	}
	for _, name := range []string{"profiles/newmod/nested/runtime.json", "profiles/newmod/nested", "profiles/newmod", "profiles/newempty/nested", "profiles/newempty"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	writeTest(t, root, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "pre-restore world")
	writeTest(t, root, "profiles/mod/runtime.json", "pre-restore mod data")
	logPath := filepath.Join(root, "profiles/newmod/nested/runtime.log")
	a.Game = &startupLogGame{hardFake: g, logPath: logPath, failNext: true}
	launches := g.launches
	req := backupRequest(a, "backup-restore", created.Backup.ID, true)
	result := a.Process(context.Background(), req)
	if result.Status != "failed" || !strings.Contains(result.Recovery, "pre-restore data recovered") {
		t.Fatal("startup log blocked rollback", result)
	}
	if value := readTest(t, root, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "pre-restore world" {
		t.Fatal("world preimage was not recovered", value)
	}
	if value := readTest(t, root, "profiles/mod/runtime.json"); value != "pre-restore mod data" {
		t.Fatal("mod preimage was not recovered", value)
	}
	if value := readTest(t, root, "profiles/newmod/nested/runtime.log"); value != "startup diagnostic retained" {
		t.Fatal("excluded startup log changed", value)
	}
	for _, name := range []string{"profiles/newmod/nested/runtime.json", "profiles/newempty"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatal("introduced runtime data or empty directories remain", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "restore.json")); !os.IsNotExist(err) {
		t.Fatal("restore journal still blocks startup", err)
	}
	if !g.alive || a.Store.State.Paused || g.launches != launches+2 {
		t.Fatal("previous release did not resume exactly once", g.launches, a.Store.State)
	}
	if err := a.recoverRestore(context.Background()); err != nil || g.launches != launches+2 {
		t.Fatal("repeated recovery changed the running release", err)
	}
	if _, _, err := a.Store.verifiedSnapshot(context.Background(), req.ID, true); err != nil {
		t.Fatal("verified restore safety backup was lost", err)
	}
}

func TestRestoreExcludesCaseRenamedManagedSettingsRemovedByNewRelease(t *testing.T) {
	s := testStore(t)
	g := &hardFake{fakeGame: &fakeGame{store: s}}
	a := &Agent{Store: s, Game: g}
	oldPath := "profiles/ExpansionMod/Settings/obsolete.json"
	archivedPath := "profiles/ExpansionMod/Settings/Obsolete.json"
	old := testBundle(t, map[string]string{oldPath: `{"obsolete":true}`})
	if r := a.Process(context.Background(), submitTest(t, s, old, true)); r.Status != "succeeded" {
		t.Fatal(r)
	}
	writeTest(t, s.Config.InstallDir, "profiles/mod/runtime.json", "archived runtime")
	if err := os.Rename(filepath.Join(s.Config.InstallDir, oldPath), filepath.Join(s.Config.InstallDir, archivedPath)); err != nil {
		t.Fatal(err)
	}
	// A stopped backup keeps the archive's actual case without a resume reapplying
	// the old managed spelling on the Linux client-test filesystem.
	g.alive = false
	s.State.PID, s.State.ProcessToken = 0, ""
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", false))
	if created.Status != "succeeded" || created.Backup == nil {
		t.Fatal(created)
	}
	found := false
	for _, entry := range created.Backup.Manifest.Entries {
		found = found || entry.Path == archivedPath
	}
	if !found {
		t.Fatal("fixture did not archive the case-renamed managed file")
	}
	if err := os.Rename(filepath.Join(s.Config.InstallDir, archivedPath), filepath.Join(s.Config.InstallDir, oldPath)); err != nil {
		t.Fatal(err)
	}
	currentPath := "profiles/ExpansionMod/Settings/current.json"
	next := testBundle(t, map[string]string{currentPath: `{"current":true}`})
	if r := a.Process(context.Background(), submitTest(t, s, next, true)); r.Status != "succeeded" {
		t.Fatal(r)
	}
	if _, err := os.Stat(filepath.Join(s.Config.InstallDir, oldPath)); !os.IsNotExist(err) {
		t.Fatal("new release did not remove obsolete managed file", err)
	}
	writeTest(t, s.Config.InstallDir, "profiles/mod/runtime.json", "later runtime")
	result := a.Process(context.Background(), backupRequest(a, "backup-restore", created.Backup.ID, true))
	if result.Status != "succeeded" {
		t.Fatal(result)
	}
	for _, name := range []string{oldPath, archivedPath} {
		if _, err := os.Stat(filepath.Join(s.Config.InstallDir, name)); !os.IsNotExist(err) {
			t.Fatal("archived obsolete managed setting was resurrected", name, err)
		}
	}
	if value := readTest(t, s.Config.InstallDir, currentPath); value != `{"current":true}` {
		t.Fatal("current managed configuration changed", value)
	}
	if value := readTest(t, s.Config.InstallDir, "profiles/mod/runtime.json"); value != "archived runtime" {
		t.Fatal("runtime data was not restored", value)
	}
	if s.State.Installed.ID != next.Manifest.ID || !g.alive {
		t.Fatal("installed release changed during restore")
	}
}
