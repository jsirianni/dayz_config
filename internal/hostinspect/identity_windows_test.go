//go:build windows

package hostinspect

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsCollectorExactKernelCreationToken(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "fixture.go")
	exe := filepath.Join(root, "DayZServer_x64.exe")
	if err := os.WriteFile(source, []byte("package main\nimport \"time\"\nfunc main(){time.Sleep(time.Minute)}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", exe, source).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	child := exec.Command(exe)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(child.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime)
	query, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer query.Close()
	done := make(chan struct{})
	defer func() { query.Close(); <-done }()
	go func() {
		defer close(done)
		b := make([]byte, 1024)
		_, peer, e := query.ReadFromUDP(b)
		if e == nil {
			_, _ = query.WriteToUDP(append([]byte{255, 255, 255, 255, 73, 1}, []byte("Test\x00nasdara\x00")...), peer)
		}
	}()
	command, err := Command(Spec{Service: "dzdeploy-fixture-not-installed", Executable: exe, QueryPort: query.LocalAddr().(*net.UDPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(command, " ")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, parts[0], parts[1:]...).Output()
	if err != nil {
		t.Fatal(err)
	}
	var observed Snapshot
	if err = json.Unmarshal(out, &observed); err != nil {
		t.Fatal(err)
	}
	if len(observed.Processes) != 1 || observed.Processes[0].PID != child.Process.Pid || observed.Processes[0].Token != expected {
		t.Fatalf("kernel token %s did not match collector %+v", expected, observed.Processes)
	}
}
