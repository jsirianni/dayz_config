package deploy

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func fixtureHostKey(t *testing.T, e Endpoint) ssh.PublicKey {
	t.Helper()
	raw, err := os.ReadFile(e.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	_, _, key, _, _, err := ssh.ParseKnownHosts(raw)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestAutomaticFirstUseTrust(t *testing.T) {
	e, _ := sshFixture(t)
	key := fixtureHostKey(t, e)
	if err := os.Remove(e.KnownHosts); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A dry-run accepts the first key without modifying local trust.
	r, err := ConnectFirstUse(ctx, e, "", false)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if _, err := os.Stat(e.KnownHosts); !os.IsNotExist(err) {
		t.Fatal("dry-run persisted trust", err)
	}
	wrong := "SHA256:" + strings.Repeat("A", 43)
	if _, err := ConnectFirstUse(ctx, e, wrong, true); err == nil {
		t.Fatal("explicit fingerprint mismatch accepted")
	}
	if _, err := os.Stat(e.KnownHosts); !os.IsNotExist(err) {
		t.Fatal("mismatched fingerprint persisted trust", err)
	}
	// Real first deployment needs no fingerprint, prompt, or manual keyscan.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := ConnectFirstUse(ctx, e, "", true)
			if err != nil {
				t.Error(err)
				return
			}
			r.Close()
		}()
	}
	wg.Wait()
	raw, err := os.ReadFile(e.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimSpace(string(raw)), "\n")) != 1 {
		t.Fatal("concurrent first use duplicated trust")
	}
	if got := fixtureHostKey(t, e); !bytes.Equal(got.Marshal(), key.Marshal()) {
		t.Fatal("saved wrong host key")
	}
	r, err = Connect(ctx, e)
	if err != nil {
		t.Fatal("saved key not usable by subsequent commands", err)
	}
	r.Close()
	other, _ := sshFixture(t)
	otherKey := fixtureHostKey(t, other)
	changed := []byte(knownhosts.Line([]string{e.Address}, otherKey) + "\n")
	if err := os.WriteFile(e.KnownHosts, changed, 0600); err != nil {
		t.Fatal(err)
	}
	for _, persist := range []bool{false, true} {
		if _, err := ConnectFirstUse(ctx, e, "", persist); err == nil {
			t.Fatal("first-use policy accepted a changed key")
		}
	}
	if got, _ := os.ReadFile(e.KnownHosts); !bytes.Equal(got, changed) {
		t.Fatal("changed key overwrote existing trust")
	}
}

func TestPinnedFirstConnectionAndConcurrentEnrollment(t *testing.T) {
	e, _ := sshFixture(t)
	key := fixtureHostKey(t, e)
	pin := ssh.FingerprintSHA256(key)
	if err := os.Remove(e.KnownHosts); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := Connect(ctx, e); err == nil {
		t.Fatal("unknown key accepted")
	}
	r, err := ConnectPinned(ctx, e, pin, false)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if _, err = os.Stat(e.KnownHosts); !os.IsNotExist(err) {
		t.Fatal("dry-run changed known_hosts", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := ConnectPinned(ctx, e, pin, true)
			if err != nil {
				t.Error(err)
				return
			}
			r.Close()
		}()
	}
	wg.Wait()
	raw, err := os.ReadFile(e.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimSpace(string(raw)), "\n")) != 1 {
		t.Fatal("concurrent enrollment duplicated known_hosts")
	}
	r, err = Connect(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	before := string(raw)
	wrong := "SHA256:" + strings.Repeat("A", 43)
	if _, err = ConnectPinned(ctx, e, wrong, true); err == nil {
		t.Fatal("different pin replaced trust")
	}
	raw, _ = os.ReadFile(e.KnownHosts)
	if string(raw) != before {
		t.Fatal("host key changed")
	}
}

func TestChangedKnownKeyCannotBeOverriddenByPin(t *testing.T) {
	e, _ := sshFixture(t)
	key := fixtureHostKey(t, e)
	other, otherRoot := sshFixture(t)
	_ = otherRoot
	otherKey := fixtureHostKey(t, other)
	if err := os.WriteFile(e.KnownHosts, []byte(knownhosts.Line([]string{e.Address}, otherKey)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := ConnectPinned(ctx, e, ssh.FingerprintSHA256(key), true); err == nil {
		t.Fatal("changed host key accepted with a new pin")
	}
}

func TestEnrollmentProbeRefusesDamagedOrForeignState(t *testing.T) {
	e, root := sshFixture(t)
	r, err := Connect(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err = r.EnrollmentNeeded("nasdara-staging"); err == nil {
		t.Fatal("state without host config accepted")
	}
	if err = WriteJSON(filepath.Join(root, "host.json"), HostConfig{Version: Protocol, Target: "nasdara-staging"}); err != nil {
		t.Fatal(err)
	}
	if needed, err := r.EnrollmentNeeded("nasdara-staging"); err != nil || needed {
		t.Fatal("healthy existing enrollment was reset", needed, err)
	}
	if err = os.WriteFile(filepath.Join(root, "state.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnrollmentNeeded("nasdara-staging"); err == nil {
		t.Fatal("corrupt state treated as fresh")
	}
	for _, f := range []string{"host.json", "state.json", "heartbeat.json"} {
		if err = os.Remove(filepath.Join(root, f)); err != nil {
			t.Fatal(err)
		}
	}
	if needed, err := r.EnrollmentNeeded("nasdara-staging"); err != nil || !needed {
		t.Fatal("empty host not enrollable", needed, err)
	}
	if err = os.WriteFile(filepath.Join(root, "unmanaged"), []byte("existing data"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnrollmentNeeded("nasdara-staging"); err == nil {
		t.Fatal("unmanaged directory reset")
	}
	journal := Enrollment{Version: Protocol, Target: "nasdara-staging", ID: NewID(), Phase: "installing"}
	if err = WriteJSON(filepath.Join(root, "enrollment.json"), journal); err != nil {
		t.Fatal(err)
	}
	if needed, err := r.EnrollmentNeeded("nasdara-staging"); err != nil || !needed {
		t.Fatal("interrupted enrollment cannot resume", needed, err)
	}
	journal.Target = "deer-isle"
	if err = WriteJSON(filepath.Join(root, "enrollment.json"), journal); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnrollmentNeeded("nasdara-staging"); err == nil {
		t.Fatal("foreign enrollment accepted")
	}
}

func TestWindowsEnrollmentPathsRejectShellAndTraversal(t *testing.T) {
	for _, p := range []string{`C:\ProgramData\DayZDeploy\nasdara`, `D:/DayZ Server/server`} {
		if _, err := WindowsPath(p); err != nil {
			t.Fatal(p, err)
		}
	}
	for _, p := range []string{`C:\`, `C:\a\..\outside`, `C:\a&whoami`, `C:\a%PATH%`, `C:\a!x!`, `C:\a\CON`, `\\host\share`, `C:\a:secret`, `C:\a"x`, `/tmp/x`, `C:\a^b`} {
		if _, err := WindowsPath(p); err == nil {
			t.Errorf("unsafe path accepted: %s", p)
		}
	}
	c := HostConfig{Version: Protocol, Target: "nasdara", StateDir: `C:\State\nasdara`, InstallDir: `C:\DayZ\server`, SteamCMD: `C:\Steam\steamcmd.exe`, WorkshopDir: `C:\Steam\workshop`, RConPasswordFile: `C:\State\nasdara\secret`, SteamUser: "test", RConAddress: "127.0.0.1:2310", QueryAddress: "127.0.0.1:27016"}
	e := Endpoint{Host: &c, RemoteDir: "/C:/State/nasdara"}
	if err := CheckEnrollment(e, "nasdara"); err != nil {
		t.Fatal(err)
	}
	e.RemoteDir = "/C:/State/another"
	if err := CheckEnrollment(e, "nasdara"); err == nil {
		t.Fatal("unbound remote directory accepted")
	}
}

func TestFirstCopyOnlyProvisionsOnceWithoutStarting(t *testing.T) {
	s := testStore(t)
	g := &fakeGame{store: s}
	a := Agent{Store: s, Game: g}
	e := Enrollment{Version: Protocol, Target: s.Config.Target, ID: NewID(), Phase: "service-ready"}
	if err := WriteJSON(enrollmentPath(s.Config), e); err != nil {
		t.Fatal(err)
	}
	b := testBundle(t, nil)
	req := submitTest(t, s, b, false)
	r := a.Process(context.Background(), req)
	if r.Status != "succeeded" || g.alive || strings.Join(g.events, ",") != "update" || !s.State.Paused {
		t.Fatal(r, g.events)
	}
	if readTest(t, s.Config.InstallDir, "serverDZ.cfg") == "stock overwritten by Steam" {
		t.Fatal("overlay not applied after provisioning")
	}
	if needed, err := provisioningNeeded(s.Config); err != nil || needed {
		t.Fatal(needed, err)
	}
	a.Process(context.Background(), req)
	if len(g.events) != 1 {
		t.Fatal("repeat operation provisioned twice")
	}
	next := testBundle(t, nil)
	r = a.Process(context.Background(), submitTest(t, s, next, false))
	if r.Status != "succeeded" || len(g.events) != 1 {
		t.Fatal("ordinary copy reran installation", r, g.events)
	}
}

func TestFirstRestartCompletesEnrollmentAfterHealth(t *testing.T) {
	s := testStore(t)
	g := &fakeGame{store: s}
	a := Agent{Store: s, Game: g}
	if err := WriteJSON(enrollmentPath(s.Config), Enrollment{Version: Protocol, Target: s.Config.Target, ID: NewID(), Phase: "service-ready"}); err != nil {
		t.Fatal(err)
	}
	r := a.Process(context.Background(), submitTest(t, s, testBundle(t, nil), true))
	if r.Status != "succeeded" || !g.alive || strings.Join(g.events, ",") != "stop,update,launch,health" {
		t.Fatal(r, g.events)
	}
	if needed, err := provisioningNeeded(s.Config); err != nil || needed {
		t.Fatal(needed, err)
	}
}

func TestAuthenticationProtocolAndRedaction(t *testing.T) {
	var stream, answers bytes.Buffer
	for _, kind := range []string{"status", "password", "guard", "approval", "ready"} {
		if err := sendAuth(&stream, kind, ""); err != nil {
			t.Fatal(err)
		}
	}
	var prompts []string
	err := authConversation(&stream, &answers, func(f AuthFrame) (string, error) { prompts = append(prompts, f.Type); return "private-response", nil })
	if err != nil || strings.Join(prompts, ",") != "status,password,guard,approval" {
		t.Fatal(err, prompts)
	}
	if strings.Count(answers.String(), "private-response") != 2 {
		t.Fatal("answers missing")
	}
	for _, data := range []string{"", `{"version":1,"type":"error","value":"secret"}` + "\n", `{"version":99,"type":"ready"}` + "\n", strings.Repeat("x", 9000) + "\n"} {
		err := authConversation(strings.NewReader(data), &bytes.Buffer{}, nil)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe auth failure", err)
		}
	}
}

func TestSteamPromptsSplitAcrossChunks(t *testing.T) {
	d := steamDialog{}
	if d.consume("Please enter your pass") != "" || d.consume("word:") != "password" {
		t.Fatal("split password prompt lost")
	}
	if d.consume("Steam Guard code:") != "guard" {
		t.Fatal("Guard challenge lost")
	}
	if d.consume("Please confirm the login in your Steam mobile app") != "approval" {
		t.Fatal("mobile approval lost")
	}
	d.consume("Logged in OK")
	if !d.success {
		t.Fatal("successful login not recognized")
	}
}

func stockArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, text := range entries {
		w, err := z.Create("DayZ-Central-Economy-" + nasdaraMissionCommit + "/dayzOffline.nasdara/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestStockMissionPreservesInstalledFilesAndRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	entries := map[string]string{"db/types.xml": "stock", "cfgeconomycore.xml": "core", "init.c": "init"}
	writeTest(t, root, "db/types.xml", "existing stock")
	if err := installNasdaraMission(context.Background(), root, stockArchive(t, entries)); err != nil {
		t.Fatal(err)
	}
	if readTest(t, root, "db/types.xml") != "existing stock" {
		t.Fatal("existing stock overwritten")
	}
	entries["../../outside"] = "bad"
	if err := installNasdaraMission(context.Background(), t.TempDir(), stockArchive(t, entries)); err == nil {
		t.Fatal("stock mission path escaped")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := installNasdaraMission(ctx, t.TempDir(), stockArchive(t, map[string]string{"db/types.xml": "stock"})); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAgentArtifactRejectsNonExecutable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agent.exe")
	if err := os.WriteFile(p, []byte("untrusted text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := agentBinary(Endpoint{AgentBinary: p}); err == nil {
		t.Fatal("text uploaded as executable")
	}
}

func TestOfficialStockMissionDownload(t *testing.T) {
	if os.Getenv("DZDEPLOY_CHECK_VENDOR_DOWNLOADS") != "1" {
		t.Skip("explicit network/download check only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	raw, err := downloadOfficial(ctx, "https://codeload.github.com/BohemiaInteractive/DayZ-Central-Economy/zip/"+nasdaraMissionCommit, 256<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err = installNasdaraMission(ctx, t.TempDir(), raw); err != nil {
		t.Fatal(err)
	}
}
