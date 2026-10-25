//go:build windows

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func RunEnrollment(ctx context.Context, requestFile string, output io.Writer) error {
	var req EnrollmentRequest
	if err := ReadJSON(requestFile, &req); err != nil {
		return err
	}
	c := req.Config
	if req.Version != Protocol {
		return VersionError(req.Version)
	}
	if !ValidID(req.ID) || !digestPattern.MatchString(req.BinarySHA256) {
		return errors.New("invalid enrollment identity/hash")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := req.Launcher.Validate(); err != nil {
		return err
	}
	canonical, _ := TargetFor(c.Target)
	if req.Launcher.Folder != canonical.Folder || req.Launcher.Mission != canonical.Mission {
		return errors.New("enrollment launcher identity mismatch")
	}
	_, query, _ := net.SplitHostPort(c.QueryAddress)
	port, _ := strconv.Atoi(query)
	if req.Launcher.QueryPort != port {
		return errors.New("query_address must match the tracked target query port")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("automatic enrollment requires an administrator SSH account")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	if Digest(binary) != req.BinarySHA256 {
		return errors.New("uploaded agent SHA-256 mismatch")
	}
	for _, p := range []string{c.StateDir, c.InstallDir, c.SteamCMD, c.WorkshopDir, c.RConPasswordFile} {
		if _, err := WindowsPath(p); err != nil {
			return err
		}
		if _, err := SafeJoin(filepath.Dir(p), filepath.Base(p)); err != nil {
			return err
		}
	}
	if err = os.MkdirAll(c.StateDir, 0700); err != nil {
		return err
	}
	if err = adminCommand(ctx, "icacls.exe", c.StateDir, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "/Q"); err != nil {
		return err
	}
	lockDir := filepath.Join(c.StateDir, "setup", "enroll-lock")
	if err = os.MkdirAll(lockDir, 0700); err != nil {
		return err
	}
	unlock, err := lockHost(lockDir)
	if err != nil {
		return errors.New("another enrollment is in progress; rerun deploy after it finishes")
	}
	defer unlock()
	var previous HostConfig
	if err = ReadJSON(filepath.Join(c.StateDir, "host.json"), &previous); err == nil {
		if err = previous.Validate(); err != nil {
			return err
		}
		if !reflect.DeepEqual(previous, c) {
			return errors.New("existing host configuration differs; refusing to reset the installation")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var journal Enrollment
	if err = ReadJSON(enrollmentPath(c), &journal); err == nil {
		if !validEnrollment(journal, c.Target) {
			return errors.New("invalid existing enrollment journal")
		}
	} else if os.IsNotExist(err) {
		if _, err = os.Stat(filepath.Join(c.StateDir, "state.json")); err == nil {
			return errors.New("existing deployment state has no enrollment journal; refusing to reset it")
		} else if !os.IsNotExist(err) {
			return err
		}
		journal = Enrollment{Version: Protocol, Target: c.Target, ID: req.ID, Phase: "installing", Updated: time.Now().UTC()}
		if err = WriteJSON(enrollmentPath(c), journal); err != nil {
			return err
		}
	} else {
		return err
	}
	if journal.Phase == "ready" {
		return errors.New("host is already provisioned; use its managed service")
	}
	fmt.Fprintf(output, "Enrollment %s for %s: %s\n", journal.ID, c.Target, journal.Phase)
	running, err := enrollmentServiceRunning(c, previous.Target != "")
	if err != nil {
		return err
	}
	if running {
		if err = setupGameFirewall(ctx, c, req.Launcher); err != nil {
			return err
		}
		journal.Phase = "service-ready"
		journal.Updated = time.Now().UTC()
		return WriteJSON(enrollmentPath(c), journal)
	}
	if err = ensureNoGame(c.InstallDir); err != nil {
		return err
	}
	for _, dir := range []string{filepath.Dir(c.SteamCMD), c.WorkshopDir, c.InstallDir, filepath.Join(c.StateDir, "logs")} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	fmt.Fprintln(output, "Installing verified SteamCMD and Microsoft runtime prerequisites...")
	if err = installPrerequisites(ctx, c); err != nil {
		return err
	}
	fmt.Fprintln(output, "Installing the per-instance Windows service...")
	if _, err = Bootstrap(ctx, BootstrapOptions{Config: c, Virtual: true}); err != nil {
		return err
	}
	if err = setupGameFirewall(ctx, c, req.Launcher); err != nil {
		return err
	}
	journal.Phase = "service-ready"
	journal.Updated = time.Now().UTC()
	if err = WriteJSON(enrollmentPath(c), journal); err != nil {
		return err
	}
	fmt.Fprintln(output, "Service enrolled; Steam authentication and installation continue through dzdeploy.")
	return nil
}

func enrollmentServiceRunning(c HostConfig, bound bool) (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName(c.Target))
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return false, nil
		}
		return false, err
	}
	defer s.Close()
	if !bound {
		return false, errors.New("service exists without matching host configuration")
	}
	cfg, err := s.Config()
	if err != nil {
		return false, err
	}
	args, err := windows.DecomposeCommandLine(cfg.BinaryPathName)
	if err != nil {
		return false, err
	}
	if len(args) != 4 || !strings.EqualFold(args[0], filepath.Join(c.StateDir, "bin", "dzdeploy.exe")) || args[1] != "agent" || args[2] != "--host-config" || !strings.EqualFold(args[3], filepath.Join(c.StateDir, "host.json")) || !strings.EqualFold(cfg.ServiceStartName, `NT SERVICE\`+ServiceName(c.Target)) {
		return false, errors.New("existing service identity or executable differs; refusing to overwrite it")
	}
	status, err := s.Query()
	return status.State == svc.Running, err
}

func installPrerequisites(ctx context.Context, c HostConfig) error {
	stage := filepath.Join(c.StateDir, "setup", "prerequisites")
	if err := os.MkdirAll(stage, 0700); err != nil {
		return err
	}
	if _, err := os.Stat(c.SteamCMD); os.IsNotExist(err) {
		raw, err := downloadOfficial(ctx, steamInstallerURL, 32<<20)
		if err != nil {
			return err
		}
		exe, err := steamExecutable(raw)
		if err != nil {
			return err
		}
		candidate := filepath.Join(stage, "steamcmd.exe")
		if err = AtomicWrite(candidate, exe, 0700); err != nil {
			return err
		}
		if err = verifyPublisher(candidate, "Valve", "Valve Corp.", "Valve Corporation"); err != nil {
			return err
		}
		if err = AtomicWrite(c.SteamCMD, exe, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err = verifyPublisher(c.SteamCMD, "Valve", "Valve Corp.", "Valve Corporation"); err != nil {
		return err
	}
	var completed struct {
		Version int `json:"version"`
	}
	marker := filepath.Join(stage, "runtimes.json")
	if err := ReadJSON(marker, &completed); err == nil && completed.Version == Protocol {
		return nil
	}
	for _, arch := range []string{"x64", "x86"} {
		url := "https://aka.ms/vs/17/release/vc_redist." + arch + ".exe"
		raw, err := downloadOfficial(ctx, url, 128<<20)
		if err != nil {
			return err
		}
		dest := filepath.Join(stage, "vc_redist."+arch+".exe")
		if err = AtomicWrite(dest, raw, 0700); err != nil {
			return err
		}
		if err = verifyPublisher(dest, "Microsoft Corporation"); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, dest, "/install", "/quiet", "/norestart")
		prepareCommand(cmd)
		if err = cmd.Run(); err != nil {
			var exit *exec.ExitError
			// The runtime is usable after a 3010 result; record the reboot request
			// but never reboot an operator's host as a side effect of deployment.
			if !errors.As(err, &exit) || (exit.ExitCode() != 3010 && exit.ExitCode() != 1638) {
				return fmt.Errorf("Microsoft runtime installation failed: %w", err)
			}
			if exit.ExitCode() == 3010 {
				if err = AtomicWrite(filepath.Join(c.StateDir, "logs", "runtime-reboot-requested.txt"), []byte("Microsoft runtime installer requested a host reboot.\n"), 0600); err != nil {
					return err
				}
			}
		}
	}
	return WriteJSON(marker, struct {
		Version int `json:"version"`
	}{Protocol})
}

func setupGameFirewall(ctx context.Context, c HostConfig, t Target) error {
	if t.GamePort > 65532 || t.QueryPort < 1 {
		return errors.New("explicit valid game/query ports are required for provisioning")
	}
	name := "dzdeploy game " + c.Target
	_ = adminCommand(ctx, "netsh.exe", "advfirewall", "firewall", "delete", "rule", "name="+name)
	ports := fmt.Sprintf("%d-%d,%d", t.GamePort, t.GamePort+3, t.QueryPort)
	return adminCommand(ctx, "netsh.exe", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=allow", "protocol=UDP", "localport="+ports, "program="+filepath.Join(c.InstallDir, "DayZServer_x64.exe"))
}

// Only this helper bridges SSH to the private service pipe. No secret appears
// in a command argument, file, environment variable, or persistent operation.
func RelaySteamAuth(ctx context.Context, c HostConfig, in io.Reader, out io.Writer) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("Steam enrollment relay requires an administrator SSH account")
	}
	return relayAuthPipe(ctx, c, in, out)
}
