package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func testConfig(t *testing.T) HostConfig {
	t.Helper()
	root := t.TempDir()
	return HostConfig{Version: Protocol, Target: "nasdara-staging", InstallDir: filepath.Join(root, "install"), StateDir: filepath.Join(root, "state"), SteamCMD: filepath.Join(root, "steam", "steamcmd.exe"), WorkshopDir: filepath.Join(root, "workshop"), SteamUser: "test", RConAddress: "127.0.0.1:2999", QueryAddress: "127.0.0.1:27018", RConPasswordFile: filepath.Join(root, "state", "secret"), StartupSeconds: 1, ShutdownSeconds: 1, UpdateSeconds: 1}
}
func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func testBundle(t *testing.T, files map[string]string) *Bundle {
	t.Helper()
	target, _ := TargetFor("nasdara-staging")
	b := &Bundle{Manifest: Manifest{Version: Protocol, ID: NewID(), Target: "nasdara-staging", Commit: strings.Repeat("a", 40), Created: time.Now().UTC(), Launcher: target}, Content: map[string][]byte{}}
	if files == nil {
		files = map[string]string{}
	}
	if _, ok := files["serverDZ.cfg"]; !ok {
		files["serverDZ.cfg"] = "hostname = \"Test\";\n"
	}
	for source, value := range files {
		dest, ok := Destination(target, source)
		if !ok {
			t.Fatalf("invalid test source %s", source)
		}
		raw := []byte(value)
		b.Content[source] = raw
		b.Manifest.Files = append(b.Manifest.Files, File{Source: source, Destination: dest, SHA256: Digest(raw), Size: int64(len(raw))})
	}
	sort.Slice(b.Manifest.Files, func(i, j int) bool { return b.Manifest.Files[i].Destination < b.Manifest.Files[j].Destination })
	return b
}
func writeTest(t *testing.T, root, rel, value string) {
	t.Helper()
	p, err := SafeJoin(root, rel)
	if err == nil {
		err = AtomicWrite(p, []byte(value), 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
}
func readTest(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestOwnershipAndPortablePaths(t *testing.T) {
	target, _ := TargetFor("deer-isle")
	cases := map[string]bool{"serverDZ.cfg": true, "empty.deerisle/db/types.xml": true, "profiles/ExpansionMod/Settings/AISettings.json": true, "profiles/ExpansionMod/Quests/Quests/Quest_1.json": true, "profiles/VPPAdminTools/Permissions/SuperAdmins/SuperAdmins.txt": true, "profiles/VPPAdminTools/Permissions/credentials.txt": false, "profiles/ExpansionMod/ATM/balance.json": false, "profiles/ExpansionMod/Quests/PersistentServerData.json": false, "profiles/ExpansionMod/Quests/PlayerData/user.bin": false, "profiles/ExpansionMod/Logs/a.log": false, "empty.deerisle/storage_1/data.json": false, "dzcli.exe": false, "validation/result.json": false}
	for p, want := range cases {
		dest, ok := Destination(target, p)
		if ok != want {
			t.Errorf("%s allowed=%t want %t", p, ok, want)
		}
		if p == "empty.deerisle/db/types.xml" && dest != "mpmissions/empty.deerisle/db/types.xml" {
			t.Fatal(dest)
		}
	}
	for _, p := range []string{"../outside", "C:/outside", "a\\b", "/root", "a/../b", "CON.txt", "a/trailing.", "a/trailing ", "file:stream", "a//b", "a\nfile"} {
		if SafeRelative(p) == nil {
			t.Errorf("unsafe path accepted: %q", p)
		}
	}
}
func TestApplyDriftDeletionAndRepeat(t *testing.T) {
	s := testStore(t)
	old := testBundle(t, map[string]string{"mpmissions/dayzOffline.nasdara/init.c": "old"})
	if _, _, err := s.Apply(old, NewID()); err != nil {
		t.Fatal(err)
	}
	writeTest(t, s.Config.InstallDir, "serverDZ.cfg", "live operator edit")
	writeTest(t, s.Config.InstallDir, "mpmissions/dayzOffline.nasdara/db/types.xml", "stock")
	writeTest(t, s.Config.InstallDir, "profiles/ExpansionMod/ATM/player.json", "balance")
	next := testBundle(t, nil)
	id := NewID()
	changed, deleted, err := s.Apply(next, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || len(deleted) != 1 {
		t.Fatalf("changed=%v deleted=%v", changed, deleted)
	}
	if readTest(t, s.Config.StateDir, "backups/"+id+"/serverDZ.cfg") != "live operator edit" {
		t.Fatal("lost drift preimage")
	}
	if readTest(t, s.Config.InstallDir, "mpmissions/dayzOffline.nasdara/db/types.xml") != "stock" || readTest(t, s.Config.InstallDir, "profiles/ExpansionMod/ATM/player.json") != "balance" {
		t.Fatal("unmanaged data changed")
	}
	changed, deleted, err = s.Apply(next, NewID())
	if err != nil || len(changed)+len(deleted) != 0 {
		t.Fatalf("repeat not idempotent: %v %v %v", changed, deleted, err)
	}
}
func TestEveryPartialWriteRestoresPreimages(t *testing.T) {
	for failure := 1; failure <= 3; failure++ {
		t.Run(string(rune('0'+failure)), func(t *testing.T) {
			s := testStore(t)
			old := testBundle(t, map[string]string{"mpmissions/dayzOffline.nasdara/init.c": "old"})
			if _, _, err := s.Apply(old, NewID()); err != nil {
				t.Fatal(err)
			}
			writeTest(t, s.Config.InstallDir, "serverDZ.cfg", "live drift")
			next := testBundle(t, map[string]string{"profiles/ExpansionMod/Loadouts/new.json": "{}", "serverDZ.cfg": "new"})
			calls := 0
			s.BeforeWrite = func(string) error {
				calls++
				if calls == failure {
					return errors.New("injected disk failure")
				}
				return nil
			}
			if _, _, err := s.Apply(next, NewID()); err == nil {
				t.Fatal("expected failure")
			}
			if readTest(t, s.Config.InstallDir, "serverDZ.cfg") != "live drift" || readTest(t, s.Config.InstallDir, "mpmissions/dayzOffline.nasdara/init.c") != "old" {
				t.Fatal("preimages not restored")
			}
			if _, err := os.Stat(filepath.Join(s.Config.InstallDir, "profiles", "ExpansionMod", "Loadouts", "new.json")); !os.IsNotExist(err) {
				t.Fatal("new file survived rollback")
			}
			if s.State.Installed.ID != old.Manifest.ID {
				t.Fatal("ownership not restored")
			}
		})
	}
}
func TestInterruptedJournalRecovery(t *testing.T) {
	s := testStore(t)
	old := testBundle(t, nil)
	if _, _, err := s.Apply(old, NewID()); err != nil {
		t.Fatal(err)
	}
	id := NewID()
	writeTest(t, s.Config.StateDir, "backups/"+id+"/serverDZ.cfg", "operator preimage")
	writeTest(t, s.Config.InstallDir, "serverDZ.cfg", "half written")
	j := Journal{Version: Protocol, Operation: id, Before: &old.Manifest, Changes: []Preimage{{Destination: "serverDZ.cfg", Existed: true, Mode: 0600, SHA256: Digest([]byte("operator preimage"))}}}
	if err := WriteJSON(filepath.Join(s.Config.StateDir, "journal.json"), j); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	if readTest(t, s.Config.InstallDir, "serverDZ.cfg") != "operator preimage" {
		t.Fatal("recovery lost file")
	}
	var r Result
	if err := ReadJSON(filepath.Join(s.Config.StateDir, "results", id+".json"), &r); err != nil || r.Status != "failed" {
		t.Fatal("interrupted request was not finalized", err)
	}
}
func TestFailedRecoveryRetainsJournal(t *testing.T) {
	s := testStore(t)
	j := Journal{Version: Protocol, Operation: NewID(), Changes: []Preimage{{Destination: "serverDZ.cfg", Existed: true}}}
	_ = WriteJSON(filepath.Join(s.Config.StateDir, "journal.json"), j)
	if err := s.Recover(); err == nil {
		t.Fatal("missing backup accepted")
	}
	if _, err := os.Stat(filepath.Join(s.Config.StateDir, "journal.json")); err != nil {
		t.Fatal("lost recovery journal")
	}
}
func TestBundleIntegrityAndCollision(t *testing.T) {
	b := testBundle(t, nil)
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw, b.Manifest.Target)
	if err != nil || string(decoded.Content["serverDZ.cfg"]) != string(b.Content["serverDZ.cfg"]) {
		t.Fatal(err)
	}
	b.Content["serverDZ.cfg"] = []byte("tampered")
	if err = b.Verify(b.Manifest.Target); err == nil {
		t.Fatal("tampering accepted")
	}
	b = testBundle(t, nil)
	f := b.Manifest.Files[0]
	f.Source = "SERVERDZ.cfg"
	f.Destination = "SERVERDZ.cfg"
	b.Manifest.Files = append(b.Manifest.Files, f)
	b.Content[f.Source] = b.Content["serverDZ.cfg"]
	if err = b.Verify(b.Manifest.Target); err == nil {
		t.Fatal("case collision accepted")
	}
}
func TestHostLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lockHost(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if second, err := lockHost(dir); err == nil {
		second()
		t.Fatal("concurrent agent lock succeeded")
	}
}
func TestSymlinkRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlink privilege unavailable")
	}
	if _, err := SafeJoin(root, "link/file"); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

type fakeGame struct {
	store        *Store
	alive        bool
	events       []string
	failFirst    bool
	launches     int
	stopErr      error
	healthErr    error
	healthReport *DZSAReport
}

func (g *fakeGame) Alive(State) bool { return g.alive }
func (g *fakeGame) Stop(context.Context, State) error {
	g.events = append(g.events, "stop")
	if g.stopErr != nil {
		return g.stopErr
	}
	g.alive = false
	return nil
}
func (g *fakeGame) Update(context.Context, Target) error {
	g.events = append(g.events, "update")
	return AtomicWrite(filepath.Join(g.store.Config.InstallDir, "serverDZ.cfg"), []byte("stock overwritten by Steam"), 0600)
}
func (g *fakeGame) Launch(Target) (int, string, error) {
	g.events = append(g.events, "launch")
	g.launches++
	if read, err := os.ReadFile(filepath.Join(g.store.Config.InstallDir, "serverDZ.cfg")); err != nil || string(read) == "stock overwritten by Steam" {
		return 0, "", errors.New("overlay not reapplied")
	}
	if g.failFirst && g.launches == 1 {
		g.alive = false
		return 42, "fake", nil
	}
	g.alive = true
	return 42, "fake", nil
}
func (g *fakeGame) Healthy(context.Context, Target) (*DZSAReport, error) {
	g.events = append(g.events, "health")
	return g.healthReport, g.healthErr
}
func submitTest(t *testing.T, s *Store, b *Bundle, restart bool) Request {
	t.Helper()
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err = AtomicWrite(filepath.Join(s.Config.StateDir, "inbox", b.Manifest.ID+".zip"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return Request{Version: Protocol, ID: b.Manifest.ID, Target: s.Config.Target, Action: "deploy", Restart: restart, PackageSHA256: Digest(raw)}
}
func TestEngineCopyThenRestartAndRedelivery(t *testing.T) {
	s := testStore(t)
	g := &fakeGame{store: s, alive: true}
	s.State.PID = 42
	s.State.Paused = false
	a := Agent{Store: s, Game: g}
	b := testBundle(t, nil)
	req := submitTest(t, s, b, false)
	r := a.Process(context.Background(), req)
	if r.Status != "succeeded" || !r.RestartRequired || len(g.events) != 0 {
		t.Fatalf("copy restarted game: %+v %v", r, g.events)
	}
	req = Request{Version: Protocol, ID: NewID(), Target: s.Config.Target, Action: "restart"}
	r = a.Process(context.Background(), req)
	if r.Status != "succeeded" || s.State.RunningCommit != b.Manifest.Commit {
		t.Fatalf("restart: %+v", r)
	}
	if strings.Join(g.events, ",") != "stop,update,launch,health" {
		t.Fatal(g.events)
	}
	a.Process(context.Background(), req)
	if g.launches != 1 {
		t.Fatal("redelivery restarted server twice")
	}
}
func TestEngineFailedStartupRecoversOnce(t *testing.T) {
	s := testStore(t)
	g := &fakeGame{store: s}
	a := Agent{Store: s, Game: g}
	old := testBundle(t, nil)
	if r := a.Process(context.Background(), submitTest(t, s, old, false)); r.Status != "succeeded" {
		t.Fatal(r)
	}
	g.failFirst = true
	next := testBundle(t, map[string]string{"serverDZ.cfg": "hostname = \"New\";"})
	next.Manifest.Commit = strings.Repeat("b", 40)
	r := a.Process(context.Background(), submitTest(t, s, next, true))
	if r.Status != "failed" || !strings.Contains(r.Recovery, "recovered") || g.launches != 2 {
		t.Fatalf("recovery %+v launches=%d", r, g.launches)
	}
	if s.State.Installed.ID != old.Manifest.ID || s.State.Paused {
		t.Fatal("previous release not running")
	}
}
func TestEngineStopFailureDoesNotUpdateOrApply(t *testing.T) {
	s := testStore(t)
	writeTest(t, s.Config.InstallDir, "serverDZ.cfg", "running config")
	g := &fakeGame{store: s, alive: true, stopErr: errors.New("timeout")}
	a := Agent{Store: s, Game: g}
	r := a.Process(context.Background(), submitTest(t, s, testBundle(t, nil), true))
	if r.Status != "failed" || strings.Join(g.events, ",") != "stop" || readTest(t, s.Config.InstallDir, "serverDZ.cfg") != "running config" {
		t.Fatal(r, g.events)
	}
}
func TestRollbackRequiresSuccessfulReceipt(t *testing.T) {
	s := testStore(t)
	a := Agent{Store: s, Game: &fakeGame{store: s}}
	r := a.Process(context.Background(), Request{Version: Protocol, ID: NewID(), Target: s.Config.Target, Action: "rollback", Deployment: NewID()})
	if r.Status != "failed" {
		t.Fatal("unknown rollback allowed")
	}
}
