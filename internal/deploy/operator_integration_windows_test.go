//go:build windows

package deploy

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestOperatorServiceIntegration(t *testing.T) {
	if os.Getenv("DZDEPLOY_OPERATOR_INTEGRATION") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("requires disposable Windows CI runner")
	}
	programData := filepath.Clean(os.Getenv("ProgramData"))
	root, err := os.MkdirTemp(programData, "dzdeploy-operator-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(root) != programData || !strings.HasPrefix(filepath.Base(root), "dzdeploy-operator-integration-") {
			t.Error("unsafe integration cleanup")
			return
		}
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	if err = os.WriteFile(filepath.Join(root, "fixture.marker"), []byte("disposable operator integration"), 0600); err != nil {
		t.Fatal(err)
	}
	c := testConfig(t)
	c.InstallDir = filepath.Join(root, "install")
	c.StateDir = filepath.Join(root, "state")
	c.SteamCMD = filepath.Join(root, "steam", "steamcmd.exe")
	c.WorkshopDir = filepath.Join(root, "workshop")
	c.RConPasswordFile = filepath.Join(c.StateDir, "secret")
	c.StartupSeconds = 30
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := udp.LocalAddr().String()
	udp.Close()
	c.QueryAddress = address
	rconProbe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rconProbe.Close()
	c.RConAddress = rconProbe.LocalAddr().String()
	s, err := OpenStore(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "operator-host.exe")
	if out, e := exec.Command("go", "build", "-buildvcs=false", "-o", helper, "./testdata/operator-host").CombinedOutput(); e != nil {
		t.Fatalf("fixture build: %v %s", e, out)
	}
	if err = AtomicCopy(helper, filepath.Join(c.InstallDir, "DayZServer_x64.exe"), 0700); err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int32
	queryServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "result": DZSAServer{Game: "dayz", Name: "Test", Map: "nasdara", GamePort: 2901, Version: "fixture-1.0"}})
	}))
	defer queryServer.Close()
	ca := filepath.Join(root, "fixture-ca.pem")
	if err = os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: queryServer.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("dzdeploy-operator-fixture-%d", os.Getpid())
	configFile := filepath.Join(root, "fixture.json")
	if err = WriteJSON(configFile, map[string]any{"Host": c, "URL": queryServer.URL, "CA": ca, "Service": name}); err != nil {
		t.Fatal(err)
	}
	manager, err := mgr.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Disconnect()
	if old, e := manager.OpenService(name); e == nil {
		old.Close()
		t.Fatal("refusing existing service")
	} else if !errors.Is(e, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		t.Fatal(e)
	}
	service, err := manager.CreateService(name, helper, mgr.Config{StartType: mgr.StartAutomatic, ServiceStartName: "LocalSystem", DisplayName: "Disposable dzdeploy operator fixture"}, configFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = service.Control(svc.Stop)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			v, e := service.Query()
			if e != nil || v.State == svc.Stopped {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = service.Delete()
		service.Close()
	}()
	if err = service.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		var heartbeat Heartbeat
		if ReadJSON(filepath.Join(c.StateDir, "heartbeat.json"), &heartbeat) == nil && len(heartbeat.Capabilities) >= 3 {
			break
		}
		if time.Now().After(deadline) {
			data, _ := os.ReadFile(filepath.Join(c.StateDir, "fixture-error.txt"))
			t.Fatalf("agent heartbeat missing: %s", data)
		}
		time.Sleep(100 * time.Millisecond)
	}
	submit := func(req Request) Result {
		t.Helper()
		if err := WriteJSON(filepath.Join(c.StateDir, "inbox", req.ID+".json"), req); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(60 * time.Second)
		for {
			var result Result
			if ReadJSON(filepath.Join(c.StateDir, "results", req.ID+".json"), &result) == nil && (result.Status == "succeeded" || result.Status == "failed") {
				if result.Status != "succeeded" {
					t.Fatalf("operation failed: %+v", result)
				}
				return result
			}
			if time.Now().After(deadline) {
				t.Fatal("operation timeout", req.ID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	b := testBundle(t, map[string]string{"mpmissions/dayzOffline.nasdara/db/types.xml": "<types/>"})
	submit(submitTest(t, s, b, true))
	state := func() State {
		t.Helper()
		var v State
		if err := ReadJSON(filepath.Join(c.StateDir, "state.json"), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	request := func(action string, stop bool) Request {
		v := state()
		return Request{Version: Protocol, ID: NewID(), Target: c.Target, Action: action, Stop: stop, ExpectedPID: v.PID, ExpectedToken: v.ProcessToken, ExpectedRelease: v.Installed.ID}
	}
	original := state()
	hard := request("restart", false)
	hard.Hard = true
	submit(hard)
	if current := state(); current.PID == original.PID || current.Paused {
		t.Fatal("hard restart did not produce a healthy new process")
	}
	writeTest(t, c.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "saved world")
	backup := submit(request("backup-create", true))
	if backup.Backup == nil || backup.Backup.Manifest.GameVersion != "fixture-1.0" {
		t.Fatal("backup provenance missing", backup)
	}
	writeTest(t, c.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db", "changed world")
	restore := request("backup-restore", true)
	restore.Backup = backup.Backup.ID
	submit(restore)
	if value := readTest(t, c.InstallDir, "mpmissions/dayzOffline.nasdara/storage_1/players.db"); value != "saved world" {
		t.Fatal(value)
	}
	if value := readTest(t, c.InstallDir, "serverDZ.cfg"); !strings.Contains(value, `hostname = "Test"`) {
		t.Fatal("managed config changed", value)
	}
	if queries.Load() < 4 {
		t.Fatal("fixture HTTPS readiness was not exercised")
	}
	current := state()
	if !(&NativeGame{Config: c}).Alive(current) || current.Paused || current.RestartRequired {
		t.Fatal("game not running after restore", current)
	}
	_ = rconProbe.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buffer := make([]byte, 1024)
	if n, _, _ := rconProbe.ReadFrom(buffer); n > 0 {
		t.Fatal("maintenance emitted an RCon probe")
	}
	if _, err = service.Control(svc.Stop); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(30 * time.Second)
	for {
		v, e := service.Query()
		if e != nil {
			t.Fatal(e)
		}
		if v.State == svc.Stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service shutdown timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if (&NativeGame{Config: c}).Alive(current) {
		t.Fatal("fixture game survived service shutdown")
	}
}
