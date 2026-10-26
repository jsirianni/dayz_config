//go:build windows

package tools

import (
	"errors"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

func startManaged(c *exec.Cmd) (func(), error) {
	job, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return nil, e
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, e = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); e != nil {
		windows.CloseHandle(job)
		return nil, e
	}
	// Suspend until job assignment, so no child can escape the job during startup.
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW, HideWindow: true}
	if e = c.Start(); e != nil {
		windows.CloseHandle(job)
		return nil, e
	}
	fail := func(err error) (func(), error) {
		_ = c.Process.Kill()
		_ = c.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	process, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(c.Process.Pid))
	if e != nil {
		return fail(e)
	}
	e = windows.AssignProcessToJobObject(job, process)
	windows.CloseHandle(process)
	if e != nil {
		return fail(e)
	}
	snapshot, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if e != nil {
		return fail(e)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	e = windows.Thread32First(snapshot, &entry)
	for e == nil {
		if entry.OwnerProcessID == uint32(c.Process.Pid) {
			thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if err != nil {
				return fail(err)
			}
			_, err = windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			if err != nil {
				return fail(err)
			}
			return func() { windows.CloseHandle(job) }, nil
		}
		e = windows.Thread32Next(snapshot, &entry)
	}
	return fail(errors.New("cannot find suspended build process primary thread"))
}
