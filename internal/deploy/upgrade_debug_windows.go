//go:build windows

package deploy

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var upgradeDebugKernel = windows.NewLazySystemDLL("kernel32.dll")
var upgradeDebugAttach = upgradeDebugKernel.NewProc("DebugActiveProcess")
var upgradeDebugDetach = upgradeDebugKernel.NewProc("DebugActiveProcessStop")
var upgradeDebugKillOnExit = upgradeDebugKernel.NewProc("DebugSetProcessKillOnExit")
var upgradeDebugWait = upgradeDebugKernel.NewProc("WaitForDebugEvent")
var upgradeDebugContinue = upgradeDebugKernel.NewProc("ContinueDebugEvent")
var upgradeDebugInJob = upgradeDebugKernel.NewProc("IsProcessInJob")
var upgradeDebugAdjust = windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")

const (
	upgradeDebugContinueStatus = 0x00010002
	upgradeDebugCreateProcess  = 3
	upgradeDebugExitProcess    = 5
	upgradeDebugLoadDLL        = 6
)

// DEBUG_EVENT from the Windows SDK's minwinbase.h. The x64 union is 160
// bytes (EXCEPTION_DEBUG_INFO) and begins at offset 16, making the event 176
// bytes. Legacy handoff is deliberately limited to Windows amd64.
type upgradeDebugEvent struct {
	Code, PID, TID uint32
	_              uint32
	Data           [20]uintptr
}

type upgradeDebugCommand struct {
	retire bool
	close  bool
	arm    bool
	result chan upgradeDebugResponse
}

type upgradeDebugResponse struct {
	err  error
	done bool
}

// frozenUpgradeManager owns one debugger thread and a retained process handle.
// The coordinator must suppress SCM recovery before creating it. It never
// attaches to, suspends, or terminates the game process.
type frozenUpgradeManager struct {
	commands chan upgradeDebugCommand
	done     chan struct{}
	mu       sync.Mutex
	finished bool
	finalErr error
}

func freezeUpgradeManager(ctx context.Context, manager, game ProcessIdentity) (*frozenUpgradeManager, error) {
	return freezeUpgradeManagerPrepared(ctx, manager, game, upgradeDebugPrivilege, upgradeDebugOpen)
}

func freezeUpgradeManagerPrepared(ctx context.Context, manager, game ProcessIdentity, privilege func() (func(), error), open func(ProcessIdentity, uint32) (windows.Handle, error)) (*frozenUpgradeManager, error) {
	if runtime.GOARCH != "amd64" || unsafe.Sizeof(upgradeDebugEvent{}) != 176 || unsafe.Offsetof(upgradeDebugEvent{}.Data) != 16 {
		return nil, errors.New("legacy agent handoff requires the Windows amd64 debugger ABI")
	}
	if manager.PID <= 0 || game.PID <= 0 || manager.PID == game.PID || manager.Token == "" || game.Token == "" || manager.Executable == "" || game.Executable == "" {
		return nil, errors.New("legacy handoff requires distinct, complete manager and game identities")
	}
	f := &frozenUpgradeManager{commands: make(chan upgradeDebugCommand), done: make(chan struct{})}
	ready := make(chan error, 1)
	go f.run(ctx, manager, game, ready, privilege, open)
	if err := <-ready; err != nil {
		<-f.done
		return nil, err
	}
	return f, nil
}

func (f *frozenUpgradeManager) Retire() error { return f.command(true, false, false) }
func (f *frozenUpgradeManager) Resume() error { return f.command(false, false, false) }

// ArmCrashRetirement is used by the independent retirement guardian only after
// construction has frozen and verified the manager. If that guardian itself
// dies, Windows retires the debugged manager while leaving its child untouched.
func (f *frozenUpgradeManager) ArmCrashRetirement() error { return f.command(false, false, true) }

// Close resumes only an unchanged live game. If the game exited independently,
// it retires the manager instead and returns an error so recovery stays paused.
func (f *frozenUpgradeManager) Close() error { return f.command(false, true, false) }

func (f *frozenUpgradeManager) command(retire, closing, arm bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.finished {
		return f.finalErr
	}
	result := make(chan upgradeDebugResponse, 1)
	f.commands <- upgradeDebugCommand{retire: retire, close: closing, arm: arm, result: result}
	response := <-result
	if response.done {
		<-f.done
		f.finished, f.finalErr = true, response.err
	}
	return response.err
}

func upgradeDebugCall(proc *windows.LazyProc, args ...uintptr) error {
	r, _, err := proc.Call(args...)
	if r != 0 {
		return nil
	}
	if err == windows.ERROR_SUCCESS {
		return fmt.Errorf("%s failed", proc.Name)
	}
	return fmt.Errorf("%s: %w", proc.Name, err)
}

func upgradeDebugPrivilege() (func(), error) {
	restore, err := upgradePrivilege("SeDebugPrivilege")
	if err != nil {
		return nil, fmt.Errorf("legacy manager handoff requires SeDebugPrivilege: %w", err)
	}
	return restore, nil
}

// upgradePrivilege must be acquired and released on the same locked OS thread.
// It modifies only that thread's impersonation token; restoring it never alters
// privileges on the coordinator's other threads.
func upgradePrivilege(privilegeName string) (func(), error) {
	var token windows.Token
	impersonated := false
	err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, true, &token)
	if errors.Is(err, windows.ERROR_NO_TOKEN) {
		if err = windows.ImpersonateSelf(windows.SecurityImpersonation); err != nil {
			return nil, err
		}
		impersonated = true
		err = windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, true, &token)
	}
	if err != nil {
		if impersonated {
			_ = windows.RevertToSelf()
		}
		return nil, err
	}
	cleanup := func() {
		_ = token.Close()
		if impersonated {
			_ = windows.RevertToSelf()
		}
	}
	name, err := windows.UTF16PtrFromString(privilegeName)
	if err != nil {
		cleanup()
		return nil, err
	}
	var luid windows.LUID
	if err = windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		cleanup()
		return nil, err
	}
	next := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}}}
	var previous windows.Tokenprivileges
	var size uint32
	r, _, last := upgradeDebugAdjust.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&next)), unsafe.Sizeof(previous), uintptr(unsafe.Pointer(&previous)), uintptr(unsafe.Pointer(&size)))
	if r == 0 || errors.Is(last, windows.ERROR_NOT_ALL_ASSIGNED) {
		cleanup()
		return nil, fmt.Errorf("enable %s: %w", privilegeName, last)
	}
	return func() {
		_ = windows.AdjustTokenPrivileges(token, false, &previous, 0, nil, nil)
		cleanup()
	}, nil
}

func upgradeDebugOpen(identity ProcessIdentity, access uint32) (windows.Handle, error) {
	h, err := windows.OpenProcess(access|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(identity.PID))
	if err != nil {
		return 0, err
	}
	if err = upgradeDebugIdentity(h, identity); err != nil {
		windows.CloseHandle(h)
		return 0, err
	}
	var inJob int32
	if err = upgradeDebugCall(upgradeDebugInJob, uintptr(h), 0, uintptr(unsafe.Pointer(&inJob))); err != nil {
		windows.CloseHandle(h)
		return 0, err
	}
	if inJob != 0 {
		windows.CloseHandle(h)
		return 0, errors.New("job membership blocks legacy handoff; child survival cannot be guaranteed")
	}
	return h, nil
}

func upgradeDebugIdentity(h windows.Handle, expected ProcessIdentity) error {
	token, err := handleIdentity(h, expected.Executable)
	if err != nil {
		return err
	}
	if token != expected.Token {
		return errors.New("legacy handoff process creation token changed")
	}
	var code uint32
	if err = windows.GetExitCodeProcess(h, &code); err != nil {
		return err
	}
	if code != 259 {
		return errors.New("legacy handoff process exited")
	}
	return nil
}

func upgradeDebugEventFiles(event *upgradeDebugEvent) {
	if (event.Code == upgradeDebugCreateProcess || event.Code == upgradeDebugLoadDLL) && event.Data[0] != 0 && windows.Handle(event.Data[0]) != windows.InvalidHandle {
		_ = windows.CloseHandle(windows.Handle(event.Data[0]))
		event.Data[0] = 0
	}
}

func upgradeDebugWaitEvent(ctx context.Context, event *upgradeDebugEvent) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r, _, err := upgradeDebugWait.Call(uintptr(unsafe.Pointer(event)), 100)
		if r != 0 {
			return nil
		}
		if !errors.Is(err, windows.ERROR_SEM_TIMEOUT) {
			return fmt.Errorf("WaitForDebugEvent: %w", err)
		}
	}
}

func (f *frozenUpgradeManager) run(ctx context.Context, manager, game ProcessIdentity, ready chan<- error, privilege func() (func(), error), open func(ProcessIdentity, uint32) (windows.Handle, error)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(f.done)
	restore, err := privilege()
	if err != nil {
		ready <- err
		return
	}
	defer restore()
	managerHandle, err := open(manager, windows.PROCESS_ALL_ACCESS)
	if err != nil {
		ready <- fmt.Errorf("inspect legacy manager: %w", err)
		return
	}
	defer windows.CloseHandle(managerHandle)
	gameHandle, err := open(game, 0)
	if err != nil {
		ready <- fmt.Errorf("inspect preserved game: %w", err)
		return
	}
	defer windows.CloseHandle(gameHandle)
	if err = ctx.Err(); err != nil {
		ready <- err
		return
	}
	if err = upgradeDebugCall(upgradeDebugAttach, uintptr(manager.PID)); err != nil {
		ready <- err
		return
	}
	attached := true
	defer func() {
		if attached {
			// Constructor errors can occur while waiting for the first event.
			// Never thaw an unpaused legacy manager after independent game exit.
			if upgradeDebugIdentity(gameHandle, game) != nil {
				_ = windows.TerminateProcess(managerHandle, 1)
			}
			_ = upgradeDebugCall(upgradeDebugDetach, uintptr(manager.PID))
		}
	}()
	if err = upgradeDebugCall(upgradeDebugKillOnExit, 0); err != nil {
		ready <- err
		return
	}
	var event upgradeDebugEvent
	if err = upgradeDebugWaitEvent(ctx, &event); err != nil {
		ready <- err
		return
	}
	upgradeDebugEventFiles(&event)
	if event.PID != uint32(manager.PID) || event.Code != upgradeDebugCreateProcess {
		ready <- errors.New("unexpected initial manager debug event")
		return
	}
	if err = upgradeDebugIdentity(managerHandle, manager); err != nil {
		ready <- err
		return
	}
	if err = upgradeDebugIdentity(gameHandle, game); err != nil {
		// The original agent must not resume and automatically relaunch a game
		// that exited while the manager was being frozen.
		retireErr := upgradeDebugRetire(managerHandle, manager.PID, &event)
		ready <- errors.Join(err, retireErr)
		return
	}
	ready <- nil
	for command := range f.commands {
		gameErr := upgradeDebugIdentity(gameHandle, game)
		if command.arm {
			err = gameErr
			if err == nil {
				err = upgradeDebugIdentity(managerHandle, manager)
			}
			if err == nil {
				err = upgradeDebugCall(upgradeDebugKillOnExit, 1)
			}
			command.result <- upgradeDebugResponse{err: err}
			continue
		}
		if !command.retire && gameErr != nil && !command.close {
			command.result <- upgradeDebugResponse{err: fmt.Errorf("refusing to resume legacy manager after game identity loss; retire it and recover paused: %w", gameErr)}
			continue
		}
		if command.retire || gameErr != nil {
			err = upgradeDebugRetire(managerHandle, manager.PID, &event)
			if command.close && gameErr != nil {
				err = errors.Join(errors.New("game exited during handoff; legacy manager retired and recovery must remain paused"), err)
			}
		} else {
			err = upgradeDebugCall(upgradeDebugDetach, uintptr(manager.PID))
		}
		if err == nil || ((command.retire || command.close) && upgradeDebugExited(managerHandle)) {
			attached = false // retire performs its own detach/event cleanup
			command.result <- upgradeDebugResponse{err: err, done: true}
			return
		}
		command.result <- upgradeDebugResponse{err: err}
	}
}

func upgradeDebugExited(manager windows.Handle) bool {
	result, err := windows.WaitForSingleObject(manager, 0)
	return err == nil && result == windows.WAIT_OBJECT_0
}

func upgradeDebugRetire(manager windows.Handle, pid int, pending *upgradeDebugEvent) error {
	if err := windows.TerminateProcess(manager, 1); err != nil {
		if upgradeDebugExited(manager) {
			_ = upgradeDebugCall(upgradeDebugDetach, uintptr(pid))
			return nil
		}
		return fmt.Errorf("terminate verified legacy manager: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	event := *pending
	for {
		upgradeDebugEventFiles(&event)
		if err := upgradeDebugCall(upgradeDebugContinue, uintptr(event.PID), uintptr(event.TID), upgradeDebugContinueStatus); err != nil {
			// A terminating process can have already closed its debug port.
			if result, _ := windows.WaitForSingleObject(manager, 0); result == windows.WAIT_OBJECT_0 {
				return nil
			}
			_ = upgradeDebugCall(upgradeDebugDetach, uintptr(pid))
			return err
		}
		if event.Code == upgradeDebugExitProcess {
			break
		}
		if err := upgradeDebugWaitEvent(ctx, &event); err != nil {
			_ = upgradeDebugCall(upgradeDebugDetach, uintptr(pid))
			return err
		}
		if event.PID != uint32(pid) {
			_ = upgradeDebugCall(upgradeDebugDetach, uintptr(pid))
			return errors.New("unexpected process in legacy manager debug stream")
		}
	}
	result, err := windows.WaitForSingleObject(manager, 2000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("verified legacy manager did not exit after debugger retirement")
	}
	return nil
}
