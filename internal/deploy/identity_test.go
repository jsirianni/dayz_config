package deploy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const deerIsleHostname = "(Under Development) First Light | Deer Isle | Dayz Expansion | us-east1"
const deerIsleQueryHostname = "(Under Development) First Light | Deer Isle | Dayz Expansion | "

func TestServerIdentitySteamHostnameLimit(t *testing.T) {
	target, _ := TargetFor("deer-isle")
	for _, tc := range []struct {
		name, configured, reported, terrain string
		wantError                           bool
	}{
		{"Deer Isle live regression", deerIsleHostname, deerIsleQueryHostname, "deerisle", false},
		{"full long name", deerIsleHostname, deerIsleHostname, "deerisle", false},
		{"short exact", "Deer Isle", "Deer Isle", "deerisle", false},
		{"63 bytes exact", strings.Repeat("a", 63), strings.Repeat("a", 63), "deerisle", false},
		{"64 bytes truncated", strings.Repeat("a", 64), strings.Repeat("a", 63), "deerisle", false},
		{"UTF-8 byte limit", strings.Repeat("é", 31) + "abc", strings.Repeat("é", 31) + "a", "deerisle", false},
		{"truncated too early", deerIsleHostname, deerIsleQueryHostname[:62], "deerisle", true},
		{"extra byte", deerIsleHostname, deerIsleHostname[:64], "deerisle", true},
		{"different 63 byte name", deerIsleHostname, deerIsleQueryHostname[:62] + "x", "deerisle", true},
		{"short name prefix", "Deer Isle", "Deer", "deerisle", true},
		{"63 byte name prefix", strings.Repeat("a", 63), strings.Repeat("a", 62), "deerisle", true},
		{"wrong case", "Deer Isle", "deer isle", "deerisle", true},
		{"wrong map", deerIsleHostname, deerIsleQueryHostname, "nasdara", true},
		{"empty name", "", "", "deerisle", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyServerIdentity(target, tc.configured, tc.reported, tc.terrain)
			if (err != nil) != tc.wantError {
				t.Fatalf("verifyServerIdentity() = %v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

func TestNativeStartupAcceptsTruncatedDeerIsleHostname(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			buffer := make([]byte, 1500)
			_, peer, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			packet := []byte{255, 255, 255, 255, 0x49, 17}
			packet = append(packet, []byte(deerIsleQueryHostname+"\x00deerisle\x00dayz\x00DayZ\x00")...)
			packet = append(packet, 0, 0, 0, 60, 0, 'd', 'w', 0, 1)
			packet = append(packet, []byte("1.30\x00")...)
			_, _ = conn.WriteTo(packet, peer)
		}
	}()
	t.Cleanup(func() { conn.Close(); <-done })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "serverDZ.cfg"), []byte("hostname = \""+deerIsleHostname+"\";\n"), 0600); err != nil {
		t.Fatal(err)
	}
	target, _ := TargetFor("deer-isle")
	game := NativeGame{Config: HostConfig{InstallDir: root, QueryAddress: conn.LocalAddr().String()}}
	if report, err := game.Healthy(context.Background(), target); report != nil || err != nil {
		t.Fatalf("query-ready Deer Isle rejected: report=%+v err=%v", report, err)
	}
}

func TestDZSAAcceptsTruncatedHostname(t *testing.T) {
	target, found := dzsaFixture(t)
	found.Name = deerIsleQueryHostname
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, dzsaJSON(t, found))
	}))
	defer server.Close()
	report, err := checkDZSA(context.Background(), server.Client(), server.URL+"/", testDZSAAddress, target, deerIsleHostname)
	if err != nil || report == nil || report.Server.Name != deerIsleQueryHostname {
		t.Fatalf("truncated DZSA hostname rejected: report=%+v err=%v", report, err)
	}
}
