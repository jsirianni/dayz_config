package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func backupFixture(t *testing.T) (*Agent, *hardFake, *Bundle) {
	t.Helper()
	s := testStore(t)
	g := &hardFake{fakeGame: &fakeGame{store: s}}
	a := &Agent{Store: s, Game: g}
	b := testBundle(t, map[string]string{"mpmissions/dayzOffline.nasdara/db/types.xml": "<types/>"})
	if r := a.Process(context.Background(), submitTest(t, s, b, true)); r.Status != "succeeded" {
		t.Fatal(r)
	}
	writeTest(t, s.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "original world")
	writeTest(t, s.Config.InstallDir, "profiles/mod/runtime.json", "original mod data")
	writeTest(t, s.Config.InstallDir, "profiles/crash_test.log", "sensitive log excluded")
	return a, g, b
}

func backupRequest(a *Agent, action, id string, stop bool) Request {
	s := a.Store.State
	r := Request{Version: Protocol, ID: NewID(), Target: a.Store.Config.Target, Action: action, Backup: id, Stop: stop, ExpectedPID: s.PID, ExpectedToken: s.ProcessToken}
	if s.Installed != nil {
		r.ExpectedRelease = s.Installed.ID
	}
	return r
}

func TestBackupStopRequiredThenRestoresCurrentManagedConfig(t *testing.T) {
	a, g, b := backupFixture(t)
	beforeUpdates := strings.Count(strings.Join(g.events, ","), "update")
	refused := a.Process(context.Background(), backupRequest(a, "backup-create", "", false))
	if refused.Status != "failed" || g.hardStops != 0 || !g.alive {
		t.Fatal(refused)
	}
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" || created.Backup == nil || !g.alive {
		t.Fatal(created)
	}
	if strings.Count(strings.Join(g.events, ","), "update") != beforeUpdates {
		t.Fatal("backup ran Steam updates")
	}
	for _, file := range created.Backup.Manifest.Entries {
		if strings.HasSuffix(file.Path, ".log") {
			t.Fatal("logs entered backup")
		}
	}
	writeTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "new world")
	writeTest(t, a.Store.Config.InstallDir, "profiles/mod/obsolete.json", "new obsolete data")
	// Install a new desired config, retaining the same game release identity for this test.
	desired := "hostname = \"Current desired hostname\";\n"
	b.Content["serverDZ.cfg"] = []byte(desired)
	for i := range b.Manifest.Files {
		if b.Manifest.Files[i].Source == "serverDZ.cfg" {
			b.Manifest.Files[i].SHA256 = Digest([]byte(desired))
			b.Manifest.Files[i].Size = int64(len(desired))
		}
	}
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err = AtomicWrite(packagePath(a.Store.Config.StateDir, b.Manifest.ID), raw, 0600); err != nil {
		t.Fatal(err)
	}
	a.Store.State.Installed = &b.Manifest
	restored := a.Process(context.Background(), backupRequest(a, "backup-restore", created.Backup.ID, true))
	if restored.Status != "succeeded" || !g.alive {
		t.Fatal(restored)
	}
	if value := readTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "original world" {
		t.Fatal(value)
	}
	if value := readTest(t, a.Store.Config.InstallDir, "serverDZ.cfg"); value != desired {
		t.Fatal("archived managed config replaced desired bytes", value)
	}
	if _, err = os.Stat(filepath.Join(a.Store.Config.InstallDir, "profiles/mod/obsolete.json")); !os.IsNotExist(err) {
		t.Fatal("obsolete runtime file remains", err)
	}
	if value := readTest(t, a.Store.Config.InstallDir, "profiles/crash_test.log"); value != "sensitive log excluded" {
		t.Fatal("excluded log modified")
	}
	if a.Store.State.RestartRequired || a.Store.State.Installed.Commit != b.Manifest.Commit {
		t.Fatal(a.Store.State)
	}
	safety, _, err := a.Store.verifiedSnapshot(context.Background(), restored.ID, true)
	if err != nil || !safety.Safety {
		t.Fatal(safety, err)
	}
}

func TestStoppedBackupStaysStoppedAndLowDiskRefusesBeforeStop(t *testing.T) {
	a, g, _ := backupFixture(t)
	a.Store.AvailableSpace = func(string) (uint64, error) { return 0, nil }
	refused := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if refused.Status != "failed" || g.hardStops != 0 {
		t.Fatal(refused)
	}
	a.Store.AvailableSpace = nil
	g.alive = false
	a.Store.State.PID = 0
	a.Store.State.ProcessToken = ""
	launches := g.launches
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", false))
	if created.Status != "succeeded" || g.alive || g.launches != launches || !a.Store.State.Paused {
		t.Fatal(created, a.Store.State)
	}
}

func TestRestoreFailureRecoversPreimages(t *testing.T) {
	a, g, _ := backupFixture(t)
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" {
		t.Fatal(created)
	}
	writeTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "pre-restore world")
	once := true
	a.Store.BeforeWrite = func(string) error {
		if once {
			once = false
			return errors.New("injected disk write failure")
		}
		return nil
	}
	result := a.Process(context.Background(), backupRequest(a, "backup-restore", created.Backup.ID, true))
	if result.Status != "failed" || !strings.Contains(result.Recovery, "recovered") || !g.alive {
		t.Fatal(result)
	}
	if value := readTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "pre-restore world" {
		t.Fatal(value)
	}
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "restore.json")); !os.IsNotExist(err) {
		t.Fatal("recovery journal remained", err)
	}
}

func TestCorruptBackupNeverStopsGame(t *testing.T) {
	a, g, _ := backupFixture(t)
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" {
		t.Fatal(created)
	}
	archive, _ := snapshotPath(a.Store.Config, created.Backup.ID, false)
	if err := os.WriteFile(archive, []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	stops := g.hardStops
	r := a.Process(context.Background(), backupRequest(a, "backup-restore", created.Backup.ID, true))
	if r.Status != "failed" || g.hardStops != stops || !g.alive {
		t.Fatal(r)
	}
}

func TestInterruptedRestoreRecoveryAndCorruptSafetyRefusal(t *testing.T) {
	a, g, _ := backupFixture(t)
	created := a.Process(context.Background(), backupRequest(a, "backup-create", "", true))
	if created.Status != "succeeded" {
		t.Fatal(created)
	}
	writeTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "before interruption")
	writeTest(t, a.Store.Config.InstallDir, "profiles/mod/runtime.json", "before interruption")
	count := 0
	a.Store.BeforeWrite = func(string) error {
		count++
		if count == 2 {
			panic("simulated process loss")
		}
		return nil
	}
	req := backupRequest(a, "backup-restore", created.Backup.ID, true)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("interruption was not reached")
			}
		}()
		_ = a.Process(context.Background(), req)
	}()
	a.Store.BeforeWrite = nil
	if _, err := os.Stat(filepath.Join(a.Store.Config.StateDir, "restore.json")); err != nil {
		t.Fatal(err)
	}
	safety, _ := snapshotPath(a.Store.Config, req.ID, true)
	original, err := os.ReadFile(safety)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(safety, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	before := readTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db")
	if err = a.recoverRestore(context.Background()); err == nil {
		t.Fatal("corrupt safety archive accepted")
	}
	if value := readTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != before {
		t.Fatal("live data changed before preimage validation")
	}
	if err = os.WriteFile(safety, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err = a.recoverRestore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if value := readTest(t, a.Store.Config.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "before interruption" {
		t.Fatal(value)
	}
	if g.alive || !a.Store.State.Paused || a.Store.State.PID != 0 {
		t.Fatal("interrupted recovery launched a game", a.Store.State)
	}
}

func TestSnapshotRetentionKeepsTenAndSafety(t *testing.T) {
	a, g, _ := backupFixture(t)
	g.alive = false
	var ids []string
	for i := 0; i < 12; i++ {
		id := NewID()
		ids = append(ids, id)
		if _, err := a.Store.createSnapshot(context.Background(), id, false, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	safetyID := NewID()
	if _, err := a.Store.createSnapshot(context.Background(), safetyID, true, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.retainSnapshots(); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		file, _ := snapshotPath(a.Store.Config, id, false)
		_, err := os.Stat(file)
		if i < 2 && !os.IsNotExist(err) {
			t.Fatal("old backup retained", id, err)
		}
		if i >= 2 && err != nil {
			t.Fatal("recent backup removed", id, err)
		}
	}
	file, _ := snapshotPath(a.Store.Config, safetyID, true)
	if _, err := os.Stat(file); err != nil {
		t.Fatal("safety backup pruned", err)
	}
}

func TestRemoteBackupVerifyDownloadAndExclusiveDestination(t *testing.T) {
	a, g, _ := backupFixture(t)
	g.alive = false
	id := NewID()
	receipt, err := a.Store.createSnapshot(context.Background(), id, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := sshFixture(t)
	endpoint.RemoteDir = "/" + strings.TrimPrefix(filepath.ToSlash(a.Store.Config.StateDir), "/")
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	checked, err := r.VerifyBackup(context.Background(), a.Store.Config.Target, id)
	if err != nil || checked.SHA256 != receipt.SHA256 {
		t.Fatal(checked, err)
	}
	dest := filepath.Join(t.TempDir(), "download.zip")
	if _, err = r.DownloadBackup(context.Background(), a.Store.Config.Target, id, dest); err != nil {
		t.Fatal(err)
	}
	if _, err = r.DownloadBackup(context.Background(), a.Store.Config.Target, id, dest); err == nil {
		t.Fatal("existing download overwritten")
	}
	backups, err := r.ListBackups(a.Store.Config.Target)
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
}
