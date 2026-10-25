//go:build windows

package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func dialAgent(ctx context.Context) (net.Conn, error) {
	return winio.DialPipeContext(ctx, `\\.\pipe\openssh-ssh-agent`)
}
func replaceFile(from, to string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dest, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// Windows readers (including SFTP) may briefly open a file without delete
	// sharing. Retry that transient condition without weakening ACLs or forcing
	// a held game file closed. Persistent sharing/access failures still abort.
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = windows.MoveFileEx(src, dest, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func isReparse(p string) bool {
	p16, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(p16)
	return err == nil && attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func lockHost(dir string) (func(), error) {
	p, err := windows.UTF16PtrFromString(filepath.Join(dir, "agent.lock"))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, errors.New("another agent owns this instance (or lock path is inaccessible)")
	}
	return func() { windows.CloseHandle(h) }, nil
}
func prepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
func processToken(pid int, exe string) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err = windows.GetExitCodeProcess(h, &code); err != nil {
		return "", err
	}
	if code != 259 {
		return "", os.ErrProcessDone
	}
	var buf [32768]uint16
	size := uint32(len(buf))
	if err = windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	full, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(windows.UTF16ToString(buf[:size]), full) {
		return "", errors.New("PID executable identity changed")
	}
	var create, exit, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &create, &exit, &kernel, &user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", create.HighDateTime, create.LowDateTime), nil
}

func freeSpace(dir string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var available, total, free uint64
	err = windows.GetDiskFreeSpaceEx(p, &available, &total, &free)
	return available, err
}
