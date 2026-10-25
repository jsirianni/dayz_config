//go:build linux

package deploy

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func dialAgent(ctx context.Context) (net.Conn, error) {
	p := os.Getenv("SSH_AUTH_SOCK")
	if p == "" {
		return nil, errors.New("SSH_AUTH_SOCK unset")
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", p)
}
func replaceFile(from, to string) error { return os.Rename(from, to) }
func isReparse(string) bool             { return false }
func lockHost(dir string) (func(), error) {
	f, err := os.OpenFile(dir+"/agent.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another agent owns this instance")
	}
	return func() { f.Close() }, nil
}
func prepareCommand(*exec.Cmd) {}
func processToken(pid int, exe string) (string, error) {
	if err := syscall.Kill(pid, 0); err != nil {
		return "", err
	}
	return strconv.Itoa(pid), nil
}

func freeSpace(dir string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(dir, &stat)
	return stat.Bavail * uint64(stat.Bsize), err
}
