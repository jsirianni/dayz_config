package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentProcessesQueueAndResumesAfterServiceRestart(t *testing.T) {
	s := testStore(t)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	b := testBundle(t, nil)
	req := submitTest(t, s, b, true)
	if err := WriteJSON(filepath.Join(s.Config.StateDir, "inbox", req.ID+".json"), req); err != nil {
		t.Fatal(err)
	}
	run := func(g *fakeGame) (context.CancelFunc, <-chan error) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- RunAgent(ctx, s.Config, g) }()
		return cancel, done
	}
	g := &fakeGame{store: s}
	cancel, done := run(g)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var result Result
		if ReadJSON(filepath.Join(s.Config.StateDir, "results", req.ID+".json"), &result) == nil && result.Status == "succeeded" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("queued deployment not completed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var state State
	if err := ReadJSON(filepath.Join(s.Config.StateDir, "state.json"), &state); err != nil {
		t.Fatal(err)
	}
	if state.Paused {
		t.Fatal("stopping service disabled automatic startup")
	}
	g = &fakeGame{store: s}
	cancel, done = run(g)
	deadline = time.Now().Add(5 * time.Second)
	for {
		var state State
		if ReadJSON(filepath.Join(s.Config.StateDir, "state.json"), &state) == nil && state.RunningCommit == b.Manifest.Commit && state.PID != 0 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			stopErr := <-done
			t.Fatalf("service restart did not resume game: state=%+v agent=%v", state, stopErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if g.launches != 1 {
		t.Fatalf("resumed %d game processes", g.launches)
	}
}

func TestCorruptBackupBlocksRecoveryBeforeAnyWrite(t *testing.T) {
	s := testStore(t)
	id := NewID()
	writeTest(t, s.Config.InstallDir, "serverDZ.cfg", "live")
	writeTest(t, s.Config.StateDir, "backups/"+id+"/serverDZ.cfg", "corrupt")
	j := Journal{Version: Protocol, Operation: id, Changes: []Preimage{{Destination: "serverDZ.cfg", Existed: true, Mode: 0600, SHA256: Digest([]byte("expected"))}}}
	if err := WriteJSON(filepath.Join(s.Config.StateDir, "journal.json"), j); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	if readTest(t, s.Config.InstallDir, "serverDZ.cfg") != "live" {
		t.Fatal("recovery touched live file before verifying backup")
	}
}
func TestRecoveryCannotDeletePersistence(t *testing.T) {
	s := testStore(t)
	path := "mpmissions/dayzOffline.nasdara/storage_1/data.json"
	writeTest(t, s.Config.InstallDir, path, "players")
	j := Journal{Version: Protocol, Operation: NewID(), Changes: []Preimage{{Destination: path}}}
	if err := s.restore(j); err == nil {
		t.Fatal("unsafe recovery allowed")
	}
	if readTest(t, s.Config.InstallDir, path) != "players" {
		t.Fatal("persistence changed")
	}
}
func TestRetentionPreservesCurrentRelease(t *testing.T) {
	s := testStore(t)
	var oldest string
	for i := 0; i < 12; i++ {
		b := testBundle(t, nil)
		b.Manifest.ID = time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format("20060102T150405") + "-0000000000000000"
		if i == 0 {
			oldest = b.Manifest.ID
			s.State.Installed = &b.Manifest
		}
		if err := WriteJSON(filepath.Join(s.Config.StateDir, "releases", b.Manifest.ID+".success.json"), b.Manifest); err != nil {
			t.Fatal(err)
		}
		raw, _ := b.Encode()
		if err := AtomicWrite(packagePath(s.Config.StateDir, b.Manifest.ID), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Retain(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(packagePath(s.Config.StateDir, oldest)); err != nil {
		t.Fatal("current release pruned")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Config.StateDir, "releases"))
	if len(entries) != 22 {
		t.Fatalf("expected ten recent releases plus current, got %d artifacts", len(entries))
	}
}
func TestModSyncPreservesUnchangedAndRemovesOnlyModExtras(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeTest(t, source, "addons/data.pbo", "source bytes")
	writeTest(t, dest, "obsolete/file.txt", "old")
	if err := syncMod(source, dest); err != nil {
		t.Fatal(err)
	}
	if readTest(t, dest, "addons/data.pbo") != "source bytes" {
		t.Fatal("mod not copied")
	}
	if _, err := os.Stat(filepath.Join(dest, "obsolete")); !os.IsNotExist(err) {
		t.Fatal("obsolete mod path retained")
	}
	before, err := os.Stat(filepath.Join(dest, "addons", "data.pbo"))
	if err != nil {
		t.Fatal(err)
	}
	if err = syncMod(source, dest); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(filepath.Join(dest, "addons", "data.pbo"))
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged mod file rewritten")
	}
}
