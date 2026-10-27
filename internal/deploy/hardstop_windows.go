//go:build windows

package deploy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func handleIdentity(h windows.Handle, exe string) (string, error) {
	var name [32768]uint16
	size := uint32(len(name))
	if err := windows.QueryFullProcessImageName(h, 0, &name[0], &size); err != nil {
		return "", err
	}
	full, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(windows.UTF16ToString(name[:size]), full) {
		return "", errors.New("process executable identity changed")
	}
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime), nil
}

// checkOtherGames refuses unmanaged/duplicate processes for this installation.
// An inaccessible DayZ process is not assumed safe to ignore.
func checkOtherGames(exe string, except int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if int(entry.ProcessID) == except || !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "DayZServer_x64.exe") {
			continue
		}
		h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if e != nil {
			return fmt.Errorf("cannot inspect other DayZ process %d: %w", entry.ProcessID, e)
		}
		var name [32768]uint16
		size := uint32(len(name))
		e = windows.QueryFullProcessImageName(h, 0, &name[0], &size)
		windows.CloseHandle(h)
		if e != nil {
			return e
		}
		full, _ := filepath.Abs(exe)
		if strings.EqualFold(windows.UTF16ToString(name[:size]), full) {
			return fmt.Errorf("unmanaged or duplicate game process %d blocks maintenance", entry.ProcessID)
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return err
	}
	return nil
}

func (g *NativeGame) HardStop(ctx context.Context, state State) error {
	exe := filepath.Join(g.Config.InstallDir, "DayZServer_x64.exe")
	if state.PID == 0 {
		return checkOtherGames(exe, 0)
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(state.PID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return checkOtherGames(exe, 0)
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	token, err := handleIdentity(h, exe)
	if err != nil {
		return err
	}
	if token != state.ProcessToken {
		return errors.New("process creation token changed; refusing hard stop")
	}
	if err = checkOtherGames(exe, state.PID); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	var code uint32
	if err = windows.GetExitCodeProcess(h, &code); err != nil {
		return err
	}
	if code != 259 {
		return nil
	}
	if err = windows.TerminateProcess(h, 1); err != nil {
		return err
	}
	for {
		result, e := windows.WaitForSingleObject(h, 100)
		if e != nil {
			return e
		}
		if result == windows.WAIT_OBJECT_0 {
			return nil
		}
		if result != uint32(windows.WAIT_TIMEOUT) {
			return fmt.Errorf("unexpected process wait result %d", result)
		}
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("identified process did not exit after hard stop: %w", err)
		}
	}
}
