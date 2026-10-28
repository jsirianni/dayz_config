package deploy

import (
	"bytes"
	"debug/buildinfo"
	"debug/pe"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

// AgentIdentity separates executable provenance from the installed game release.
type AgentIdentity struct {
	Protocol int    `json:"protocol"`
	Revision string `json:"revision,omitempty"`
	Dirty    bool   `json:"dirty"`
	SHA256   string `json:"sha256,omitempty"`
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
	PID      int    `json:"pid,omitempty"`
	Token    string `json:"token,omitempty"`
}

const agentModule = "github.com/jsirianni/dayz_config"

func identityFromBuildInfo(info *debug.BuildInfo) (AgentIdentity, error) {
	id := AgentIdentity{Protocol: Protocol, Dirty: true}
	if info == nil || info.Main.Path != agentModule || info.Path != agentModule+"/cmd/dzdeploy" {
		return id, errors.New("agent executable does not identify the dzdeploy command module")
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			id.Revision = setting.Value
		case "vcs.modified":
			id.Dirty = setting.Value != "false"
		case "GOOS":
			id.GOOS = setting.Value
		case "GOARCH":
			id.GOARCH = setting.Value
		}
	}
	return id, nil
}

// ReadAgentIdentity inspects Go's embedded provenance without executing the file.
func ReadAgentIdentity(file string) (AgentIdentity, error) {
	info, err := os.Stat(file)
	if err != nil {
		return AgentIdentity{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxFileBytes {
		return AgentIdentity{}, errors.New("invalid agent executable")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return AgentIdentity{}, err
	}
	if len(raw) > MaxFileBytes {
		return AgentIdentity{}, errors.New("agent executable exceeds size limit")
	}
	build, err := buildinfo.Read(bytes.NewReader(raw))
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("agent build identity unavailable: %w", err)
	}
	id, err := identityFromBuildInfo(build)
	if err != nil {
		return id, err
	}
	id.SHA256 = Digest(raw)
	return id, nil
}

func CurrentAgentIdentity() AgentIdentity {
	id := AgentIdentity{Protocol: Protocol, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, PID: os.Getpid(), Dirty: true}
	if info, ok := debug.ReadBuildInfo(); ok {
		if recorded, err := identityFromBuildInfo(info); err == nil {
			id = recorded
			id.PID = os.Getpid()
		}
	}
	if exe, err := os.Executable(); err == nil {
		if raw, readErr := os.ReadFile(exe); readErr == nil && len(raw) <= MaxFileBytes {
			id.SHA256 = Digest(raw)
		}
		id.Token, _ = processToken(id.PID, exe)
	}
	return id
}

func ValidateUpgradeCandidate(id AgentIdentity, revision string) error {
	if len(revision) != 40 || strings.Trim(revision, "0123456789abcdef") != "" {
		return errors.New("upgrade requires an exact Git revision")
	}
	if id.Protocol != Protocol || id.GOOS != "windows" || id.GOARCH != "amd64" {
		return errors.New("upgrade requires a compatible Windows amd64 agent")
	}
	if id.Dirty || id.Revision != revision || !digestPattern.MatchString(id.SHA256) {
		return errors.New("agent binary must record a clean build of freshly fetched origin/main; rebuild or use its matching package")
	}
	return nil
}

func LoadUpgradeCandidate(endpoint Endpoint) ([]byte, AgentIdentity, error) {
	raw, err := agentBinary(endpoint)
	if err != nil {
		return nil, AgentIdentity{}, err
	}
	// Inspect the exact bytes being uploaded, without executing or rewriting them.
	image, err := pe.NewFile(bytes.NewReader(raw))
	if err != nil {
		return nil, AgentIdentity{}, fmt.Errorf("invalid Windows agent image: %w", err)
	}
	defer image.Close()
	if image.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return nil, AgentIdentity{}, errors.New("upgrade requires a Windows amd64 image")
	}
	info, err := buildinfo.Read(bytes.NewReader(raw))
	if err != nil {
		return nil, AgentIdentity{}, err
	}
	id, err := identityFromBuildInfo(info)
	id.SHA256 = Digest(raw)
	return raw, id, err
}
