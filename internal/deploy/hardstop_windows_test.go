//go:build windows

package deploy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeHardStopExactIdentity(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	if err := os.WriteFile(source, []byte("package main\nimport \"time\"\nfunc main(){for{time.Sleep(time.Second)}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "DayZServer_x64.exe")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", exe, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	child := exec.Command(exe)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	token, err := processToken(child.Process.Pid, exe)
	if err != nil {
		t.Fatal(err)
	}
	g := NativeGame{Config: HostConfig{InstallDir: dir}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = g.HardStop(ctx, State{PID: child.Process.Pid, ProcessToken: "reused"}); err == nil {
		t.Fatal("reused PID accepted")
	}
	if _, err = processToken(child.Process.Pid, exe); err != nil {
		t.Fatal("wrong identity was killed")
	}
	// A second process in the same installation must prevent any termination.
	other := exec.Command(exe)
	if err = other.Start(); err != nil {
		t.Fatal(err)
	}
	if err = g.HardStop(ctx, State{PID: child.Process.Pid, ProcessToken: token}); err == nil {
		_ = other.Process.Kill()
		_ = other.Wait()
		t.Fatal("duplicate process accepted")
	}
	_ = other.Process.Kill()
	_ = other.Wait()
	if _, err = processToken(child.Process.Pid, exe); err != nil {
		t.Fatal("original was killed on duplicate refusal")
	}
	if err = g.HardStop(ctx, State{PID: child.Process.Pid, ProcessToken: token}); err != nil {
		t.Fatal(err)
	}
	if g.Alive(State{PID: child.Process.Pid, ProcessToken: token}) {
		t.Fatal("identified process still alive")
	}
}
