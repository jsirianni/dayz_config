// Package deploy implements the dzdeploy client and durable host protocol.
package deploy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Protocol = 1
const MaxPackageBytes = 256 << 20
const MaxFileBytes = 64 << 20

//go:embed targets.json
var defaultTargets []byte

type Mod struct {
	ID     string `json:"id"`
	Folder string `json:"folder"`
}
type Target struct {
	Folder         string `json:"folder"`
	Mission        string `json:"mission"`
	GamePort       int    `json:"gamePort"`
	QueryPort      int    `json:"queryPort"`
	ValidateServer bool   `json:"validateServer"`
	Mods           []Mod  `json:"mods"`
}

func Targets(data []byte) (map[string]Target, error) {
	if data == nil {
		data = defaultTargets
	}
	var targets map[string]Target
	if err := json.Unmarshal(data, &targets); err != nil {
		return nil, err
	}
	for _, alias := range []string{"nasdara", "nasdara-staging", "deer-isle"} {
		t, ok := targets[alias]
		if !ok {
			return nil, fmt.Errorf("missing target %s", alias)
		}
		if err := t.Validate(); err != nil {
			return nil, fmt.Errorf("target %s: %w", alias, err)
		}
	}
	if len(targets) != 3 {
		return nil, errors.New("only the three current servers are supported")
	}
	return targets, nil
}
func TargetFor(alias string) (Target, error) {
	ts, err := Targets(nil)
	if err != nil {
		return Target{}, err
	}
	t, ok := ts[alias]
	if !ok {
		return t, fmt.Errorf("unknown server %q (use nasdara, nasdara-staging, deer-isle)", alias)
	}
	return t, nil
}
func (t Target) Validate() error {
	if err := SafeRelative(t.Folder); err != nil {
		return err
	}
	if strings.Contains(t.Folder, "/") {
		return errors.New("invalid server folder")
	}
	if err := SafeRelative(t.Mission); err != nil {
		return err
	}
	if strings.Contains(t.Mission, "/") {
		return errors.New("invalid mission")
	}
	if t.GamePort < 1 || t.GamePort > 65535 || t.QueryPort < 0 || t.QueryPort > 65535 {
		return errors.New("invalid port")
	}
	seen := map[string]bool{}
	for _, m := range t.Mods {
		if !regexp.MustCompile(`^[0-9]+$`).MatchString(m.ID) || !regexp.MustCompile(`^@[A-Za-z0-9_-]+$`).MatchString(m.Folder) || seen[strings.ToLower(m.Folder)] {
			return errors.New("invalid or duplicate mod")
		}
		seen[strings.ToLower(m.Folder)] = true
	}
	if len(t.Mods) == 0 {
		return errors.New("no mods configured")
	}
	return nil
}

type File struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
}
type Manifest struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Target   string    `json:"target"`
	Commit   string    `json:"commit"`
	Created  time.Time `json:"created"`
	Launcher Target    `json:"launcher"`
	Files    []File    `json:"files"`
}
type Request struct {
	Version       int    `json:"version"`
	ID            string `json:"id"`
	Target        string `json:"target"`
	Action        string `json:"action"`
	PackageSHA256 string `json:"packageSha256,omitempty"`
	Deployment    string `json:"deployment,omitempty"`
	Restart       bool   `json:"restart"`
}
type Result struct {
	Cause           string      `json:"cause,omitempty"`
	Version         int         `json:"version"`
	ID              string      `json:"id"`
	Target          string      `json:"target"`
	Status          string      `json:"status"`
	Phase           string      `json:"phase,omitempty"`
	Commit          string      `json:"commit,omitempty"`
	Changed         []string    `json:"changed,omitempty"`
	Deleted         []string    `json:"deleted,omitempty"`
	RestartRequired bool        `json:"restartRequired"`
	Error           string      `json:"error,omitempty"`
	Recovery        string      `json:"recovery,omitempty"`
	Updated         time.Time   `json:"updated"`
	DZSA            *DZSAReport `json:"dzsa,omitempty"`
}
type State struct {
	Version         int         `json:"version"`
	Target          string      `json:"target"`
	Installed       *Manifest   `json:"installed,omitempty"`
	RunningCommit   string      `json:"runningCommit,omitempty"`
	PID             int         `json:"pid,omitempty"`
	ProcessToken    string      `json:"processToken,omitempty"`
	Phase           string      `json:"phase"`
	LastOperation   string      `json:"lastOperation,omitempty"`
	Error           string      `json:"error,omitempty"`
	Updated         time.Time   `json:"updated"`
	Paused          bool        `json:"paused"`
	RestartRequired bool        `json:"restartRequired"`
	DZSA            *DZSAReport `json:"dzsa,omitempty"`
}
type Endpoint struct {
	Address          string      `json:"address" yaml:"address"`
	User             string      `json:"user" yaml:"user"`
	KeyFile          string      `json:"keyFile,omitempty" yaml:"key_file,omitempty"`
	KeyPassphraseEnv string      `json:"keyPassphraseEnv,omitempty" yaml:"key_passphrase_env,omitempty"`
	KnownHosts       string      `json:"knownHosts" yaml:"known_hosts"`
	RemoteDir        string      `json:"remoteDir" yaml:"remote_dir"`
	PublicKeyFile    string      `json:"publicKeyFile,omitempty" yaml:"public_key_file,omitempty"`
	AgentBinary      string      `json:"agentBinary,omitempty" yaml:"agent_binary,omitempty"`
	SSHSources       []string    `json:"sshSources,omitempty" yaml:"ssh_sources,omitempty"`
	Host             *HostConfig `json:"host,omitempty" yaml:"host,omitempty"`
}
type ClientConfig struct {
	Version int                 `json:"version" yaml:"version"`
	Targets map[string]Endpoint `json:"targets" yaml:"targets"`
}
type HostConfig struct {
	Version            int    `json:"version" yaml:"version,omitempty"`
	Target             string `json:"target" yaml:"target,omitempty"`
	InstallDir         string `json:"installDir" yaml:"install_dir"`
	StateDir           string `json:"stateDir" yaml:"state_dir"`
	SteamCMD           string `json:"steamcmd" yaml:"steamcmd"`
	WorkshopDir        string `json:"workshopDir" yaml:"workshop_dir"`
	SteamUser          string `json:"steamUser" yaml:"steam_user"`
	RConAddress        string `json:"rconAddress" yaml:"rcon_address"`
	RConPasswordFile   string `json:"rconPasswordFile" yaml:"rcon_password_file"`
	QueryAddress       string `json:"queryAddress" yaml:"query_address"`
	PublicQueryAddress string `json:"publicQueryAddress,omitempty" yaml:"public_query_address,omitempty"`
	ShutdownSeconds    int    `json:"shutdownSeconds" yaml:"shutdown_seconds,omitempty"`
	UpdateSeconds      int    `json:"updateSeconds" yaml:"update_seconds,omitempty"`
	StartupSeconds     int    `json:"startupSeconds" yaml:"startup_seconds,omitempty"`
}

func (c *HostConfig) Validate() error {
	if c.Version != Protocol {
		return VersionError(c.Version)
	}
	if _, err := TargetFor(c.Target); err != nil {
		return err
	}
	paths := make([]string, 0, 5)
	for _, p := range []string{c.InstallDir, c.StateDir, c.SteamCMD, c.WorkshopDir, c.RConPasswordFile} {
		clean, err := normalizedHostPath(p)
		if err != nil {
			return err
		}
		paths = append(paths, clean)
	}
	install, state := paths[0], paths[1]
	volumeRoot := func(p string) bool { return path.Dir(p) == p || (len(p) == 2 && p[1] == ':') }
	if volumeRoot(install) || volumeRoot(state) {
		return errors.New("installation and state directories must not be volume roots")
	}
	steamRoot := path.Dir(paths[2])
	if volumeRoot(steamRoot) || volumeRoot(paths[3]) {
		return errors.New("Steam/workshop directories must not be volume roots")
	}
	contains := func(root, p string) bool { return p == root || strings.HasPrefix(p, strings.TrimRight(root, "/")+"/") }
	if contains(install, state) || contains(state, install) {
		return errors.New("state directory and installation must not overlap")
	}
	if c.SteamUser == "" || c.RConAddress == "" || c.QueryAddress == "" {
		return errors.New("steamUser, rconAddress and queryAddress are required")
	}
	if !contains(state, paths[4]) {
		return errors.New("RCon password file must be inside the protected state directory")
	}
	for _, address := range []string{c.RConAddress, c.QueryAddress} {
		host, port, err := net.SplitHostPort(address)
		n, numberErr := strconv.Atoi(port)
		if err != nil || numberErr != nil || n < 1 || n > 65535 || host != "127.0.0.1" {
			return errors.New("host RCon/query addresses must be 127.0.0.1:<valid-port>")
		}
	}
	if c.PublicQueryAddress != "" {
		if _, err := publicQueryAddress(c.PublicQueryAddress); err != nil {
			return err
		}
	}
	if c.ShutdownSeconds == 0 {
		c.ShutdownSeconds = 180
	}
	if c.UpdateSeconds == 0 {
		c.UpdateSeconds = 1800
	}
	if c.StartupSeconds == 0 {
		c.StartupSeconds = 600
	}
	if c.ShutdownSeconds < 1 || c.UpdateSeconds < 1 || c.StartupSeconds < 1 {
		return errors.New("timeouts must be positive")
	}
	return nil
}

func normalizedHostPath(p string) (string, error) {
	if len(p) > 1 && p[1] == ':' {
		win, err := WindowsPath(p)
		if err != nil {
			return "", err
		}
		return strings.ToLower(strings.ReplaceAll(win, "\\", "/")), nil
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("host path must be absolute: %s", p)
	}
	return filepath.ToSlash(filepath.Clean(p)), nil
}
func VersionError(v int) error {
	return fmt.Errorf("protocol version %d is incompatible with %d; upgrade client and host agent together", v, Protocol)
}
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}

var idPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}-[a-f0-9]{16}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }
func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ReadJSON(p string, v any) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if len(b) > MaxFileBytes {
		return errors.New("JSON file too large")
	}
	return json.Unmarshal(b, v)
}
func WriteJSON(p string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(p, append(b, '\n'), 0600)
}
func AtomicWrite(p string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".dzdeploy-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tmp, p)
}

// AtomicCopy streams large Workshop assets instead of buffering multi-GB PBOs.
func AtomicCopy(source, dest string, mode os.FileMode) error {
	return atomicCopyContext(context.Background(), source, dest, mode)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
func atomicCopyContext(ctx context.Context, source, dest string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(dest), ".dzdeploy-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	if err = out.Chmod(mode); err == nil {
		_, err = io.Copy(out, contextReader{ctx, in})
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tmp, dest)
}
func within(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}
