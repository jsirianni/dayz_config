//go:build windows

// A disposable legacy manager whose SCM Stop path kills its child. Successful
// upgrade continuity therefore proves that legacy retirement skipped that path.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func main() {
	if filepath.Base(os.Args[0]) == "steamcmd.exe" {
		_ = os.WriteFile(filepath.Join(filepath.Dir(os.Args[0]), "steam-called.txt"), []byte("unexpected Steam execution"), 0600)
		os.Exit(99)
	}
	if filepath.Base(os.Args[0]) == "DayZServer_x64.exe" {
		file, err := os.OpenFile(filepath.Join(filepath.Dir(os.Args[0]), "game-launches.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(98)
		}
		_, _ = fmt.Fprintln(file, "launch")
		_ = file.Close()
	}
	if len(os.Args) == 3 && os.Args[1] == "--tick" {
		for n := 0; ; n++ {
			_ = deploy.AtomicWrite(os.Args[2], []byte(strconv.Itoa(n)), 0600)
			time.Sleep(25 * time.Millisecond)
		}
	}
	if len(os.Args) != 4 || os.Args[1] != "agent" || os.Args[2] != "--host-config" {
		os.Exit(2)
	}
	c, err := deploy.LoadHost(os.Args[3])
	if err != nil {
		os.Exit(3)
	}
	if err = svc.Run(deploy.ServiceName(c.Target), &handler{c: c}); err != nil {
		os.Exit(4)
	}
}

type handler struct{ c deploy.HostConfig }

func lock(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
}

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	stateLock, err := lock(filepath.Join(h.c.StateDir, "agent.lock"))
	if err != nil {
		return true, 10
	}
	defer windows.CloseHandle(stateLock)
	installLock, err := lock(filepath.Join(h.c.InstallDir, "agent.lock"))
	if err != nil {
		return true, 11
	}
	defer windows.CloseHandle(installLock)
	child := exec.Command(filepath.Join(h.c.InstallDir, "DayZServer_x64.exe"), "--tick", filepath.Join(h.c.InstallDir, "ticks.txt"))
	if err = child.Start(); err != nil {
		return true, 12
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(child.Process.Pid))
	if err != nil {
		return true, 13
	}
	var created, exited, kernel, user windows.Filetime
	err = windows.GetProcessTimes(handle, &created, &exited, &kernel, &user)
	windows.CloseHandle(handle)
	if err != nil {
		return true, 14
	}
	state := deploy.State{Version: deploy.Protocol, Target: h.c.Target, PID: child.Process.Pid, ProcessToken: fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime), Phase: "complete", Paused: true, Updated: time.Now().UTC()}
	if err = deploy.WriteJSON(filepath.Join(h.c.StateDir, "state.json"), state); err != nil {
		return true, 15
	}
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		// This intentionally old heartbeat has no handoff capability or identity.
		file, err := json.Marshal(map[string]any{"version": deploy.Protocol, "updated": time.Now().UTC()})
		if err != nil {
			return true, 16
		}
		if err = deploy.AtomicWrite(filepath.Join(h.c.StateDir, "heartbeat.json"), file, 0600); err != nil {
			return true, 17
		}
		select {
		case <-ticker.C:
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
			case svc.Stop, svc.Shutdown:
				_ = child.Process.Kill()
				_ = child.Wait()
				return false, 0
			}
		}
	}
}
