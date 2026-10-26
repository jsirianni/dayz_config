//go:build linux

package tools

import (
	"golang.org/x/sys/unix"
	"os/exec"
	"syscall"
)

// Keep Packer, its plugins, and the native provisioning worker in one lifetime.
func startManaged(c *exec.Cmd) (func(), error) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return unix.Kill(-c.Process.Pid, unix.SIGKILL)
	}
	if e := c.Start(); e != nil {
		return nil, e
	}
	return func() { _ = unix.Kill(-c.Process.Pid, unix.SIGKILL) }, nil
}
