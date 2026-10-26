package deploy

import (
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type EnrollmentRequest struct {
	Version      int        `json:"version"`
	ID           string     `json:"id"`
	Config       HostConfig `json:"config"`
	Launcher     Target     `json:"launcher"`
	BinarySHA256 string     `json:"binarySHA256"`
}

type Enrollment struct {
	Version int       `json:"version"`
	Target  string    `json:"target"`
	ID      string    `json:"id"`
	Phase   string    `json:"phase"`
	Updated time.Time `json:"updated"`
}

func validEnrollment(e Enrollment, target string) bool {
	return e.Version == Protocol && e.Target == target && ValidID(e.ID) && (e.Phase == "installing" || e.Phase == "service-ready" || e.Phase == "ready")
}

func hostVerifier(file, pin string, persist bool) (ssh.HostKeyCallback, error) {
	return hostVerifierPolicy(file, pin, persist, false)
}

func hostVerifierPolicy(file, pin string, persist, firstUse bool) (ssh.HostKeyCallback, error) {
	if pin != "" {
		b, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(pin, "SHA256:"))
		if !strings.HasPrefix(pin, "SHA256:") || err != nil || len(b) != sha256.Size {
			return nil, errors.New("--host-key must be an OpenSSH SHA256 fingerprint")
		}
	}
	var verify ssh.HostKeyCallback
	if _, err := os.Stat(file); err == nil {
		verify, err = knownhosts.New(file)
		if err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return func(host string, addr net.Addr, key ssh.PublicKey) error {
		if verify != nil {
			err := verify(host, addr, key)
			if err == nil {
				if pin != "" && pin != ssh.FingerprintSHA256(key) {
					return errors.New("--host-key differs from the existing trusted host key")
				}
				return nil
			}
			var ke *knownhosts.KeyError
			if !errors.As(err, &ke) || len(ke.Want) != 0 {
				return err
			}
		}
		if pin == "" && !firstUse {
			return &UntrustedHostError{Host: host}
		}
		if pin != "" && pin != ssh.FingerprintSHA256(key) {
			return errors.New("SSH host fingerprint does not match trusted bootstrap output")
		}
		if !persist {
			return nil
		}
		return rememberHost(file, host, addr, key)
	}, nil
}

type UntrustedHostError struct{ Host string }

func (e *UntrustedHostError) Error() string {
	return "unknown SSH host " + e.Host + "; supply --host-key using the trusted bootstrap fingerprint"
}

func rememberHost(file, host string, addr net.Addr, key ssh.PublicKey) error {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	lock := file + ".enroll-lock"
	if err := os.MkdirAll(lock, 0700); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	var unlock func()
	for {
		var err error
		unlock, err = lockHost(lock)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("lock known_hosts: %w", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	defer unlock()
	old, err := os.ReadFile(file)
	if err == nil {
		check, e := knownhosts.New(file)
		if e != nil {
			return e
		}
		e = check(host, addr, key)
		if e == nil {
			return nil
		}
		var ke *knownhosts.KeyError
		if !errors.As(e, &ke) || len(ke.Want) != 0 {
			return e
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(old) > 0 && old[len(old)-1] != '\n' {
		old = append(old, '\n')
	}
	return AtomicWrite(file, append(old, []byte(knownhosts.Line([]string{host}, key)+"\n")...), 0600)
}

// WindowsPath accepts drive-qualified paths, independently of the client's OS.
// Enrollment never interpolates YAML paths containing cmd.exe metacharacters.
func WindowsPath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	if len(p) < 4 || p[1:3] != ":/" || !((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) || strings.ContainsAny(p[3:], ":\"%!?&|<>^\r\n\x00") {
		return "", errors.New("enrollment requires a drive-qualified Windows path without shell metacharacters")
	}
	if err := SafeRelative(p[3:]); err != nil {
		return "", err
	}
	return strings.ReplaceAll(p, "/", "\\"), nil
}

func validateEnrollment(e Endpoint, target string) error {
	if e.Host == nil || e.Host.Target != target {
		return errors.New("first deployment requires this target's host settings in deploy.yaml")
	}
	if err := e.Host.Validate(); err != nil {
		return err
	}
	for _, p := range []string{e.Host.StateDir, e.Host.InstallDir, e.Host.SteamCMD, e.Host.WorkshopDir, e.Host.RConPasswordFile} {
		if _, err := WindowsPath(p); err != nil {
			return err
		}
	}
	want := "/" + strings.ReplaceAll(e.Host.StateDir, "\\", "/")
	if !strings.EqualFold(strings.TrimRight(e.RemoteDir, "/"), want) {
		return errors.New("remote_dir must point to host.state_dir for automatic enrollment")
	}
	return nil
}
func CheckEnrollment(e Endpoint, target string) error { return validateEnrollment(e, target) }

// EnrollmentNeeded refuses to interpret missing/corrupt state as permission to
// reset an installation. Only a genuinely empty host or our journal is resumable.
func (r *Remote) EnrollmentNeeded(target string) (bool, error) {
	var s State
	err := r.ReadJSON("state.json", &s)
	if err == nil {
		if s.Version != Protocol {
			return false, VersionError(s.Version)
		}
		if s.Target != target {
			return false, errors.New("state target mismatch")
		}
		var c HostConfig
		if err = r.ReadJSON("host.json", &c); err != nil {
			return false, fmt.Errorf("damaged enrollment: host.json: %w", err)
		}
		if c.Version != Protocol || c.Target != target {
			return false, errors.New("host config identity/version mismatch")
		}
		var enrollment Enrollment
		if err = r.ReadJSON("enrollment.json", &enrollment); err == nil {
			if !validEnrollment(enrollment, target) {
				return false, errors.New("enrollment identity/version mismatch")
			}
			return enrollment.Phase != "ready", nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("cannot read existing host state: %w", err)
	}
	var enrollment Enrollment
	if err = r.ReadJSON("enrollment.json", &enrollment); err == nil {
		if !validEnrollment(enrollment, target) {
			return false, errors.New("invalid enrollment journal")
		}
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	entries, err := r.SFTP.ReadDir(r.Endpoint.RemoteDir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Name() != "setup" && e.Name() != "enroll.lock" {
			return false, errors.New("host directory contains existing data without valid state; refusing to initialize it")
		}
	}
	return true, nil
}

func agentBinary(e Endpoint) ([]byte, error) {
	p := e.AgentBinary
	if p == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
			p = self
		} else {
			p = filepath.Join(filepath.Dir(self), "agents", "windows-amd64", "dzdeploy.exe")
		}
	}
	fi, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("Windows agent companion missing; use a packaged client or set agent_binary in deploy.yaml: %w", err)
	}
	if fi.Size() > MaxFileBytes || !fi.Mode().IsRegular() {
		return nil, errors.New("invalid Windows agent binary")
	}
	f, err := pe.Open(p)
	if err != nil {
		return nil, fmt.Errorf("agent must be a Windows executable: %w", err)
	}
	defer f.Close()
	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return nil, errors.New("DayZ provisioning requires the Windows amd64 agent")
	}
	return os.ReadFile(p)
}

func (r *Remote) Enroll(ctx context.Context, b *Bundle, progress io.Writer) error {
	if err := validateEnrollment(r.Endpoint, b.Manifest.Target); err != nil {
		return err
	}
	binary, err := agentBinary(r.Endpoint)
	if err != nil {
		return err
	}
	dir := "setup/" + NewID()
	if err = r.put(dir+"/dzdeploy.exe", binary, true); err != nil {
		return err
	}
	req := EnrollmentRequest{Version: Protocol, ID: b.Manifest.ID, Config: *r.Endpoint.Host, Launcher: b.Manifest.Launcher, BinarySHA256: Digest(binary)}
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err = r.put(dir+"/request.json", raw, true); err != nil {
		return err
	}
	root, _ := WindowsPath(r.Endpoint.Host.StateDir)
	exe := root + `\` + strings.ReplaceAll(dir, "/", `\`) + `\dzdeploy.exe`
	request := root + `\` + strings.ReplaceAll(dir, "/", `\`) + `\request.json`
	if err = r.exec(ctx, windowsRemoteCommand(exe, "enroll", "--request", request), nil, progress, progress); err != nil {
		return fmt.Errorf("host enrollment interrupted/failed; rerun deploy to resume: %w", err)
	}
	return nil
}

func (r *Remote) exec(ctx context.Context, command string, in io.Reader, out, stderr io.Writer) error {
	s, err := r.SSH.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	s.Stdin = in
	s.Stdout = out
	s.Stderr = stderr
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	return s.Run(command)
}

func (r *Remote) WaitReady(ctx context.Context, target string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := r.Ready(target); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for enrolled service: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (r *Remote) Authenticate(ctx context.Context, prompt func(AuthFrame) (string, error), progress io.Writer) error {
	var enrollment Enrollment
	if err := r.ReadJSON("enrollment.json", &enrollment); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if enrollment.Version != Protocol || enrollment.Target == "" {
		return errors.New("invalid enrollment receipt")
	}
	var c HostConfig
	if err := r.ReadJSON("host.json", &c); err != nil {
		return err
	}
	root, err := WindowsPath(c.StateDir)
	if err != nil {
		return err
	}
	s, err := r.SSH.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	in, err := s.StdinPipe()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	s.Stderr = progress
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	if err = s.Start(windowsRemoteCommand(root+`\bin\dzdeploy.exe`, "steam-auth", "--host-config", root+`\host.json`)); err != nil {
		return err
	}
	if err = authConversation(out, in, prompt); err != nil {
		return err
	}
	return s.Wait()
}

func windowsRemoteCommand(exe, action, flag, file string) string {
	return `cmd.exe /d /s /c ""` + exe + `" ` + action + ` ` + flag + ` "` + file + `""`
}

func newHostPreview(b *Bundle, restart bool) Preview {
	p := Preview{Target: b.Manifest.Target, Commit: b.Manifest.Commit, Restart: restart, Provision: true}
	for _, f := range b.Manifest.Files {
		p.Add = append(p.Add, f.Destination)
	}
	return p
}
func NewHostPreview(b *Bundle, restart bool) Preview { return newHostPreview(b, restart) }

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// These names are shared by the host installer and durable agent.
func enrollmentPath(c HostConfig) string { return filepath.Join(c.StateDir, "enrollment.json") }
func provisioningNeeded(c HostConfig) (bool, error) {
	var e Enrollment
	if err := ReadJSON(enrollmentPath(c), &e); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if !validEnrollment(e, c.Target) {
		return false, errors.New("invalid enrollment journal")
	}
	return e.Phase != "ready", nil
}
func finishProvisioning(c HostConfig) error {
	var e Enrollment
	if err := ReadJSON(enrollmentPath(c), &e); err != nil {
		return err
	}
	e.Phase = "ready"
	e.Updated = time.Now().UTC()
	return WriteJSON(enrollmentPath(c), e)
}

// Avoid using filepath.Join for Windows remote paths on a Unix client.
func remoteStateDir(c HostConfig) string {
	return path.Clean("/" + strings.ReplaceAll(c.StateDir, "\\", "/"))
}
