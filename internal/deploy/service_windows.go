//go:build windows

package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func ServiceName(alias string) string { return "dzdeploy-" + alias }

type serviceHandler struct{ config HostConfig }

func (h *serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunAgent(ctx, h.config, nil) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown}
	status <- current
	for {
		select {
		case err := <-done:
			if err != nil {
				_ = AtomicWrite(filepath.Join(h.config.StateDir, "logs", "agent-error.txt"), []byte(err.Error()), 0600)
				return true, 1
			}
			return false, 0
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown, svc.PreShutdown:
				cancel()
				current = svc.Status{State: svc.StopPending, WaitHint: uint32((h.config.ShutdownSeconds + 15) * 1000)}
				status <- current
				ticker := time.NewTicker(time.Second)
				for {
					select {
					case err := <-done:
						ticker.Stop()
						if err != nil {
							_ = AtomicWrite(filepath.Join(h.config.StateDir, "logs", "agent-error.txt"), []byte(err.Error()), 0600)
							return true, 1
						}
						return false, 0
					case <-ticker.C:
						current.CheckPoint++
						status <- current
					}
				}
			}
		}
	}
}
func RunService(c HostConfig, console bool) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if console {
		ctx, cancel := signalContext()
		defer cancel()
		return RunAgent(ctx, c, nil)
	}
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return errors.New("agent must be run by Windows SCM (or explicitly with --console)")
	}
	return svc.Run(ServiceName(c.Target), &serviceHandler{config: c})
}

type BootstrapOptions struct {
	Config        HostConfig
	Account       string
	PasswordEnv   string
	PublicKeyFile string
	SSHSources    string
	SSHPort       int
	InstallSSH    bool
}

func readServicePassword(env string) (string, error) {
	if env != "" {
		p, ok := os.LookupEnv(env)
		if !ok || p == "" {
			return "", errors.New("service password environment variable is empty")
		}
		return p, nil
	}
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return "", errors.New("bootstrap needs an interactive terminal or --password-env")
	}
	if err := windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return "", err
	}
	defer windows.SetConsoleMode(h, mode)
	fmt.Fprint(os.Stderr, "Existing service account password: ")
	p, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	p = strings.TrimRight(p, "\r\n")
	if p == "" {
		return "", errors.New("empty service account password")
	}
	return p, err
}
func adminCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	prepareCommand(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", filepath.Base(name), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Bootstrap(ctx context.Context, opt BootstrapOptions) (map[string]string, error) {
	c := opt.Config
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return nil, errors.New("bootstrap must run from an elevated Windows terminal")
	}
	if opt.Account == "" {
		return nil, errors.New("--account must name an existing non-administrator local/domain account")
	}
	sid, _, _, err := windows.LookupSID("", opt.Account)
	if err != nil {
		return nil, fmt.Errorf("service account must already exist: %w", err)
	}
	if err = ensureNoGame(c.InstallDir); err != nil {
		return nil, fmt.Errorf("stop the existing console manager and game before bootstrap: %w", err)
	}
	serviceManager, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer serviceManager.Disconnect()
	existing, openErr := serviceManager.OpenService(ServiceName(c.Target))
	if openErr == nil {
		defer existing.Close()
		status, err := existing.Query()
		if err != nil {
			return nil, err
		}
		if status.State != svc.Stopped {
			return nil, errors.New("service is already installed and running; stop it before re-running bootstrap/upgrade")
		}
	} else if !errors.Is(openErr, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil, openErr
	}
	password, err := readServicePassword(opt.PasswordEnv)
	if err != nil {
		return nil, err
	}
	if err = grantServiceLogon(sid); err != nil {
		return nil, err
	}
	for _, p := range []string{c.StateDir, c.InstallDir, filepath.Dir(c.SteamCMD), c.WorkshopDir} {
		if err = os.MkdirAll(p, 0700); err != nil {
			return nil, err
		}
	}
	// State includes the binary, RCon secret and backups. Restrict inherited access.
	grant := "*" + sid.String() + ":(OI)(CI)F"
	if err = adminCommand(ctx, "icacls.exe", c.StateDir, "/inheritance:r", "/grant:r", grant, "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "/T", "/Q"); err != nil {
		return nil, err
	}
	for _, p := range []string{c.InstallDir, filepath.Dir(c.SteamCMD), c.WorkshopDir} {
		if err = adminCommand(ctx, "icacls.exe", p, "/grant", "*"+sid.String()+":(OI)(CI)M", "/T", "/Q"); err != nil {
			return nil, err
		}
	}
	if err = setupRCon(c); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"", ".dzdeploy-backup"} {
		p := filepath.Join(c.InstallDir, "battleye", "BEServer_x64.cfg") + suffix
		if _, err = os.Stat(p); os.IsNotExist(err) {
			continue
		}
		if err = adminCommand(ctx, "icacls.exe", p, "/inheritance:r", "/grant:r", "*"+sid.String()+":F", "*S-1-5-18:F", "*S-1-5-32-544:F", "/Q"); err != nil {
			return nil, err
		}
	}
	store, err := OpenStore(c)
	if err != nil {
		return nil, err
	}
	if err = store.Save(); err != nil {
		return nil, err
	}
	if err = WriteJSON(filepath.Join(c.StateDir, "host.json"), c); err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(c.StateDir, "bin", "dzdeploy.exe")
	selfAbs, _ := filepath.Abs(self)
	destAbs, _ := filepath.Abs(dest)
	if !strings.EqualFold(selfAbs, destAbs) {
		if err = AtomicWrite(dest, binary, 0700); err != nil {
			return nil, err
		}
	}
	config := mgr.Config{StartType: mgr.StartAutomatic, ErrorControl: mgr.ErrorNormal, ServiceStartName: opt.Account, Password: password, DisplayName: "DayZ Deploy: " + c.Target, Description: "DayZ configuration deployments and supervised startup", Dependencies: []string{"Tcpip"}}
	var service *mgr.Service
	if existing != nil {
		config.BinaryPathName = `"` + dest + `" agent --host-config "` + filepath.Join(c.StateDir, "host.json") + `"`
		err = existing.UpdateConfig(config)
		service = existing
	} else {
		service, err = serviceManager.CreateService(ServiceName(c.Target), dest, config, "agent", "--host-config", filepath.Join(c.StateDir, "host.json"))
		if service != nil {
			defer service.Close()
		}
	}
	if err != nil {
		return nil, err
	}
	if err = service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 30 * time.Second}}, 86400); err != nil {
		return nil, err
	}
	if err = service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return nil, err
	}
	security, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGX;;;" + sid.String() + ")")
	if err != nil {
		return nil, err
	}
	dacl, _, err := security.DACL()
	if err != nil {
		return nil, err
	}
	if err = windows.SetSecurityInfo(windows.Handle(service.Handle), windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return nil, err
	}
	preShutdown := uint32((c.ShutdownSeconds + 30) * 1000)
	if err = windows.ChangeServiceConfig2(service.Handle, windows.SERVICE_CONFIG_PRESHUTDOWN_INFO, (*byte)(unsafe.Pointer(&preShutdown))); err != nil {
		return nil, err
	}
	info := map[string]string{"target": c.Target, "service": ServiceName(c.Target), "stateDir": c.StateDir, "activation": "initial configuration must be deployed with --restart"}
	if opt.InstallSSH {
		fingerprint, err := setupSSH(ctx, opt, sid.String())
		if err != nil {
			return nil, err
		}
		info["hostFingerprint"] = fingerprint
	}
	if err = service.Start(); err != nil {
		return nil, err
	}
	return info, nil
}

func grantServiceLogon(sid *windows.SID) error {
	adv := windows.NewLazySystemDLL("advapi32.dll")
	open := adv.NewProc("LsaOpenPolicy")
	add := adv.NewProc("LsaAddAccountRights")
	closePolicy := adv.NewProc("LsaClose")
	toErr := adv.NewProc("LsaNtStatusToWinError")
	type attrs struct {
		Length     uint32
		Root       uintptr
		Name       uintptr
		Attributes uint32
		Security   uintptr
		Quality    uintptr
	}
	type unicode struct {
		Length  uint16
		Maximum uint16
		Buffer  *uint16
	}
	obj := attrs{}
	obj.Length = uint32(unsafe.Sizeof(obj))
	var policy uintptr
	status, _, _ := open.Call(0, uintptr(unsafe.Pointer(&obj)), 0x810, uintptr(unsafe.Pointer(&policy)))
	if status != 0 {
		code, _, _ := toErr.Call(status)
		return windows.Errno(code)
	}
	defer closePolicy.Call(policy)
	text, err := windows.UTF16FromString("SeServiceLogonRight")
	if err != nil {
		return err
	}
	right := unicode{Length: uint16((len(text) - 1) * 2), Maximum: uint16(len(text) * 2), Buffer: &text[0]}
	status, _, _ = add.Call(policy, uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&right)), 1)
	if status != 0 {
		code, _, _ := toErr.Call(status)
		return windows.Errno(code)
	}
	return nil
}

func setupSSH(ctx context.Context, opt BootstrapOptions, sid string) (string, error) {
	if opt.SSHPort < 1 || opt.SSHPort > 65535 || opt.SSHSources == "" || opt.PublicKeyFile == "" {
		return "", errors.New("SSH setup requires --ssh-port, --ssh-sources and --public-key")
	}
	for _, s := range strings.Split(opt.SSHSources, ",") {
		if _, err := netip.ParsePrefix(s); err != nil {
			if _, err = netip.ParseAddr(s); err != nil {
				return "", fmt.Errorf("invalid SSH source IP/CIDR %q", s)
			}
		}
	}
	raw, err := os.ReadFile(opt.PublicKeyFile)
	if err != nil {
		return "", err
	}
	key, _, _, rest, err := ssh.ParseAuthorizedKey(raw)
	if err != nil || len(strings.TrimSpace(string(rest))) != 0 {
		return "", errors.New("public key file must contain one valid SSH key")
	}
	sshDir := filepath.Join(os.Getenv("WINDIR"), "System32", "OpenSSH")
	sshd := filepath.Join(sshDir, "sshd.exe")
	if _, err = os.Stat(sshd); os.IsNotExist(err) {
		if err = adminCommand(ctx, "dism.exe", "/Online", "/Add-Capability", "/CapabilityName:OpenSSH.Server~~~~0.0.1.0", "/NoRestart"); err != nil {
			return "", err
		}
	}
	if err = adminCommand(ctx, filepath.Join(sshDir, "ssh-keygen.exe"), "-A"); err != nil {
		return "", err
	}
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return "", errors.New("ProgramData is not set")
	}
	keysDir := filepath.Join(programData, "DayZDeploy", "ssh")
	keyFile := filepath.Join(keysDir, sid+".authorized_keys")
	if err = AtomicWrite(keyFile, ssh.MarshalAuthorizedKey(key), 0600); err != nil {
		return "", err
	}
	if err = adminCommand(ctx, "icacls.exe", keysDir, "/inheritance:r", "/grant:r", "*"+sid+":(OI)(CI)R", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "/T", "/Q"); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(programData, "ssh", "sshd_config")
	old, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		template, readErr := os.ReadFile(filepath.Join(sshDir, "sshd_config_default"))
		if readErr != nil {
			return "", readErr
		}
		old = template
	} else if err != nil {
		return "", err
	}
	account := opt.Account
	if strings.HasPrefix(account, `.\`) {
		account = strings.TrimPrefix(account, `.\`)
	}
	account = strings.ToLower(account)
	if strings.ContainsAny(account, "\r\n\t \"#") {
		return "", errors.New("unsupported SSH account name")
	}
	begin := "# dzdeploy " + sid + " begin"
	end := "# dzdeploy " + sid + " end"
	text := string(old)
	if start := strings.Index(text, begin); start >= 0 {
		finish := strings.Index(text[start:], end)
		if finish < 0 {
			return "", errors.New("incomplete existing dzdeploy SSH block")
		}
		text = text[:start] + text[start+finish+len(end):]
	}
	// Change only global Port directives. Preserve unrelated existing Match blocks.
	lines := strings.Split(text, "\n")
	inMatch := false
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.EqualFold(fields[0], "Match") {
			inMatch = true
		}
		if !inMatch && len(fields) > 0 && strings.EqualFold(fields[0], "Port") {
			lines[i] = "# port managed by dzdeploy"
		}
	}
	text = fmt.Sprintf("Port %d\n", opt.SSHPort) + strings.Join(lines, "\n") + "\n" + begin + "\nMatch User " + account + "\n    AuthenticationMethods publickey\n    PasswordAuthentication no\n    AuthorizedKeysFile " + filepath.ToSlash(keyFile) + "\n    AllowTcpForwarding no\n    PermitTTY no\n    ForceCommand internal-sftp\n" + end + "\n"
	if err = AtomicWrite(cfgPath+".dzdeploy-backup", old, 0600); err != nil {
		return "", err
	}
	if err = AtomicWrite(cfgPath, []byte(text), 0600); err != nil {
		return "", err
	}
	if err = adminCommand(ctx, sshd, "-t", "-f", cfgPath); err != nil {
		_ = AtomicWrite(cfgPath, old, 0600)
		return "", err
	}
	// Disable the standard broad installation rule before installing the scoped rule.
	_ = adminCommand(ctx, "netsh.exe", "advfirewall", "firewall", "set", "rule", "name=OpenSSH SSH Server (sshd)", "new", "enable=no")
	_ = adminCommand(ctx, "netsh.exe", "advfirewall", "firewall", "delete", "rule", "name=dzdeploy SSH")
	if err = adminCommand(ctx, "netsh.exe", "advfirewall", "firewall", "add", "rule", "name=dzdeploy SSH", "dir=in", "action=allow", "protocol=TCP", fmt.Sprintf("localport=%d", opt.SSHPort), "remoteip="+opt.SSHSources, "program="+sshd); err != nil {
		return "", err
	}
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()
	service, err := m.OpenService("sshd")
	if err != nil {
		return "", err
	}
	defer service.Close()
	cfg, err := service.Config()
	if err != nil {
		return "", err
	}
	cfg.StartType = mgr.StartAutomatic
	if err = service.UpdateConfig(cfg); err != nil {
		return "", err
	}
	status, err := service.Query()
	if err != nil {
		return "", err
	}
	if status.State != svc.Stopped {
		_, err = service.Control(svc.Stop)
		if err != nil {
			return "", err
		}
		for i := 0; i < 30; i++ {
			status, err = service.Query()
			if err != nil {
				return "", err
			}
			if status.State == svc.Stopped {
				break
			}
			time.Sleep(time.Second)
		}
	}
	if err = service.Start(); err != nil {
		return "", err
	}
	pub, err := os.ReadFile(filepath.Join(programData, "ssh", "ssh_host_ed25519_key.pub"))
	if err != nil {
		return "", err
	}
	hostKey, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(hostKey), nil
}

func ensureNoGame(install string) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "DayZServer_x64.exe") {
			if _, e := processToken(int(entry.ProcessID), filepath.Join(install, "DayZServer_x64.exe")); e == nil {
				return fmt.Errorf("existing DayZ process %d is running", entry.ProcessID)
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return err
	}
	return nil
}
