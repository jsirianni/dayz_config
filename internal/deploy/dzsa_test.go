package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testDZSAAddress = "198.51.100.10:27016"

func dzsaFixture(t *testing.T) (Target, DZSAServer) {
	t.Helper()
	target, err := TargetFor("nasdara")
	if err != nil {
		t.Fatal(err)
	}
	var server DZSAServer
	server.Endpoint.IP, server.Endpoint.Port = "198.51.100.10", 27016
	server.Game, server.GamePort, server.Name, server.Map = "dayz", 2301, "Expected Nasdara", "nasdara"
	server.MaxPlayers, server.Version = 60, "1.30.164200"
	// DZSA's order differs from the launcher order; compare Workshop ID sets.
	server.Mods = []DZSAMod{{"VPPAdminTools", 1828439124}, {"CF", 1559212036}}
	return target, server
}

func dzsaJSON(t *testing.T, server DZSAServer) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"status": 0, "result": server})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestDZSAFindsServerAndReturnsVerifiedMods(t *testing.T) {
	target, found := dzsaFixture(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/query/"+testDZSAAddress || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected DZSA request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, dzsaJSON(t, found))
	}))
	defer server.Close()
	report, err := checkDZSA(context.Background(), server.Client(), server.URL+"/api/v1/query/", testDZSAAddress, target, found.Name)
	if err != nil {
		t.Fatal(err)
	}
	if report.Address != testDZSAAddress || report.CheckedAt.IsZero() || report.Server.Mods[0].Name != "VPPAdminTools" || len(report.Server.Mods) != 2 || report.CheckURL != "https://dayzsalauncher.com/#/servercheck/"+testDZSAAddress {
		t.Fatalf("incomplete verification: %+v", report)
	}
}

func TestDZSARejectsWrongServerAndModSet(t *testing.T) {
	target, original := dzsaFixture(t)
	cases := []struct {
		name   string
		change func(*DZSAServer)
		want   string
	}{
		{"wrong IP", func(s *DZSAServer) { s.Endpoint.IP = "198.51.100.11" }, "different query endpoint"},
		{"wrong query port", func(s *DZSAServer) { s.Endpoint.Port = 27017 }, "different query endpoint"},
		{"wrong game port", func(s *DZSAServer) { s.GamePort = 2302 }, "game port"},
		{"wrong game", func(s *DZSAServer) { s.Game = "arma" }, "unexpected game"},
		{"wrong map", func(s *DZSAServer) { s.Map = "deerisle" }, "unexpected map"},
		{"wrong name", func(s *DZSAServer) { s.Name = "Another server" }, "hostname"},
		{"invalid player count", func(s *DZSAServer) { s.Players = -1 }, "invalid player counts"},
		{"missing player limit", func(s *DZSAServer) { s.MaxPlayers = 0 }, "invalid player counts"},
		{"missing mod", func(s *DZSAServer) { s.Mods = s.Mods[:1] }, "missing Workshop IDs [1559212036]"},
		{"unexpected mod", func(s *DZSAServer) { s.Mods = append(s.Mods, DZSAMod{"Unexpected", 123}) }, "unexpected Workshop IDs [123]"},
		{"no mods", func(s *DZSAServer) { s.Mods = nil }, "missing Workshop IDs"},
		{"zero Workshop ID", func(s *DZSAServer) { s.Mods[0].SteamWorkshopID = 0 }, "invalid or duplicate"},
		{"duplicate mod", func(s *DZSAServer) { s.Mods = append(s.Mods, s.Mods[0]) }, "invalid or duplicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := original
			found.Mods = append([]DZSAMod(nil), original.Mods...)
			tc.change(&found)
			body := dzsaJSON(t, found)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			report, err := checkDZSA(context.Background(), server.Client(), server.URL+"/", testDZSAAddress, target, original.Name)
			if report != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("report=%+v, error=%v; want %q", report, err, tc.want)
			}
		})
	}
}

func TestDZSARejectsFailedAndMalformedResponses(t *testing.T) {
	target, found := dzsaFixture(t)
	cases := []struct {
		name       string
		code       int
		body, want string
	}{
		{"HTTP failure", 503, "unavailable", "HTTP 503"},
		{"rate limit", 429, "slow down", "HTTP 429"},
		{"DZSA rejection", 200, `{"status":1,"error":"Rejected"}`, "Rejected"},
		{"missing status", 200, `{}`, "no status"},
		{"null status", 200, `{"status":null}`, "no status"},
		{"missing result", 200, `{"status":0}`, "no server result"},
		{"bad JSON", 200, `<html>error</html>`, "invalid DZSA response"},
		{"multiple documents", 200, dzsaJSON(t, found) + `{}`, "invalid DZSA response"},
		{"oversize", 200, strings.Repeat(" ", dzsaResponseLimit+1), "too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			if report, err := checkDZSA(context.Background(), server.Client(), server.URL+"/", testDZSAAddress, target, found.Name); report != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("report=%+v, error=%v", report, err)
			}
		})
	}
}

func TestDZSAHonorsRequestCancellation(t *testing.T) {
	target, found := dzsaFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if report, err := checkDZSA(ctx, server.Client(), server.URL+"/", testDZSAAddress, target, found.Name); report != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation lost: %+v %v", report, err)
	}
}

func TestDZSARequiresPublicIPv4AndQueryPort(t *testing.T) {
	for _, address := range []string{"", "192.168.1.196:27016", "127.0.0.1:27016", "0.0.0.0:27016", "[::1]:27016", "example.com:27016", "198.51.100.10:0", "198.51.100.10:65536", "198.51.100.10:27016/path"} {
		if _, err := publicQueryAddress(address); err == nil {
			t.Errorf("invalid public endpoint accepted: %q", address)
		}
	}
}

func TestNativeStartupDoesNotRequireRCon(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			buffer := make([]byte, 1500)
			if _, peer, err := conn.ReadFrom(buffer); err == nil {
				packet := []byte{255, 255, 255, 255, 0x49, 17}
				packet = append(packet, []byte("Expected Nasdara\x00nasdara\x00dayz\x00DayZ\x00")...)
				packet = append(packet, 0, 0, 0, 60, 0, 'd', 'w', 0, 1)
				packet = append(packet, []byte("1.30\x00")...)
				_, _ = conn.WriteTo(packet, peer)
			} else {
				return
			}
		}
	}()
	t.Cleanup(func() { conn.Close(); <-done })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "serverDZ.cfg"), []byte("hostname = \"Expected Nasdara\";\n"), 0600); err != nil {
		t.Fatal(err)
	}
	target, _ := dzsaFixture(t)
	game := NativeGame{Config: HostConfig{InstallDir: root, QueryAddress: conn.LocalAddr().String(), RConPasswordFile: filepath.Join(root, "absent-rcon-secret")}}
	if report, err := game.Healthy(context.Background(), target); report != nil || err != nil {
		t.Fatalf("query-ready game incorrectly depends on RCon: %+v %v", report, err)
	}
	game.Config.PublicQueryAddress = testDZSAAddress
	game.nextDZSA = time.Now().Add(time.Minute)
	game.lastDZSAError = fmt.Errorf("%w: retry later", ErrDZSAUnconfirmed)
	if report, err := game.Healthy(context.Background(), target); report != nil || !errors.Is(err, ErrDZSAUnconfirmed) {
		t.Fatalf("pending DZSA confirmation accepted: %+v %v", report, err)
	}
	_, found := dzsaFixture(t)
	game.lastDZSAReport, game.lastDZSAError = &DZSAReport{Address: testDZSAAddress, Server: found}, nil
	if report, err := game.Healthy(context.Background(), target); report != game.lastDZSAReport || err != nil {
		t.Fatalf("cached verification lost: %+v %v", report, err)
	}
}

type exitingDZSAGame struct {
	*fakeGame
	cancel context.CancelFunc
}

func (g *exitingDZSAGame) Healthy(context.Context, Target) (*DZSAReport, error) {
	g.alive = false
	g.cancel()
	return nil, fmt.Errorf("%w: request interrupted", ErrDZSAUnconfirmed)
}

func TestStartupPersistsDZSAReportAndOutageLeavesGameRunning(t *testing.T) {
	t.Run("confirmed", func(t *testing.T) {
		s := testStore(t)
		_, found := dzsaFixture(t)
		report := &DZSAReport{Address: testDZSAAddress, CheckedAt: time.Now().UTC(), Server: found}
		game := &fakeGame{store: s, healthReport: report}
		a := Agent{Store: s, Game: game}
		bundle := testBundle(t, nil)
		result := a.Process(context.Background(), submitTest(t, s, bundle, true))
		if result.Status != "succeeded" || result.DZSA != report || s.State.DZSA != report || s.State.RunningCommit != bundle.Manifest.Commit || s.State.RestartRequired {
			t.Fatalf("confirmed startup not recorded: %+v %+v", result, s.State)
		}
		var saved State
		if err := ReadJSON(filepath.Join(s.Config.StateDir, "state.json"), &saved); err != nil || saved.DZSA == nil || len(saved.DZSA.Server.Mods) != 2 {
			t.Fatalf("missing persisted report: %+v %v", saved, err)
		}
	})
	t.Run("unconfirmed", func(t *testing.T) {
		s := testStore(t)
		game := &fakeGame{store: s}
		a := Agent{Store: s, Game: game}
		oldBundle := testBundle(t, nil)
		if result := a.Process(context.Background(), submitTest(t, s, oldBundle, false)); result.Status != "succeeded" {
			t.Fatal(result)
		}
		game.healthErr = fmt.Errorf("%w: HTTP 503", ErrDZSAUnconfirmed)
		newBundle := testBundle(t, map[string]string{"serverDZ.cfg": "new desired config"})
		result := a.Process(context.Background(), submitTest(t, s, newBundle, true))
		if result.Status != "failed" || !game.alive || game.launches != 1 || s.State.Installed.ID != newBundle.Manifest.ID || s.State.DZSA != nil || !strings.Contains(result.Recovery, "game left running") {
			t.Fatalf("external check stopped or rolled back game: %+v %+v", result, s.State)
		}
		if len(game.events) < 4 || strings.Join(game.events[:3], ",") != "stop,update,launch" {
			t.Fatalf("unexpected recovery events: %v", game.events)
		}
		for _, event := range game.events[3:] {
			if event != "health" {
				t.Fatalf("external failure triggered recovery: %v", game.events)
			}
		}
	})
	t.Run("game exits during public check", func(t *testing.T) {
		s := testStore(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		game := &exitingDZSAGame{fakeGame: &fakeGame{store: s}, cancel: cancel}
		a := Agent{Store: s, Game: game}
		result := a.Process(ctx, submitTest(t, s, testBundle(t, nil), true))
		if result.Status != "failed" || game.alive || strings.Contains(result.Recovery, "game left running") {
			t.Fatalf("exited game reported running: %+v", result)
		}
	})
}

func TestRemoteDZSACheckIsReadOnlyAndUsesConfiguredPublicAddress(t *testing.T) {
	endpoint, root := sshFixture(t)
	store := testStore(t)
	bundle := testBundle(t, nil)
	state := State{Version: Protocol, Target: bundle.Manifest.Target, Installed: &bundle.Manifest}
	if err := WriteJSON(filepath.Join(root, "state.json"), state); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(filepath.Join(root, "host.json"), store.Config); err != nil {
		t.Fatal(err)
	}
	writeTest(t, store.Config.InstallDir, "serverDZ.cfg", "hostname = \"Expected Nasdara\";\n")
	endpoint.Host = &HostConfig{PublicQueryAddress: testDZSAAddress}
	remote, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	_, found := dzsaFixture(t)
	found.GamePort = bundle.Manifest.Launcher.GamePort
	found.Mods = nil
	for _, mod := range bundle.Manifest.Launcher.Mods {
		id, _ := strconv.ParseUint(mod.ID, 10, 64)
		found.Mods = append(found.Mods, DZSAMod{Name: mod.Folder, SteamWorkshopID: id})
	}
	body := dzsaJSON(t, found)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+testDZSAAddress {
			t.Errorf("used SSH address instead of public query address: %s", r.URL)
		}
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	before, _ := os.ReadFile(filepath.Join(root, "state.json"))
	report, err := remote.checkDZSA(context.Background(), bundle.Manifest.Target, server.Client(), server.URL+"/")
	if err != nil || report.Address != testDZSAAddress {
		t.Fatalf("public check failed: %+v %v", report, err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "state.json"))
	if string(before) != string(after) {
		t.Fatal("read-only check changed agent state")
	}
	if _, err := os.Stat(filepath.Join(root, "inbox")); !os.IsNotExist(err) {
		t.Fatal("read-only check submitted an operation")
	}
	remote.Endpoint.Host = nil
	if _, err := remote.checkDZSA(context.Background(), bundle.Manifest.Target, server.Client(), server.URL+"/"); err == nil || !strings.Contains(err.Error(), "public_query_address") {
		t.Fatalf("private SSH endpoint incorrectly used for public check: %v", err)
	}
}
