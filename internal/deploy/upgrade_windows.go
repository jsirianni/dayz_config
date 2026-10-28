//go:build windows

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// The coordinator is deliberately outside the service-writable state tree. A
// service compromise must not turn an upgrade into execution as LocalSystem.
type upgradeServicePolicy struct {
	StartType     uint32               `json:"startType"`
	Delayed       bool                 `json:"delayed"`
	Actions       []mgr.RecoveryAction `json:"actions"`
	Reset         uint32               `json:"reset"`
	NonCrash      bool                 `json:"nonCrash"`
	Command       string               `json:"command"`
	RebootMessage string               `json:"rebootMessage"`
}

type upgradeJournal struct {
	Request           UpgradeRequest       `json:"request"`
	Result            UpgradeResult        `json:"result"`
	Policy            upgradeServicePolicy `json:"policy"`
	BinaryACL         string               `json:"binaryACL"`
	InboxACL          string               `json:"inboxACL"`
	InboxGated        bool                 `json:"inboxGated"`
	InitiallyStopped  bool                 `json:"initiallyStopped"`
	GameParent        int                  `json:"gameParent"`
	HostSHA256        string               `json:"hostSHA256"`
	Cooperative       bool                 `json:"cooperative"`
	Retired           bool                 `json:"retired"`
	Replaced          bool                 `json:"replaced"`
	PolicySuppressed  bool                 `json:"policySuppressed"`
	SuppressionIntent time.Time            `json:"suppressionIntent"`
	DisabledAt        time.Time            `json:"disabledAt"`
	RecoveryDrained   bool                 `json:"recoveryDrained"`
	disabledMonotonic time.Time
	Committed         bool           `json:"committed"`
	RollingBack       bool           `json:"rollingBack"`
	RollbackAgent     *AgentIdentity `json:"rollbackAgent,omitempty"`
}

func upgradeCoordinatorName(r UpgradeRequest) string {
	return "dzdeploy-upgrade-" + r.Target + "-" + r.ID
}

func protectedUpgradePath(r UpgradeRequest) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	base := os.Getenv("ProgramData")
	if !filepath.IsAbs(base) {
		return "", errors.New("ProgramData must identify an absolute protected Windows directory")
	}
	protectedBase := filepath.Join(base, "dzdeploy")
	if _, err := SafeJoin(base, "dzdeploy"); err != nil {
		return "", err
	}
	for _, runtimeRoot := range []string{r.Host.StateDir, r.Host.InstallDir, r.Host.WorkshopDir, filepath.Dir(r.Host.SteamCMD)} {
		runtimeRoot = filepath.Clean(runtimeRoot)
		if !filepath.IsAbs(runtimeRoot) {
			return "", errors.New("runtime directories must be absolute for protected upgrade staging")
		}
		if _, err := SafeJoin(filepath.Dir(runtimeRoot), filepath.Base(runtimeRoot)); err != nil {
			return "", err
		}
		if within(protectedBase, runtimeRoot) || within(runtimeRoot, protectedBase) {
			return "", fmt.Errorf("protected coordinator directory overlaps service-writable runtime directory: %s", runtimeRoot)
		}
	}
	return SafeJoin(upgradeRoot(r.Target), r.ID)
}

func upgradeSetDACL(path, sddl string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	group, _, err := sd.Group()
	if err != nil {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		info |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		info |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	current, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	oldOwner, _, err := current.Owner()
	if err != nil {
		return err
	}
	oldGroup, _, err := current.Group()
	if err != nil {
		return err
	}
	if owner != nil && oldOwner != nil && owner.Equals(oldOwner) {
		owner = nil
	}
	if group != nil && oldGroup != nil && group.Equals(oldGroup) {
		group = nil
	}
	if owner != nil {
		info |= windows.OWNER_SECURITY_INFORMATION
	}
	if group != nil {
		info |= windows.GROUP_SECURITY_INFORMATION
	}
	if owner != nil || group != nil {
		restore, err := upgradePrivilege("SeRestorePrivilege")
		if err != nil {
			return err
		}
		defer restore()
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, owner, group, dacl, nil)
}

func upgradeDirectory(path string) error {
	if _, err := SafeJoin(filepath.Dir(path), filepath.Base(path)); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	// Directory-only traversal permits an existing agent to observe that its
	// active marker is absent after a rejected upgrade. It does not inherit
	// onto requests, executables, journals, or receipts.
	return upgradeSetDACL(path, "O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;;FRFX;;;AU)")
}

// Normalize artifacts created by the SSH administrator without protecting their
// DACL: service-account read access is inherited when the coordinator grants it
// on the operation directory later.
func upgradeAdministrativeOwner(path string) error {
	if _, err := SafeJoin(filepath.Dir(path), filepath.Base(path)); err != nil {
		return err
	}
	admin, err := windows.StringToSid("S-1-5-32-544")
	if err != nil {
		return err
	}
	current, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := current.Owner()
	if err != nil {
		return err
	}
	group, _, err := current.Group()
	if err != nil {
		return err
	}
	if owner != nil && group != nil && owner.Equals(admin) && group.Equals(admin) {
		return nil
	}
	beforeDACL, err := captureUpgradeDACL(current)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := upgradePrivilege("SeRestorePrivilege")
	if err != nil {
		return err
	}
	defer restore()
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION, admin, admin, nil, nil); err != nil {
		return err
	}
	after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	afterDACL, err := captureUpgradeDACL(after)
	if err != nil {
		return err
	}
	if beforeDACL != afterDACL {
		dacl, _, err := current.DACL()
		if err != nil {
			return err
		}
		flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
		if beforeDACL.Inheritance&windows.SE_DACL_PROTECTED != 0 {
			flags |= windows.PROTECTED_DACL_SECURITY_INFORMATION
		} else {
			flags |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
		}
		if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, flags, nil, nil, dacl, nil); err != nil {
			return err
		}
		verified, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		got, err := captureUpgradeDACL(verified)
		if err != nil {
			return err
		}
		if got != beforeDACL {
			return fmt.Errorf("owner normalization could not preserve DACL inheritance: before=%+v after=%+v", beforeDACL, got)
		}
	}
	runtime.KeepAlive(current)
	return nil
}

type upgradeDACLView struct {
	SDDL        string
	Inheritance windows.SECURITY_DESCRIPTOR_CONTROL
}

// SECURITY_DESCRIPTOR.String serializes owner/group too, even when a caller
// requested only DACL information. Isolate the actual DACL and its inheritance
// controls before comparing security across an intentional owner change.
func captureUpgradeDACL(source *windows.SECURITY_DESCRIPTOR) (upgradeDACLView, error) {
	dacl, _, err := source.DACL()
	if err != nil {
		return upgradeDACLView{}, err
	}
	control, _, err := source.Control()
	if err != nil {
		return upgradeDACLView{}, err
	}
	mask := windows.SECURITY_DESCRIPTOR_CONTROL(windows.SE_DACL_PROTECTED | windows.SE_DACL_AUTO_INHERITED | windows.SE_DACL_AUTO_INHERIT_REQ)
	clone, err := windows.NewSecurityDescriptor()
	if err != nil {
		return upgradeDACLView{}, err
	}
	if err = clone.SetDACL(dacl, true, false); err != nil {
		return upgradeDACLView{}, err
	}
	if err = clone.SetControl(mask, control&mask); err != nil {
		return upgradeDACLView{}, err
	}
	view := upgradeDACLView{SDDL: clone.String(), Inheritance: control & mask}
	runtime.KeepAlive(source)
	if view.SDDL == "" {
		return view, errors.New("unable to serialize DACL-only security descriptor")
	}
	return view, nil
}

func readUpgradeRequest(path string) (UpgradeRequest, error) {
	var req UpgradeRequest
	if _, err := SafeJoin(filepath.Dir(path), filepath.Base(path)); err != nil {
		return req, err
	}
	if err := ReadJSON(path, &req); err != nil {
		return req, err
	}
	return req, req.Validate()
}

// SubmitAgentUpgrade runs over the administrator's SSH connection, but only
// starts an independent automatic service. Losing SSH never cancels the job.
func SubmitAgentUpgrade(ctx context.Context, requestPath string, out io.Writer) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("agent upgrade submission requires an administrator SSH account")
	}
	req, err := readUpgradeRequest(requestPath)
	if err != nil {
		return err
	}
	dir, err := protectedUpgradePath(req)
	if err != nil {
		return err
	}
	candidate := filepath.Join(filepath.Dir(requestPath), "dzdeploy.exe")
	id, err := ReadAgentIdentity(candidate)
	if err != nil {
		return err
	}
	if err = ValidateUpgradeCandidate(id, req.Revision); err != nil {
		return err
	}
	if id.SHA256 != req.CandidateSHA256 {
		return errors.New("uploaded agent checksum differs from upgrade request")
	}
	for _, p := range []string{filepath.Dir(filepath.Dir(upgradeRoot(req.Target))), filepath.Dir(upgradeRoot(req.Target)), upgradeRoot(req.Target)} {
		if err = upgradeDirectory(p); err != nil {
			return err
		}
	}
	unlock, err := lockHost(upgradeRoot(req.Target))
	if err != nil {
		return fmt.Errorf("another agent upgrade submission owns this target: %w", err)
	}
	defer unlock()
	var active struct {
		Version int    `json:"version"`
		ID      string `json:"id"`
		Target  string `json:"target"`
	}
	activePath := filepath.Join(upgradeRoot(req.Target), "active.json")
	if e := ReadJSON(activePath, &active); e == nil && (active.ID != req.ID || active.Target != req.Target) {
		return fmt.Errorf("upgrade %s is already active; inspect that operation instead of submitting another", active.ID)
	} else if e != nil && !os.IsNotExist(e) {
		return e
	} else if e == nil && (active.Version != Protocol || !ValidID(active.ID)) {
		return errors.New("active upgrade marker identity mismatch")
	}
	protectedRequest := filepath.Join(dir, "request.json")
	var existing UpgradeRequest
	if err = ReadJSON(protectedRequest, &existing); err == nil {
		if !reflect.DeepEqual(existing, req) {
			return errors.New("operation ID already belongs to a different upgrade request")
		}
	} else if !os.IsNotExist(err) {
		return err
	} else {
		if err = upgradeDirectory(dir); err != nil {
			return err
		}
		if err = AtomicCopy(candidate, filepath.Join(dir, "dzdeploy.exe"), 0700); err != nil {
			return err
		}
		if err = AtomicCopy(candidate, filepath.Join(dir, "coordinator.exe"), 0700); err != nil {
			return err
		}
		if err = WriteJSON(protectedRequest, req); err != nil {
			return err
		}
		initial := UpgradeResult{Version: Protocol, ID: req.ID, Target: req.Target, Status: "running", Phase: "submitted", Updated: time.Now().UTC()}
		if err = WriteJSON(filepath.Join(dir, "result.json"), initial); err != nil {
			return err
		}
	}
	for _, name := range []string{"dzdeploy.exe", "coordinator.exe", "request.json", "result.json"} {
		if err = upgradeAdministrativeOwner(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	var completed UpgradeResult
	if resultErr := ReadJSON(filepath.Join(dir, "result.json"), &completed); resultErr == nil {
		if err = completed.Validate(req.Target, req.ID); err != nil {
			return err
		}
		if completed.Status != "running" {
			if out != nil {
				return json.NewEncoder(out).Encode(completed)
			}
			return nil
		}
	} else {
		return resultErr
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	name := upgradeCoordinatorName(req)
	s, err := m.OpenService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		s, err = m.CreateService(name, filepath.Join(dir, "coordinator.exe"), mgr.Config{StartType: mgr.StartAutomatic, ServiceStartName: "LocalSystem", DisplayName: "DayZ agent upgrade: " + req.Target}, "agent-upgrade-coordinator", "--request", protectedRequest)
	}
	if err != nil {
		return err
	}
	defer s.Close()
	cfg, err := s.Config()
	if err != nil {
		return err
	}
	args, err := windows.DecomposeCommandLine(cfg.BinaryPathName)
	if err != nil || len(args) != 4 || !strings.EqualFold(args[0], filepath.Join(dir, "coordinator.exe")) || args[1] != "agent-upgrade-coordinator" || args[2] != "--request" || !strings.EqualFold(args[3], protectedRequest) || !strings.EqualFold(cfg.ServiceStartName, "LocalSystem") {
		return errors.New("coordinator service identity differs from protected request")
	}
	// The journal makes recovery idempotent. An unexpected coordinator failure
	// restarts recovery without depending on the SSH client or managed agent.
	if err = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 2 * time.Second}}, 86400); err != nil {
		return err
	}
	if err = s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return err
	}
	status, err := s.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		if err = s.Start(); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return fmt.Errorf("upgrade %s submitted; inspect its status after reconnecting: %w", req.ID, err)
	}
	if out != nil {
		return json.NewEncoder(out).Encode(map[string]any{"operation": req.ID, "target": req.Target, "status": "submitted"})
	}
	return nil
}

type upgradeCoordinatorHandler struct {
	request string
	name    string
}

func (h *upgradeCoordinatorHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	// SCM stop and SSH disconnect do not authorize aborting halfway through
	// executable replacement. The independent job reaches a durable safe state.
	done := make(chan error, 1)
	go func() { done <- runUpgradeCoordinator(context.Background(), h.request) }()
	status <- svc.Status{State: svc.Running}
	for {
		select {
		case <-requests:
			status <- svc.Status{State: svc.Running}
		case err := <-done:
			var result UpgradeResult
			readErr := ReadJSON(filepath.Join(filepath.Dir(h.request), "result.json"), &result)
			if readErr == nil {
				req, reqErr := readUpgradeRequest(h.request)
				if reqErr != nil {
					readErr = reqErr
				} else {
					readErr = result.Validate(req.Target, req.ID)
				}
			}
			terminal := readErr == nil && result.Status != "running"
			if !terminal && err != nil && readErr == nil {
				result.Error = err.Error()
				result.Recovery = "coordinator will retry this existing operation automatically"
				result.Updated = time.Now().UTC()
				_ = WriteJSON(filepath.Join(filepath.Dir(h.request), "result.json"), result)
			}
			if !terminal {
				return true, 1
			}
			m, e := mgr.Connect()
			if e == nil {
				if s, e := m.OpenService(h.name); e == nil {
					_ = s.ResetRecoveryActions()
					_ = upgradeStartType(s, mgr.StartManual, false)
					_ = s.Delete()
					s.Close()
				}
				m.Disconnect()
			}
			if err != nil {
				return true, 1
			}
			return false, 0
		}
	}
}

func RunUpgradeCoordinator(requestPath string) error {
	req, err := readUpgradeRequest(requestPath)
	if err != nil {
		return err
	}
	dir, err := protectedUpgradePath(req)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(requestPath), filepath.Join(dir, "request.json")) {
		return errors.New("coordinator requires the protected request path")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("upgrade coordinator requires an elevated Windows service")
	}
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return errors.New("upgrade coordinator must run as its independent Windows service")
	}
	return svc.Run(upgradeCoordinatorName(req), &upgradeCoordinatorHandler{request: requestPath, name: upgradeCoordinatorName(req)})
}

func upgradeStartType(s *mgr.Service, start uint32, delayed bool) error {
	if err := windows.ChangeServiceConfig(s.Handle, windows.SERVICE_NO_CHANGE, start, windows.SERVICE_NO_CHANGE, nil, nil, nil, nil, nil, nil, nil); err != nil {
		return err
	}
	var info windows.SERVICE_DELAYED_AUTO_START_INFO
	if delayed {
		info.IsDelayedAutoStartUp = 1
	}
	return windows.ChangeServiceConfig2(s.Handle, windows.SERVICE_CONFIG_DELAYED_AUTO_START_INFO, (*byte)(unsafe.Pointer(&info)))
}

func snapshotUpgradePolicy(s *mgr.Service, cfg mgr.Config) (p upgradeServicePolicy, err error) {
	p.StartType = cfg.StartType
	p.Delayed = cfg.DelayedAutoStart
	if p.Actions, err = s.RecoveryActions(); err != nil {
		return
	}
	if p.Reset, err = s.ResetPeriod(); err != nil {
		return
	}
	p.NonCrash, err = s.RecoveryActionsOnNonCrashFailures()
	if err != nil {
		return
	}
	if p.Command, err = s.RecoveryCommand(); err != nil {
		return
	}
	p.RebootMessage, err = s.RebootMessage()
	return
}
func restoreUpgradePolicy(s *mgr.Service, p upgradeServicePolicy) error {
	// The narrow API preserves credentials, identity, dependencies and all ports.
	if err := upgradeStartType(s, p.StartType, p.Delayed); err != nil {
		return err
	}
	var actions []windows.SC_ACTION
	for _, a := range p.Actions {
		actions = append(actions, windows.SC_ACTION{Type: uint32(a.Type), Delay: uint32(a.Delay / time.Millisecond)})
	}
	info := windows.SERVICE_FAILURE_ACTIONS{ResetPeriod: p.Reset, ActionsCount: uint32(len(actions))}
	if len(actions) > 0 {
		info.Actions = &actions[0]
	} else {
		info.Actions = &windows.SC_ACTION{}
	}
	if err := windows.ChangeServiceConfig2(s.Handle, windows.SERVICE_CONFIG_FAILURE_ACTIONS, (*byte)(unsafe.Pointer(&info))); err != nil {
		return err
	}
	if err := s.SetRecoveryCommand(p.Command); err != nil {
		return err
	}
	if err := s.SetRebootMessage(p.RebootMessage); err != nil {
		return err
	}
	return s.SetRecoveryActionsOnNonCrashFailures(p.NonCrash)
}

func upgradeIdentityAlive(p ProcessIdentity) error {
	if p.PID <= 0 || p.Token == "" || !filepath.IsAbs(p.Executable) {
		return errors.New("incomplete upgrade process identity")
	}
	token, err := processToken(p.PID, p.Executable)
	if err != nil {
		return err
	}
	if token != p.Token {
		return errors.New("upgrade process creation token changed")
	}
	return nil
}

func upgradeContinuity(c HostConfig, p ProcessIdentity) error {
	if p.PID == 0 {
		return checkOtherGames(filepath.Join(c.InstallDir, "DayZServer_x64.exe"), 0)
	}
	return upgradeIdentityAlive(p)
}

func upgradeGameParent(p ProcessIdentity) (int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if int(entry.ProcessID) == p.PID {
			return int(entry.ParentProcessID), nil
		}
	}
	return 0, errors.New("managed game disappeared during ancestry inspection")
}

func verifyUpgradeAncestry(target string, game, manager ProcessIdentity, previous AgentIdentity) (int, error) {
	parent, err := upgradeGameParent(game)
	if err != nil {
		return 0, err
	}
	if parent == manager.PID {
		return parent, nil
	}
	var proof AdoptionProof
	if err = ReadJSON(filepath.Join(upgradeRoot(target), "adoption.json"), &proof); err != nil {
		return 0, errors.New("existing game parent is not the managed agent and has no protected adoption proof")
	}
	if proof.Version != Protocol || proof.Target != target || !upgradeSameGame(proof.Game, game) || proof.Agent.PID != manager.PID || proof.Agent.Token != manager.Token || proof.Agent.SHA256 != previous.SHA256 {
		return 0, errors.New("existing game adoption proof does not match its current manager")
	}
	if proof.Parent != parent {
		return 0, errors.New("existing game parent differs from protected adoption proof")
	}
	return parent, nil
}

func upgradeGame(c HostConfig) (ProcessIdentity, error) {
	var state State
	if err := ReadJSON(filepath.Join(c.StateDir, "state.json"), &state); err != nil {
		return ProcessIdentity{}, err
	}
	p := ProcessIdentity{PID: state.PID, Token: state.ProcessToken, Executable: filepath.Join(c.InstallDir, "DayZServer_x64.exe")}
	if p.PID == 0 {
		return p, checkOtherGames(p.Executable, 0)
	}
	if err := upgradeIdentityAlive(p); err != nil {
		return p, fmt.Errorf("agent upgrade requires the existing managed game to remain alive: %w", err)
	}
	if err := checkOtherGames(p.Executable, p.PID); err != nil {
		return p, err
	}
	return p, nil
}

func upgradeNoSteam(c HostConfig) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), filepath.Base(c.SteamCMD)) {
			continue
		}
		h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if e != nil {
			return fmt.Errorf("cannot exclude active SteamCMD process: %w", e)
		}
		var name [32768]uint16
		n := uint32(len(name))
		e = windows.QueryFullProcessImageName(h, 0, &name[0], &n)
		windows.CloseHandle(h)
		if e != nil {
			return e
		}
		if strings.EqualFold(windows.UTF16ToString(name[:n]), c.SteamCMD) {
			return errors.New("active SteamCMD process blocks manager-only agent upgrade")
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return err
	}
	return nil
}

func upgradeServiceIdentity(s *mgr.Service, c HostConfig) (mgr.Config, ProcessIdentity, error) {
	cfg, err := s.Config()
	if err != nil {
		return cfg, ProcessIdentity{}, err
	}
	args, err := windows.DecomposeCommandLine(cfg.BinaryPathName)
	exe := filepath.Join(c.StateDir, "bin", "dzdeploy.exe")
	if err != nil || len(args) != 4 || !strings.EqualFold(args[0], exe) || args[1] != "agent" || args[2] != "--host-config" || !strings.EqualFold(args[3], filepath.Join(c.StateDir, "host.json")) {
		return cfg, ProcessIdentity{}, errors.New("managed service executable or host configuration identity differs")
	}
	status, err := s.Query()
	if err != nil {
		return cfg, ProcessIdentity{}, err
	}
	if status.State == svc.Stopped {
		return cfg, ProcessIdentity{Executable: exe}, nil
	}
	if status.State != svc.Running {
		return cfg, ProcessIdentity{}, errors.New("managed agent service is transitioning; retry after it is running or stopped")
	}
	p := ProcessIdentity{PID: int(status.ProcessId), Executable: exe}
	p.Token, err = processToken(p.PID, exe)
	return cfg, p, err
}

func upgradeSameGame(want, got ProcessIdentity) bool {
	return want.PID == got.PID && want.Token == got.Token && strings.EqualFold(want.Executable, got.Executable)
}

func upgradeServiceSID(cfg mgr.Config) (string, error) {
	switch strings.ToUpper(cfg.ServiceStartName) {
	case "LOCALSYSTEM", `NT AUTHORITY\SYSTEM`:
		return "S-1-5-18", nil
	case "LOCALSERVICE", `NT AUTHORITY\LOCALSERVICE`, `NT AUTHORITY\LOCAL SERVICE`:
		return "S-1-5-19", nil
	case "NETWORKSERVICE", `NT AUTHORITY\NETWORKSERVICE`, `NT AUTHORITY\NETWORK SERVICE`:
		return "S-1-5-20", nil
	}
	var sid *windows.SID
	var err error
	if strings.HasPrefix(strings.ToUpper(cfg.ServiceStartName), `NT SERVICE\`) {
		sid, err = virtualServiceSID(cfg.ServiceStartName[len(`NT SERVICE\`):])
	} else {
		sid, _, _, err = windows.LookupSID("", cfg.ServiceStartName)
	}
	if err != nil {
		return "", err
	}
	return sid.String(), nil
}

func prepareUpgradeHold(dir string, hold UpgradeHold, sid string) error {
	if err := upgradeSetDACL(dir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FRFX;;;"+sid+")"); err != nil {
		return err
	}
	if err := WriteJSON(filepath.Join(dir, "hold.json"), hold); err != nil {
		return err
	}
	if err := AtomicWrite(filepath.Join(dir, "ack.json"), []byte("{}\n"), 0600); err != nil {
		return err
	}
	if err := upgradeSetDACL(filepath.Join(dir, "ack.json"), "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFW;;;"+sid+")"); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, "decision.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func waitUpgradeService(ctx context.Context, s *mgr.Service, want svc.State) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := s.Query()
		if err != nil {
			return err
		}
		if status.State == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitUpgradeAck(ctx context.Context, dir string, req UpgradeRequest, game ProcessIdentity, s *mgr.Service) (UpgradeAcknowledgement, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := upgradeContinuity(req.Host, game); err != nil {
			return UpgradeAcknowledgement{}, fmt.Errorf("game continuity lost while awaiting agent adoption: %w", err)
		}
		var ack UpgradeAcknowledgement
		if ReadJSON(filepath.Join(dir, "ack.json"), &ack) == nil && ack.ID != "" {
			if ack.Version != Protocol || ack.ID != req.ID || ack.Target != req.Target || !upgradeSameGame(game, ack.Game) {
				return ack, errors.New("agent adoption acknowledgement identity mismatch")
			}
			if err := ValidateUpgradeCandidate(ack.Agent, req.Revision); err != nil {
				return ack, err
			}
			if ack.Agent.SHA256 != req.CandidateSHA256 || time.Since(ack.Updated) > 15*time.Second || time.Until(ack.Updated) > 15*time.Second {
				return ack, errors.New("agent acknowledgement provenance or time mismatch")
			}
			status, err := s.Query()
			if err != nil {
				return ack, err
			}
			if status.State != svc.Running || int(status.ProcessId) != ack.Agent.PID {
				return ack, errors.New("acknowledging process is not the replacement service")
			}
			if err := upgradeIdentityAlive(ProcessIdentity{PID: ack.Agent.PID, Token: ack.Agent.Token, Executable: filepath.Join(req.Host.StateDir, "bin", "dzdeploy.exe")}); err != nil {
				return ack, err
			}
			return ack, nil
		}
		select {
		case <-ctx.Done():
			return ack, ctx.Err()
		case <-ticker.C:
		}
	}
}

func saveUpgradeJournal(dir string, j *upgradeJournal, phase string) error {
	j.Result.Phase = phase
	j.Result.Updated = time.Now().UTC()
	if err := WriteJSON(filepath.Join(dir, "journal.json"), j); err != nil {
		return err
	}
	return WriteJSON(filepath.Join(dir, "result.json"), j.Result)
}

func restoreUpgradeInbox(j *upgradeJournal) error {
	if !j.InboxGated {
		return nil
	}
	if j.InboxACL == "" {
		return errors.New("inbox gate has no saved access policy")
	}
	if err := upgradeSetDACL(filepath.Join(j.Request.Host.StateDir, "inbox"), j.InboxACL); err != nil {
		return err
	}
	j.InboxGated = false
	return nil
}

func verifyUpgradeHostUnchanged(j *upgradeJournal) error {
	raw, err := os.ReadFile(filepath.Join(j.Request.Host.StateDir, "host.json"))
	if err != nil {
		return err
	}
	if Digest(raw) != j.HostSHA256 {
		return errors.New("host configuration changed during agent upgrade")
	}
	return nil
}

func reportUpgradeRecoveryFailure(dir string, j *upgradeJournal, err error) error {
	if err == nil {
		return nil
	}
	_, gateErr := os.Stat(filepath.Join(upgradeRoot(j.Request.Target), "active.json"))
	if j.Result.Status == "running" || (gateErr == nil && j.Result.Phase != "recovery-required") {
		j.Result.Status = "running"
		j.Result.Error = err.Error()
		j.Result.Recovery = "automatic coordinator recovery is pending; inspect this operation before submitting anything else"
		if saveErr := saveUpgradeJournal(dir, j, "recovery-pending"); saveErr != nil {
			return errors.Join(err, saveErr)
		}
	}
	return err
}

type upgradeActiveOperation struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Target  string `json:"target"`
}

type upgradeCompetingOperationError struct{ id string }

func (e *upgradeCompetingOperationError) Error() string {
	return "another agent upgrade is already active: " + e.id
}

func readUpgradeActive(target string) (upgradeActiveOperation, error) {
	var active upgradeActiveOperation
	if err := ReadJSON(filepath.Join(upgradeRoot(target), "active.json"), &active); err != nil {
		return active, err
	}
	if active.Version != Protocol || !ValidID(active.ID) || active.Target != target {
		return active, errors.New("active upgrade marker identity mismatch")
	}
	return active, nil
}

func lockUpgradeTarget(ctx context.Context, req UpgradeRequest) (func(), error) {
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		unlock, err := lockHost(upgradeRoot(req.Target))
		if err == nil {
			return unlock, nil
		}
		if active, e := readUpgradeActive(req.Target); e == nil && active.ID != req.ID {
			return nil, &upgradeCompetingOperationError{id: active.ID}
		} else if e != nil && !os.IsNotExist(e) {
			return nil, e
		}
		select {
		case <-deadline.Done():
			return nil, fmt.Errorf("waiting for target upgrade owner: %w", deadline.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func claimUpgradeActive(req UpgradeRequest) error {
	active, err := readUpgradeActive(req.Target)
	if err == nil {
		if active.ID != req.ID {
			return &upgradeCompetingOperationError{id: active.ID}
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return WriteJSON(filepath.Join(upgradeRoot(req.Target), "active.json"), upgradeActiveOperation{Version: Protocol, ID: req.ID, Target: req.Target})
}

func rejectCompetingUpgrade(dir string, req UpgradeRequest, cause error) error {
	var existing upgradeJournal
	if err := ReadJSON(filepath.Join(dir, "journal.json"), &existing); err == nil {
		if err = validateUpgradeJournal(&existing, req); err != nil {
			return errors.Join(cause, err)
		}
		if existing.Result.Status != "running" {
			if err = WriteJSON(filepath.Join(dir, "result.json"), existing.Result); err != nil {
				return errors.Join(cause, err)
			}
			return cause
		}
	} else if !os.IsNotExist(err) {
		return errors.Join(cause, err)
	}
	j := upgradeJournal{Request: req, Result: UpgradeResult{Version: Protocol, ID: req.ID, Target: req.Target, Status: "failed", Phase: "rejected", Error: cause.Error(), Recovery: "existing active operation was left untouched", Updated: time.Now().UTC()}}
	if err := saveUpgradeJournal(dir, &j, "rejected"); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func suppressUpgradeRecovery(dir string, j *upgradeJournal, s *mgr.Service) error {
	if _, err := upgradeRecoveryDelay(j.Policy); err != nil {
		return err
	}
	j.PolicySuppressed = true
	j.RecoveryDrained = false
	j.DisabledAt = time.Time{}
	j.SuppressionIntent = time.Now().UTC()
	if err := saveUpgradeJournal(dir, j, "suppressing-recovery"); err != nil {
		return err
	}
	if err := s.ResetRecoveryActions(); err != nil {
		return err
	}
	if err := upgradeStartType(s, mgr.StartDisabled, false); err != nil {
		return err
	}
	confirmed := time.Now()
	j.DisabledAt = confirmed.UTC()
	j.disabledMonotonic = confirmed
	return saveUpgradeJournal(dir, j, "recovery-disabled")
}

func drainUpgradeRecovery(ctx context.Context, dir string, j *upgradeJournal, s *mgr.Service) error {
	if !j.PolicySuppressed || j.RecoveryDrained {
		return nil
	}
	if j.DisabledAt.IsZero() || j.disabledMonotonic.IsZero() {
		if err := suppressUpgradeRecovery(dir, j, s); err != nil {
			return err
		}
	}
	cfg, err := s.Config()
	if err != nil {
		return err
	}
	if cfg.StartType != mgr.StartDisabled {
		return errors.New("managed service must remain disabled while draining queued SCM restart actions")
	}
	if err = saveUpgradeJournal(dir, j, "draining-recovery"); err != nil {
		return err
	}
	if err = waitUpgradeRecoveryDrain(ctx, j.Policy, j.disabledMonotonic); err != nil {
		return err
	}
	j.RecoveryDrained = true
	return saveUpgradeJournal(dir, j, "recovery-drained")
}

func restoreDrainedUpgradePolicy(ctx context.Context, dir string, j *upgradeJournal, s *mgr.Service) error {
	if err := drainUpgradeRecovery(ctx, dir, j, s); err != nil {
		return err
	}
	return restoreUpgradePolicy(s, j.Policy)
}

func validateUpgradeJournal(j *upgradeJournal, req UpgradeRequest) error {
	if err := j.Request.Validate(); err != nil {
		return err
	}
	if !reflect.DeepEqual(j.Request, req) {
		return errors.New("upgrade recovery journal belongs to another request")
	}
	return j.Result.Validate(req.Target, req.ID)
}

func runUpgradeCoordinator(ctx context.Context, requestPath string) (retErr error) {
	req, err := readUpgradeRequest(requestPath)
	if err != nil {
		return err
	}
	dir, err := protectedUpgradePath(req)
	if err != nil {
		return err
	}
	lockDir := filepath.Join(dir, "coordinator-lock")
	if err = os.MkdirAll(lockDir, 0700); err != nil {
		return err
	}
	unlock, err := lockHost(lockDir)
	if err != nil {
		return err
	}
	defer unlock()
	// Submission never publishes a maintenance marker. Only this independently
	// supervised service may claim the target, after acquiring the same native
	// lock used by submission. SSH death cannot leave a gate without a runner.
	targetUnlock, err := lockUpgradeTarget(ctx, req)
	if err != nil {
		var competing *upgradeCompetingOperationError
		if errors.As(err, &competing) {
			return rejectCompetingUpgrade(dir, req, err)
		}
		return err
	}
	defer targetUnlock()
	var j upgradeJournal
	journalErr := ReadJSON(filepath.Join(dir, "journal.json"), &j)
	if journalErr == nil {
		if err = validateUpgradeJournal(&j, req); err != nil {
			return err
		}
	} else if !os.IsNotExist(journalErr) {
		return journalErr
	}
	if err = claimUpgradeActive(req); err != nil {
		return rejectCompetingUpgrade(dir, req, err)
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName(req.Target))
	if err != nil {
		return err
	}
	defer s.Close()
	if journalErr == nil {
		if j.Result.Status != "running" {
			return reportUpgradeRecoveryFailure(dir, &j, finalizeTerminalUpgrade(dir, &j, s))
		}
		// Persisted wall times have no monotonic/boot provenance. Reconfirm
		// suppression and drain a full interval after a coordinator restart.
		if j.PolicySuppressed {
			if err = suppressUpgradeRecovery(dir, &j, s); err != nil {
				return reportUpgradeRecoveryFailure(dir, &j, err)
			}
		}
		if !j.Cooperative && !j.Retired {
			if _, e := os.Stat(filepath.Join(dir, "retire-request.json")); e == nil {
				retired, guardianErr := waitUpgradeRetirement(ctx, dir, req)
				if !retired && guardianErr != nil {
					var outcome upgradeRetirementOutcome
					if ReadJSON(filepath.Join(dir, "retire-result.json"), &outcome) != nil {
						j.Result.Error = guardianErr.Error()
						j.Result.Recovery = "retirement guardian outcome pending; service policy remains suppressed"
						_ = saveUpgradeJournal(dir, &j, "guardian-recovery-pending")
						return guardianErr
					}
				}
				j.Retired = retired
			}
		}
		return reportUpgradeRecoveryFailure(dir, &j, recoverUpgrade(ctx, dir, &j, s, errors.New("coordinator restarted before committing upgrade"), false))
	}
	j = upgradeJournal{Request: req, Result: UpgradeResult{Version: Protocol, ID: req.ID, Target: req.Target, Status: "running", Phase: "preflight", Updated: time.Now().UTC()}}
	var heldSteamUnlock func()
	defer func() {
		if retErr != nil {
			retErr = recoverUpgrade(context.Background(), dir, &j, s, retErr, heldSteamUnlock != nil)
			retErr = reportUpgradeRecoveryFailure(dir, &j, retErr)
		}
		if heldSteamUnlock != nil {
			heldSteamUnlock()
		}
	}()
	if err = saveUpgradeJournal(dir, &j, "preflight"); err != nil {
		return err
	}
	candidate, err := ReadAgentIdentity(filepath.Join(dir, "dzdeploy.exe"))
	if err != nil {
		return err
	}
	if err = ValidateUpgradeCandidate(candidate, req.Revision); err != nil {
		return err
	}
	if candidate.SHA256 != req.CandidateSHA256 {
		return errors.New("protected candidate checksum mismatch")
	}
	j.Result.Candidate = &candidate
	var host HostConfig
	if err = ReadJSON(filepath.Join(req.Host.StateDir, "host.json"), &host); err != nil {
		return err
	}
	if !reflect.DeepEqual(host, req.Host) {
		return errors.New("host configuration changed since upgrade inspection")
	}
	hostBytes, err := os.ReadFile(filepath.Join(host.StateDir, "host.json"))
	if err != nil {
		return err
	}
	j.HostSHA256 = Digest(hostBytes)
	if err = upgradeIdle(host); err != nil {
		return err
	}
	steamCtx, cancelSteam := context.WithTimeout(ctx, 2*time.Second)
	steamUnlock, err := acquireSteam(steamCtx, host)
	cancelSteam()
	if err != nil {
		return err
	}
	heldSteamUnlock = steamUnlock
	if err = upgradeNoSteam(host); err != nil {
		return err
	}
	cfg, manager, err := upgradeServiceIdentity(s, host)
	if err != nil {
		return err
	}
	game, err := upgradeGame(host)
	if err != nil {
		return err
	}
	j.Result.Game = game
	j.InitiallyStopped = manager.PID == 0
	if !j.InitiallyStopped && game.PID == 0 {
		return errors.New("running manager with no game requires a separate managed recovery before upgrade")
	}
	previous, identityErr := ReadAgentIdentity(manager.Executable)
	if identityErr != nil {
		raw, e := os.ReadFile(manager.Executable)
		if e != nil {
			return e
		}
		previous = AgentIdentity{Protocol: Protocol, SHA256: Digest(raw), GOOS: "windows", GOARCH: "amd64", Dirty: true}
	}
	previous.PID = manager.PID
	previous.Token = manager.Token
	j.Result.Previous = &previous
	if !j.InitiallyStopped {
		j.GameParent, err = verifyUpgradeAncestry(req.Target, game, manager, previous)
		if err != nil {
			return err
		}
	}
	var heartbeat Heartbeat
	if !j.InitiallyStopped {
		if err = ReadJSON(filepath.Join(host.StateDir, "heartbeat.json"), &heartbeat); err != nil {
			return err
		}
		if heartbeat.Version != Protocol || time.Since(heartbeat.Updated) > 15*time.Second || time.Until(heartbeat.Updated) > 15*time.Second {
			return errors.New("current agent heartbeat is stale or incompatible")
		}
		if heartbeat.Agent != nil && (heartbeat.Agent.Protocol != Protocol || heartbeat.Agent.SHA256 != previous.SHA256 || heartbeat.Agent.PID != manager.PID || heartbeat.Agent.Token != manager.Token) {
			return errors.New("running agent provenance differs from its installed executable or SCM process")
		}
		for _, capability := range heartbeat.Capabilities {
			j.Cooperative = j.Cooperative || capability == AgentHandoffCapability
		}
	}
	if previous.SHA256 == candidate.SHA256 {
		if !j.InitiallyStopped && (heartbeat.Agent == nil || heartbeat.Agent.SHA256 != previous.SHA256 || heartbeat.Agent.PID != manager.PID || heartbeat.Agent.Token != manager.Token) {
			return errors.New("unchanged upgrade requires heartbeat provenance from the actual running agent")
		}
		j.Result.Status = "unchanged"
		j.Result.Continuity = true
		j.Result.StartupPending = j.InitiallyStopped
		if err = saveUpgradeJournal(dir, &j, "complete"); err != nil {
			return err
		}
		return removeUpgradeActive(req)
	}
	if j.Policy, err = snapshotUpgradePolicy(s, cfg); err != nil {
		return err
	}
	if _, err = upgradeRecoveryDelay(j.Policy); err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(manager.Executable, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	j.BinaryACL = sd.String()
	if err = AtomicCopy(manager.Executable, filepath.Join(dir, "previous.exe"), 0700); err != nil {
		return err
	}
	if err = AtomicCopy(filepath.Join(host.StateDir, "state.json"), filepath.Join(dir, "state-before.json"), 0600); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, &j, "prepared"); err != nil {
		return err
	}
	sid, err := upgradeServiceSID(cfg)
	if err != nil {
		return err
	}
	inboxSD, err := windows.GetNamedSecurityInfo(filepath.Join(host.StateDir, "inbox"), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	j.InboxACL = inboxSD.String()
	j.InboxGated = true
	if err = saveUpgradeJournal(dir, &j, "gating"); err != nil {
		return err
	}
	if err = gateUpgradeInbox(filepath.Join(host.StateDir, "inbox"), sid); err != nil {
		return err
	}
	if err = upgradeIdle(host); err != nil {
		return err
	}
	// Persist the intent before suppressing policy; replay always knows that a
	// stopped manager must not be started without the candidate hold protocol.
	if err = suppressUpgradeRecovery(dir, &j, s); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, &j, "retiring"); err != nil {
		return err
	}
	if j.InitiallyStopped {
		if status, e := s.Query(); e != nil || status.State != svc.Stopped {
			return errors.New("stopped service changed state during upgrade preflight")
		}
	} else if j.Cooperative {
		if _, err = s.Control(svc.Cmd(HandoffControl)); err != nil {
			return err
		}
		stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = waitUpgradeService(stopCtx, s, svc.Stopped)
		cancel()
		if err != nil {
			return err
		}
	} else {
		if err = retireUpgradeLegacy(ctx, dir, req, manager, game); err != nil {
			return err
		}
		stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = waitUpgradeService(stopCtx, s, svc.Stopped)
		cancel()
		if err != nil {
			return err
		}
	}
	j.Retired = true
	if err = saveUpgradeJournal(dir, &j, "retired"); err != nil {
		return err
	}
	stateUnlock, err := lockHost(host.StateDir)
	if err != nil {
		return err
	}
	installUnlock, err := lockHost(host.InstallDir)
	if err != nil {
		stateUnlock()
		return err
	}
	locked := true
	defer func() {
		if locked {
			installUnlock()
			stateUnlock()
		}
	}()
	if err = upgradeIdle(host); err != nil {
		return err
	}
	if err = upgradeContinuity(host, game); err != nil {
		return err
	}
	j.Replaced = true
	if err = saveUpgradeJournal(dir, &j, "replacing"); err != nil {
		return err
	}
	if err = replaceUpgradeBinary(filepath.Join(dir, "dzdeploy.exe"), manager.Executable, j.BinaryACL, candidate.SHA256); err != nil {
		return err
	}
	if err = verifyUpgradeHostUnchanged(&j); err != nil {
		return err
	}
	if j.InitiallyStopped {
		if err = upgradeContinuity(host, game); err != nil {
			return err
		}
		if err = restoreUpgradeInbox(&j); err != nil {
			return err
		}
		if err = restoreDrainedUpgradePolicy(ctx, dir, &j, s); err != nil {
			return err
		}
		j.Result.Status = "succeeded"
		j.Result.Continuity = true
		j.Result.StartupPending = true
		if err = saveUpgradeJournal(dir, &j, "installed-awaiting-startup"); err != nil {
			return err
		}
		return removeUpgradeActive(req)
	}
	// Target-root traversal is read-only. The sole service-writable artifact is
	// ack.json; journals, decisions, executables and adoption proof stay protected.
	if err = upgradeSetDACL(upgradeRoot(req.Target), "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FRFX;;;"+sid+")"); err != nil {
		return err
	}
	if err = upgradeSetDACL(filepath.Dir(upgradeRoot(req.Target)), "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;;FRFX;;;AU)"); err != nil {
		return err
	}
	if err = upgradeSetDACL(filepath.Dir(filepath.Dir(upgradeRoot(req.Target))), "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;;FRFX;;;AU)"); err != nil {
		return err
	}
	hold := UpgradeHold{Version: Protocol, ID: req.ID, Target: req.Target, Game: game, Previous: previous, CandidateSHA256: candidate.SHA256, Revision: candidate.Revision}
	if err = prepareUpgradeHold(dir, hold, sid); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, &j, "starting"); err != nil {
		return err
	}
	if err = drainUpgradeRecovery(ctx, dir, &j, s); err != nil {
		return err
	}
	if err = upgradeStartType(s, mgr.StartManual, false); err != nil {
		return err
	}
	if err = s.Start("dzdeploy-upgrade=" + requestPath); err != nil {
		return err
	}
	startedCtx, cancelStarted := context.WithTimeout(ctx, 10*time.Second)
	err = waitUpgradeService(startedCtx, s, svc.Running)
	cancelStarted()
	if err != nil {
		return err
	}
	status, err := s.Query()
	if err != nil {
		return err
	}
	candidate.PID = int(status.ProcessId)
	candidate.Token, err = processToken(candidate.PID, manager.Executable)
	if err != nil {
		return err
	}
	j.Result.Candidate = &candidate
	if err = saveUpgradeJournal(dir, &j, "awaiting-acknowledgement"); err != nil {
		return err
	}
	ackCtx, cancelAck := context.WithTimeout(ctx, 30*time.Second)
	ack, err := waitUpgradeAck(ackCtx, dir, req, game, s)
	cancelAck()
	if err != nil {
		return err
	}
	j.Result.Candidate = &ack.Agent
	if err = upgradeIdle(host); err != nil {
		return err
	}
	if err = upgradeContinuity(host, game); err != nil {
		return err
	}
	if err = verifyUpgradeHostUnchanged(&j); err != nil {
		return err
	}
	proof := AdoptionProof{Version: Protocol, ID: req.ID, Target: req.Target, Game: game, Previous: previous, Agent: ack.Agent, Parent: j.GameParent, Updated: time.Now().UTC()}
	if err = WriteJSON(filepath.Join(upgradeRoot(req.Target), "adoption.json"), proof); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, &j, "accepted"); err != nil {
		return err
	}
	if err = restoreDrainedUpgradePolicy(ctx, dir, &j, s); err != nil {
		return err
	}
	// Release installation locks before the held agent enters RunAgent. Commit
	// is bound to this exact new process, so a stale acknowledgement cannot win.
	installUnlock()
	stateUnlock()
	locked = false
	decision := UpgradeDecision{Version: Protocol, ID: req.ID, Target: req.Target, Decision: "commit", Agent: ProcessIdentity{PID: ack.Agent.PID, Token: ack.Agent.Token, Executable: manager.Executable}}
	j.Committed = true
	if err = saveUpgradeJournal(dir, &j, "committing"); err != nil {
		return err
	}
	if err = WriteJSON(filepath.Join(dir, "decision.json"), decision); err != nil {
		j.Committed = false
		return err
	}
	if err = waitUpgradeHeartbeat(ctx, host, ack.Agent, game); err != nil {
		return err
	}
	if err = restoreUpgradeInbox(&j); err != nil {
		return err
	}
	j.Result.Status = "succeeded"
	j.Result.Continuity = true
	j.Result.StartupPending = false
	if err = saveUpgradeJournal(dir, &j, "complete"); err != nil {
		return err
	}
	return removeUpgradeActive(req)
}

func finalizeTerminalUpgrade(dir string, j *upgradeJournal, s *mgr.Service) error {
	// Journal and public receipt are separate atomic files. Replay repairs a
	// crash between either write and the final active-marker removal.
	if j.Result.Phase != "recovery-required" {
		if err := restoreUpgradeInbox(j); err != nil {
			return err
		}
		if j.Policy.StartType != 0 {
			if err := restoreDrainedUpgradePolicy(context.Background(), dir, j, s); err != nil {
				return err
			}
		}
	}
	if err := saveUpgradeJournal(dir, j, j.Result.Phase); err != nil {
		return err
	}
	if j.Result.Phase == "recovery-required" {
		return nil
	}
	return removeUpgradeActive(j.Request)
}

func replaceUpgradeBinary(source, dest, acl, sha string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if Digest(raw) != sha {
		return errors.New("replacement agent checksum mismatch")
	}
	tmp := dest + ".upgrade-" + NewID()
	defer os.Remove(tmp)
	if err = AtomicWrite(tmp, raw, 0700); err != nil {
		return err
	}
	if err = upgradeSetDACL(tmp, acl); err != nil {
		return err
	}
	if err = replaceFile(tmp, dest); err != nil {
		return err
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	if Digest(got) != sha {
		return errors.New("installed agent checksum mismatch")
	}
	return nil
}

func waitUpgradeHeartbeat(ctx context.Context, c HostConfig, agent AgentIdentity, game ProcessIdentity) error {
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	started := time.Now().UTC()
	for {
		if err := upgradeContinuity(c, game); err != nil {
			return fmt.Errorf("game continuity lost after manager adoption: %w", err)
		}
		if err := upgradeIdentityAlive(ProcessIdentity{PID: agent.PID, Token: agent.Token, Executable: filepath.Join(c.StateDir, "bin", "dzdeploy.exe")}); err != nil {
			return err
		}
		var heartbeat struct {
			Version int            `json:"version"`
			Updated time.Time      `json:"updated"`
			Agent   *AgentIdentity `json:"agent"`
		}
		if ReadJSON(filepath.Join(c.StateDir, "heartbeat.json"), &heartbeat) == nil && heartbeat.Version == Protocol && heartbeat.Updated.After(started) && heartbeat.Agent != nil && heartbeat.Agent.PID == agent.PID && heartbeat.Agent.Token == agent.Token && heartbeat.Agent.SHA256 == agent.SHA256 {
			return nil
		}
		select {
		case <-deadline.Done():
			return errors.New("replacement manager did not publish a fresh matching heartbeat")
		case <-ticker.C:
		}
	}
}

func removeUpgradeActive(req UpgradeRequest) error {
	p := filepath.Join(upgradeRoot(req.Target), "active.json")
	active, err := readUpgradeActive(req.Target)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if active.ID != req.ID || active.Target != req.Target {
		return errors.New("active upgrade belongs to another operation")
	}
	return os.Remove(p)
}

func pauseUpgradeState(c HostConfig) error {
	var state State
	if err := ReadJSON(filepath.Join(c.StateDir, "state.json"), &state); err != nil {
		return err
	}
	if state.Version != Protocol || state.Target != c.Target {
		return errors.New("cannot pause foreign upgrade state")
	}
	state.Paused = true
	state.Updated = time.Now().UTC()
	return WriteJSON(filepath.Join(c.StateDir, "state.json"), state)
}

// Recovery never starts a legacy manager: it cannot acknowledge a hold, and
// could relaunch DayZ if the game exits between inspection and startup.
func recoverUpgrade(ctx context.Context, dir string, j *upgradeJournal, s *mgr.Service, cause error, steamHeld bool) error {
	host := j.Request.Host
	if j.PolicySuppressed && !steamHeld {
		steamCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		unlock, err := acquireSteam(steamCtx, host)
		cancel()
		if err != nil {
			return errors.Join(cause, err)
		}
		defer unlock()
	}
	if !j.Cooperative && !j.Retired {
		if _, e := os.Stat(filepath.Join(dir, "retire-request.json")); e == nil {
			retired, e := waitUpgradeRetirement(ctx, dir, j.Request)
			if e != nil && !retired {
				var terminal upgradeRetirementOutcome
				if ReadJSON(filepath.Join(dir, "retire-result.json"), &terminal) != nil {
					j.Result.Error = e.Error()
					j.Result.Recovery = "existing retirement guardian has no terminal outcome; service policy remains suppressed"
					_ = saveUpgradeJournal(dir, j, "guardian-recovery-pending")
					return errors.Join(cause, e)
				}
			}
			j.Retired = retired
		}
	}
	// A persisted commit-intent bit is not proof of publication. The durable
	// process-bound decision and acknowledgement determine replay's next action.
	if agent, ok := committedUpgradeAgent(dir, j); ok {
		if err := upgradeContinuity(host, j.Result.Game); err == nil {
			if err = waitUpgradeHeartbeat(ctx, host, agent, j.Result.Game); err == nil {
				if err = restoreUpgradeInbox(j); err != nil {
					return errors.Join(cause, err)
				}
				if err = restoreDrainedUpgradePolicy(ctx, dir, j, s); err != nil {
					return errors.Join(cause, err)
				}
				j.Result.Continuity = true
				j.Result.StartupPending = false
				if j.RollingBack {
					j.Result.Status = "failed"
					j.Result.Error = cause.Error()
					j.Result.Recovery = "previous agent restored and adopted the unchanged game"
				} else {
					j.Result.Status = "succeeded"
					j.Result.Error = ""
					j.Result.Recovery = "coordinator recovered and verified the accepted agent"
				}
				if err = saveUpgradeJournal(dir, j, "complete"); err != nil {
					return errors.Join(cause, err)
				}
				if err = removeUpgradeActive(j.Request); err != nil {
					return errors.Join(cause, err)
				}
				if j.RollingBack {
					return cause
				}
				return nil
			}
		}
		// The active marker still inhibits all queue and game actions. A failed
		// acceptance can therefore retire this verified manager without game I/O.
	}
	status, err := s.Query()
	if err != nil {
		return errors.Join(cause, err)
	}
	if !j.Retired && !j.Replaced && j.Result.Previous != nil && status.State == svc.Running && int(status.ProcessId) == j.Result.Previous.PID {
		manager := ProcessIdentity{PID: j.Result.Previous.PID, Token: j.Result.Previous.Token, Executable: filepath.Join(host.StateDir, "bin", "dzdeploy.exe")}
		if upgradeIdentityAlive(manager) == nil && upgradeContinuity(host, j.Result.Game) == nil {
			if j.PolicySuppressed {
				if err = drainUpgradeRecovery(ctx, dir, j, s); err != nil {
					return errors.Join(cause, err)
				}
			}
			if continuityErr := upgradeContinuity(host, j.Result.Game); continuityErr == nil {
				if err = restoreUpgradeInbox(j); err != nil {
					return errors.Join(cause, err)
				}
				if j.PolicySuppressed {
					if err = restoreUpgradePolicy(s, j.Policy); err != nil {
						return errors.Join(cause, err)
					}
				}
				j.Result.Status = "failed"
				j.Result.Error = cause.Error()
				j.Result.Recovery = "original manager retained; original game continues"
				j.Result.Continuity = true
				if err = saveUpgradeJournal(dir, j, "aborted"); err != nil {
					return errors.Join(cause, err)
				}
				_ = removeUpgradeActive(j.Request)
				return cause
			} else {
				cause = errors.Join(cause, continuityErr)
			}
		}
	}
	if !j.PolicySuppressed {
		if err = restoreUpgradeInbox(j); err != nil {
			return errors.Join(cause, err)
		}
		j.Result.Status = "failed"
		j.Result.Error = cause.Error()
		j.Result.Recovery = "preflight rejected before manager replacement"
		_ = saveUpgradeJournal(dir, j, "rejected")
		_ = removeUpgradeActive(j.Request)
		return cause
	}
	if err = suppressUpgradeRecovery(dir, j, s); err != nil {
		return errors.Join(cause, err)
	}
	// A held candidate has not entered RunAgent. Retiring that verified process
	// uses its retained handle and never SCM Stop (which would stop the game).
	status, err = s.Query()
	if err != nil {
		return errors.Join(cause, err)
	}
	if status.State != svc.Stopped {
		var expected *AgentIdentity
		if j.Result.Candidate != nil && int(status.ProcessId) == j.Result.Candidate.PID {
			expected = j.Result.Candidate
		}
		if expected == nil && j.Result.Previous != nil && int(status.ProcessId) == j.Result.Previous.PID {
			expected = j.Result.Previous
		}
		if j.RollbackAgent != nil && int(status.ProcessId) == j.RollbackAgent.PID {
			expected = j.RollbackAgent
		}
		if expected == nil {
			// Crash may fall between StartService and capturing its PID. Prove the
			// exact SCM executable and installed protected digest before retaining
			// a handle; never identify an arbitrary process by name alone.
			_, live, e := upgradeServiceIdentity(s, host)
			if e != nil {
				return errors.Join(cause, e)
			}
			raw, e := os.ReadFile(live.Executable)
			if e != nil {
				return errors.Join(cause, e)
			}
			sha := Digest(raw)
			if j.Result.Candidate != nil && sha == j.Result.Candidate.SHA256 {
				copy := *j.Result.Candidate
				copy.PID = live.PID
				copy.Token = live.Token
				expected = &copy
				j.Result.Candidate = expected
			}
			if expected == nil && j.RollingBack && j.Result.Previous != nil && sha == j.Result.Previous.SHA256 {
				copy := *j.Result.Previous
				copy.PID = live.PID
				copy.Token = live.Token
				expected = &copy
				j.RollbackAgent = expected
			}
			if expected == nil {
				return errors.Join(cause, errors.New("unknown manager identity blocks automatic rollback"))
			}
			if e = saveUpgradeJournal(dir, j, "recovering-manager"); e != nil {
				return errors.Join(cause, e)
			}
		}
		if err = terminateUpgradeManager(ProcessIdentity{PID: expected.PID, Token: expected.Token, Executable: filepath.Join(host.StateDir, "bin", "dzdeploy.exe")}); err != nil {
			return errors.Join(cause, err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = waitUpgradeService(waitCtx, s, svc.Stopped)
		cancel()
		if err != nil {
			return errors.Join(cause, err)
		}
	}
	stateUnlock, err := lockHost(host.StateDir)
	if err != nil {
		return errors.Join(cause, err)
	}
	installUnlock, err := lockHost(host.InstallDir)
	if err != nil {
		stateUnlock()
		return errors.Join(cause, err)
	}
	locksHeld := true
	release := func() {
		if locksHeld {
			installUnlock()
			stateUnlock()
			locksHeld = false
		}
	}
	defer release()
	j.RollingBack = true
	if err = saveUpgradeJournal(dir, j, "restoring-previous"); err != nil {
		return errors.Join(cause, err)
	}
	if j.Replaced && j.Result.Previous != nil {
		if err = replaceUpgradeBinary(filepath.Join(dir, "previous.exe"), filepath.Join(host.StateDir, "bin", "dzdeploy.exe"), j.BinaryACL, j.Result.Previous.SHA256); err != nil {
			return errors.Join(cause, err)
		}
	}
	if j.Cooperative && !j.InitiallyStopped && j.Result.Previous != nil && upgradeContinuity(host, j.Result.Game) == nil && upgradeIdle(host) == nil && ValidateUpgradeCandidate(*j.Result.Previous, j.Result.Previous.Revision) == nil {
		if err = resumePreviousUpgradeAgent(ctx, dir, j, s, release); err == nil {
			j.Result.Status = "failed"
			j.Result.Error = cause.Error()
			j.Result.Recovery = "previous agent restored and adopted the unchanged game"
			j.Result.Continuity = true
			j.Result.StartupPending = false
			if err = saveUpgradeJournal(dir, j, "rolled-back"); err != nil {
				return errors.Join(cause, err)
			}
			if err = removeUpgradeActive(j.Request); err != nil {
				return errors.Join(cause, err)
			}
			return cause
		}
		cause = errors.Join(cause, fmt.Errorf("cooperative agent rollback failed: %w", err))
		if err = suppressUpgradeRecovery(dir, j, s); err != nil {
			return errors.Join(cause, err)
		}
		// A rollback candidate may have started but failed acknowledgement. Keep
		// normal supervision blocked and retire only its verified manager handle.
		if st, e := s.Query(); e == nil && st.State != svc.Stopped {
			_, p, e := upgradeServiceIdentity(s, host)
			if e != nil {
				return errors.Join(cause, e)
			}
			raw, e := os.ReadFile(p.Executable)
			if e != nil || Digest(raw) != j.Result.Previous.SHA256 {
				return errors.Join(cause, errors.New("rollback executable identity changed"))
			}
			if e = terminateUpgradeManager(p); e != nil {
				return errors.Join(cause, e)
			}
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			e = waitUpgradeService(waitCtx, s, svc.Stopped)
			cancel()
			if e != nil {
				return errors.Join(cause, e)
			}
		}
		if !locksHeld {
			stateUnlock, err = lockHost(host.StateDir)
			if err != nil {
				return errors.Join(cause, err)
			}
			installUnlock, err = lockHost(host.InstallDir)
			if err != nil {
				stateUnlock()
				return errors.Join(cause, err)
			}
			locksHeld = true
		}
	}
	if err = pauseUpgradeState(host); err != nil {
		return errors.Join(cause, fmt.Errorf("paused state could not be persisted; service stays disabled: %w", err))
	}
	if err = restoreUpgradeInbox(j); err != nil {
		return errors.Join(cause, err)
	}
	if err = restoreDrainedUpgradePolicy(ctx, dir, j, s); err != nil {
		return errors.Join(cause, err)
	}
	j.Result.Status = "failed"
	j.Result.Error = cause.Error()
	j.Result.Recovery = "previous executable restored; service stopped and supervision paused; explicit recovery required"
	j.Result.Continuity = upgradeContinuity(host, j.Result.Game) == nil
	j.Result.StartupPending = true
	if err = saveUpgradeJournal(dir, j, "recovery-required"); err != nil {
		return errors.Join(cause, err)
	}
	// Keep active.json while recovery is required. New agents refuse ordinary
	// boot, and the explicit operation remains discoverable after reconnecting.
	return cause
}

func committedUpgradeAgent(dir string, j *upgradeJournal) (AgentIdentity, bool) {
	var decision UpgradeDecision
	var ack UpgradeAcknowledgement
	if ReadJSON(filepath.Join(dir, "decision.json"), &decision) != nil || ReadJSON(filepath.Join(dir, "ack.json"), &ack) != nil {
		return AgentIdentity{}, false
	}
	if decision.Version != Protocol || decision.ID != j.Request.ID || decision.Target != j.Request.Target || decision.Decision != "commit" || ack.Version != Protocol || ack.ID != decision.ID || ack.Target != decision.Target || !upgradeSameGame(ack.Game, j.Result.Game) || decision.Agent.PID != ack.Agent.PID || decision.Agent.Token != ack.Agent.Token {
		return AgentIdentity{}, false
	}
	expected := j.Result.Candidate
	if j.RollingBack {
		expected = j.Result.Previous
	}
	if expected == nil || expected.SHA256 != ack.Agent.SHA256 || expected.Revision != ack.Agent.Revision {
		return AgentIdentity{}, false
	}
	if !strings.EqualFold(decision.Agent.Executable, filepath.Join(j.Request.Host.StateDir, "bin", "dzdeploy.exe")) {
		return AgentIdentity{}, false
	}
	return ack.Agent, true
}

func resumePreviousUpgradeAgent(ctx context.Context, dir string, j *upgradeJournal, s *mgr.Service, release func()) error {
	previous := *j.Result.Previous
	cfg, err := s.Config()
	if err != nil {
		return err
	}
	sid, err := upgradeServiceSID(cfg)
	if err != nil {
		return err
	}
	hold := UpgradeHold{Version: Protocol, ID: j.Request.ID, Target: j.Request.Target, Game: j.Result.Game, Previous: previous, CandidateSHA256: previous.SHA256, Revision: previous.Revision}
	if err = prepareUpgradeHold(dir, hold, sid); err != nil {
		return err
	}
	if err = drainUpgradeRecovery(ctx, dir, j, s); err != nil {
		return err
	}
	if err = upgradeStartType(s, mgr.StartManual, false); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, j, "starting-previous"); err != nil {
		return err
	}
	if err = s.Start("dzdeploy-upgrade=" + filepath.Join(dir, "request.json")); err != nil {
		return err
	}
	req := j.Request
	req.CandidateSHA256 = previous.SHA256
	req.Revision = previous.Revision
	ackCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	ack, err := waitUpgradeAck(ackCtx, dir, req, j.Result.Game, s)
	cancel()
	if err != nil {
		return err
	}
	j.RollbackAgent = &ack.Agent
	proof := AdoptionProof{Version: Protocol, ID: req.ID, Target: req.Target, Game: j.Result.Game, Previous: previous, Agent: ack.Agent, Parent: j.GameParent, Updated: time.Now().UTC()}
	if err = WriteJSON(filepath.Join(upgradeRoot(req.Target), "adoption.json"), proof); err != nil {
		return err
	}
	if err = saveUpgradeJournal(dir, j, "previous-accepted"); err != nil {
		return err
	}
	if err = restoreDrainedUpgradePolicy(ctx, dir, j, s); err != nil {
		return err
	}
	release()
	decision := UpgradeDecision{Version: Protocol, ID: req.ID, Target: req.Target, Decision: "commit", Agent: ProcessIdentity{PID: ack.Agent.PID, Token: ack.Agent.Token, Executable: filepath.Join(req.Host.StateDir, "bin", "dzdeploy.exe")}}
	if err = WriteJSON(filepath.Join(dir, "decision.json"), decision); err != nil {
		return err
	}
	if err = waitUpgradeHeartbeat(ctx, req.Host, ack.Agent, j.Result.Game); err != nil {
		return err
	}
	return restoreUpgradeInbox(j)
}

func terminateUpgradeManager(p ProcessIdentity) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(p.PID))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	token, err := handleIdentity(h, p.Executable)
	if err != nil {
		return err
	}
	if token != p.Token {
		return errors.New("manager creation token changed; refusing rollback termination")
	}
	if err = windows.TerminateProcess(h, 1); err != nil {
		return err
	}
	result, err := windows.WaitForSingleObject(h, 10000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("verified manager did not exit during rollback")
	}
	return nil
}

type upgradeRetirementRequest struct {
	Request UpgradeRequest  `json:"request"`
	Manager ProcessIdentity `json:"manager"`
	Game    ProcessIdentity `json:"game"`
}
type upgradeRetirementOutcome struct {
	Version int       `json:"version"`
	ID      string    `json:"id"`
	Target  string    `json:"target"`
	Retired bool      `json:"retired"`
	Error   string    `json:"error,omitempty"`
	Updated time.Time `json:"updated"`
}

// Retirement runs in an independent guardian process. A coordinator crash must
// not detach and resume a frozen legacy manager while DayZ is exiting.
func retireUpgradeLegacy(ctx context.Context, dir string, req UpgradeRequest, manager, game ProcessIdentity) error {
	p := filepath.Join(dir, "retire-request.json")
	if err := WriteJSON(p, upgradeRetirementRequest{Request: req, Manager: manager, Game: game}); err != nil {
		return err
	}
	_, err := waitUpgradeRetirement(ctx, dir, req)
	return err
}

func startUpgradeGuardian(dir string) error {
	p := filepath.Join(dir, "retire-request.json")
	cmd := exec.Command(filepath.Join(dir, "coordinator.exe"), "agent-upgrade-retire", "--request", p)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	identity := ProcessIdentity{PID: cmd.Process.Pid, Executable: filepath.Join(dir, "coordinator.exe")}
	token, err := processToken(identity.PID, identity.Executable)
	identity.Token = token
	if err == nil {
		err = WriteJSON(filepath.Join(dir, "guardian.json"), identity)
	}
	_ = cmd.Process.Release()
	return err
}

type upgradeGuardianNative struct {
	alive  func(ProcessIdentity) (bool, error)
	start  func(string) error
	retire func(ProcessIdentity) error
}

// nativeUpgradeIdentityAlive distinguishes a gone/reused PID from inability to
// inspect a process. Access failures never count as proof of manager retirement.
func nativeUpgradeIdentityAlive(p ProcessIdentity) (bool, error) {
	if p.PID <= 0 || p.Token == "" || !filepath.IsAbs(p.Executable) {
		return false, errors.New("invalid native upgrade process identity")
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(p.PID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err = windows.GetExitCodeProcess(h, &code); err != nil {
		return false, err
	}
	if code != 259 {
		return false, nil
	}
	var name [32768]uint16
	size := uint32(len(name))
	if err = windows.QueryFullProcessImageName(h, 0, &name[0], &size); err != nil {
		return false, err
	}
	if !strings.EqualFold(windows.UTF16ToString(name[:size]), p.Executable) {
		return false, nil
	}
	token, err := handleIdentity(h, p.Executable)
	if err != nil {
		return false, err
	}
	return token == p.Token, nil
}

func retireUpgradeManagerWithoutGame(p ProcessIdentity) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := upgradeDebugPrivilege()
	if err != nil {
		return err
	}
	defer restore()
	h, err := upgradeDebugOpen(p, windows.PROCESS_TERMINATE)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if err = windows.TerminateProcess(h, 1); err != nil {
		return err
	}
	result, err := windows.WaitForSingleObject(h, 10000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("verified legacy manager did not exit after continuity loss")
	}
	return nil
}

func validateUpgradeRetirementOutcome(r upgradeRetirementOutcome, req UpgradeRequest) error {
	if r.Version != Protocol || r.ID != req.ID || r.Target != req.Target {
		return errors.New("guardian retirement outcome identity mismatch")
	}
	return nil
}

// Reconcile under the guardian's native lock before interpreting missing
// receipts. This also closes the crash gap between intent and CreateProcess.
// A restarted guardian uses the original request/operation and competes for
// this same lock, so only one process can freeze or retire the manager.
func reconcileUpgradeGuardian(dir string, req UpgradeRequest, native upgradeGuardianNative) (bool, error) {
	var r upgradeRetirementRequest
	if err := ReadJSON(filepath.Join(dir, "retire-request.json"), &r); err != nil {
		return false, err
	}
	if !reflect.DeepEqual(r.Request, req) || r.Request.Validate() != nil {
		return false, errors.New("guardian request identity mismatch")
	}
	if !strings.EqualFold(r.Manager.Executable, filepath.Join(req.Host.StateDir, "bin", "dzdeploy.exe")) || !strings.EqualFold(r.Game.Executable, filepath.Join(req.Host.InstallDir, "DayZServer_x64.exe")) {
		return false, errors.New("guardian process paths differ from the managed installation")
	}
	var guardian ProcessIdentity
	guardianAlive := false
	if err := ReadJSON(filepath.Join(dir, "guardian.json"), &guardian); err == nil {
		if !strings.EqualFold(guardian.Executable, filepath.Join(dir, "coordinator.exe")) {
			return false, errors.New("guardian executable escaped the protected operation")
		}
		var e error
		guardianAlive, e = native.alive(guardian)
		if e != nil {
			return false, e
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	lockDir := filepath.Join(dir, "guardian-lock")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return false, err
	}
	unlock, err := lockHost(lockDir)
	if err != nil {
		return false, nil
	}
	defer unlock()
	var outcome upgradeRetirementOutcome
	if err = ReadJSON(filepath.Join(dir, "retire-result.json"), &outcome); err == nil {
		return true, validateUpgradeRetirementOutcome(outcome, req)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if guardianAlive {
		return false, nil
	}
	managerAlive, err := native.alive(r.Manager)
	if err != nil {
		return false, err
	}
	gameAlive, err := native.alive(r.Game)
	if err != nil {
		return false, err
	}
	if !managerAlive {
		outcome = upgradeRetirementOutcome{Version: Protocol, ID: req.ID, Target: req.Target, Retired: true, Updated: time.Now().UTC(), Error: "retirement guardian exited without a receipt; original manager is confirmed gone; paused recovery required"}
		return true, WriteJSON(filepath.Join(dir, "retire-result.json"), outcome)
	}
	if !gameAlive {
		if err = native.retire(r.Manager); err != nil {
			return false, err
		}
		outcome = upgradeRetirementOutcome{Version: Protocol, ID: req.ID, Target: req.Target, Retired: true, Updated: time.Now().UTC(), Error: "original game exited while retirement guardian was absent; only the verified manager was retired; paused recovery required"}
		return true, WriteJSON(filepath.Join(dir, "retire-result.json"), outcome)
	}
	// Start while retaining the lock; the helper acquires it with a bounded
	// wait after this function releases. No operation ID or intent is replaced.
	return false, native.start(dir)
}

func waitUpgradeRetirement(ctx context.Context, dir string, req UpgradeRequest) (bool, error) {
	deadline, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var result upgradeRetirementOutcome
		if err := ReadJSON(filepath.Join(dir, "retire-result.json"), &result); err == nil {
			if err = validateUpgradeRetirementOutcome(result, req); err != nil {
				return false, err
			}
			if result.Error != "" {
				return result.Retired, errors.New(result.Error)
			}
			if !result.Retired {
				return false, errors.New("guardian did not retire the legacy manager")
			}
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
		if ready, e := reconcileUpgradeGuardian(dir, req, upgradeGuardianNative{alive: nativeUpgradeIdentityAlive, start: startUpgradeGuardian, retire: retireUpgradeManagerWithoutGame}); e != nil {
			return false, e
		} else if ready {
			continue
		}
		select {
		case <-deadline.Done():
			return false, errors.New("legacy retirement guardian has no terminal outcome; inspect existing operation before recovery")
		case <-ticker.C:
		}
	}
}

func RunUpgradeRetirement(requestPath string) (retErr error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("legacy retirement guardian requires an elevated administrator")
	}
	var r upgradeRetirementRequest
	if err := ReadJSON(requestPath, &r); err != nil {
		return err
	}
	if err := r.Request.Validate(); err != nil {
		return err
	}
	dir, err := protectedUpgradePath(r.Request)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(requestPath), filepath.Join(dir, "retire-request.json")) {
		return errors.New("retirement guardian requires its protected operation request")
	}
	lockDir := filepath.Join(dir, "guardian-lock")
	if err = os.MkdirAll(lockDir, 0700); err != nil {
		return err
	}
	lockCtx, cancelLock := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLock()
	var unlock func()
	for {
		unlock, err = lockHost(lockDir)
		if err == nil {
			break
		}
		select {
		case <-lockCtx.Done():
			return errors.New("another guardian owns this retirement")
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer unlock()
	var existing upgradeRetirementOutcome
	if ReadJSON(filepath.Join(dir, "retire-result.json"), &existing) == nil {
		if err = validateUpgradeRetirementOutcome(existing, r.Request); err != nil {
			return err
		}
		return errors.New("legacy retirement already has a terminal outcome")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	identity := ProcessIdentity{PID: os.Getpid(), Executable: self}
	identity.Token, err = processToken(identity.PID, self)
	if err != nil {
		return err
	}
	if err = WriteJSON(filepath.Join(dir, "guardian.json"), identity); err != nil {
		return err
	}
	outcome := upgradeRetirementOutcome{Version: Protocol, ID: r.Request.ID, Target: r.Request.Target}
	defer func() {
		if alive, identityErr := nativeUpgradeIdentityAlive(r.Manager); identityErr == nil && !alive {
			outcome.Retired = true
		} else if identityErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("cannot prove original manager retirement: %w", identityErr))
		}
		if retErr != nil {
			outcome.Error = retErr.Error()
		}
		outcome.Updated = time.Now().UTC()
		if err := WriteJSON(filepath.Join(dir, "retire-result.json"), outcome); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()
	if err = upgradeIdle(r.Request.Host); err != nil {
		return err
	}
	if err = upgradeNoSteam(r.Request.Host); err != nil {
		return err
	}
	attachCtx, cancelAttach := context.WithTimeout(context.Background(), 10*time.Second)
	frozen, err := freezeUpgradeManager(attachCtx, r.Manager, r.Game)
	cancelAttach()
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, frozen.Close()) }()
	if err = frozen.ArmCrashRetirement(); err != nil {
		return err
	}
	if err = upgradeIdle(r.Request.Host); err != nil {
		return err
	}
	if err = upgradeNoSteam(r.Request.Host); err != nil {
		return err
	}
	game, err := upgradeGame(r.Request.Host)
	if err != nil {
		return err
	}
	if !upgradeSameGame(game, r.Game) {
		return errors.New("game identity changed while legacy manager was frozen")
	}
	if err = frozen.Retire(); err != nil {
		return err
	}
	outcome.Retired = true
	return nil
}
