package hostinspect

import (
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCommandFitsWindowsSSHCommandLine(t *testing.T) {
	command, err := Command(Spec{Service: "dzdeploy-nasdara", Executable: `C:\DayZ\server\DayZServer_x64.exe`, QueryPort: 27016})
	if err != nil || len(command) > 8000 {
		t.Fatalf("command length %d: %v", len(command), err)
	}
	if strings.Contains(command, "DayZServer") {
		t.Fatal("unencoded input in command")
	}
}

func TestWindowsCollectorLocalQueryFixture(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only native collector")
	}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	done := make(chan struct{})
	defer func() { conn.Close(); <-done }()
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		_, peer, e := conn.ReadFromUDP(buf)
		if e == nil {
			_, _ = conn.WriteToUDP(append([]byte{255, 255, 255, 255, 73, 1}, []byte("Test\x00nasdara\x00")...), peer)
		}
	}()
	command, err := Command(Spec{Service: "dzdeploy-fixture-not-installed", Executable: `C:\dzdeploy-fixture\DayZServer_x64.exe`, QueryPort: conn.LocalAddr().(*net.UDPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	parts := strings.Split(command, " ")
	out, err := exec.CommandContext(ctx, parts[0], parts[1:]...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("collector: %v: %s", err, exit.Stderr)
		}
		t.Fatalf("collector: %v", err)
	}
	var snapshot Snapshot
	if err = json.Unmarshal(out, &snapshot); err != nil || snapshot.Query.Name != "Test" || snapshot.Query.Map != "nasdara" {
		t.Fatalf("%v %s", err, out)
	}
}

func TestCollectorPowerShellParses(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell parser")
	}
	// Parse without executing any host inspection or network request.
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$e=$null;[void][Management.Automation.Language.Parser]::ParseFile('inspect.ps1',[ref]$null,[ref]$e);if($e.Count){$e;exit 1}`).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
}
