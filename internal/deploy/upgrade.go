package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/hostinspect"
)

const AgentHandoffCapability = "agent-handoff.v1"
const HandoffControl = 200

// ProcessIdentity binds a PID to a particular executable and Windows creation time.
type ProcessIdentity struct {
	PID        int    `json:"pid"`
	Token      string `json:"token"`
	Executable string `json:"executable"`
}

type AgentStatus struct {
	State
	Agent          *AgentIdentity `json:"agent,omitempty"`
	AgentHeartbeat time.Time      `json:"agentHeartbeat,omitempty"`
	AgentFresh     bool           `json:"agentFresh"`
}

type UpgradeRequest struct {
	Version         int        `json:"version"`
	ID              string     `json:"id"`
	Target          string     `json:"target"`
	CandidateSHA256 string     `json:"candidateSHA256"`
	Revision        string     `json:"revision"`
	Host            HostConfig `json:"host"`
}

type UpgradeResult struct {
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Target         string          `json:"target"`
	Status         string          `json:"status"`
	Phase          string          `json:"phase"`
	Error          string          `json:"error,omitempty"`
	Recovery       string          `json:"recovery,omitempty"`
	Updated        time.Time       `json:"updated"`
	Previous       *AgentIdentity  `json:"previous,omitempty"`
	Candidate      *AgentIdentity  `json:"candidate,omitempty"`
	Game           ProcessIdentity `json:"game"`
	Continuity     bool            `json:"continuity"`
	StartupPending bool            `json:"startupPending"`
}

type UpgradeHold struct {
	Version         int             `json:"version"`
	ID              string          `json:"id"`
	Target          string          `json:"target"`
	Game            ProcessIdentity `json:"game"`
	Previous        AgentIdentity   `json:"previous"`
	CandidateSHA256 string          `json:"candidateSHA256"`
	Revision        string          `json:"revision"`
}

type UpgradeAcknowledgement struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Target  string          `json:"target"`
	Agent   AgentIdentity   `json:"agent"`
	Game    ProcessIdentity `json:"game"`
	Updated time.Time       `json:"updated"`
}

type UpgradeDecision struct {
	Version  int             `json:"version"`
	ID       string          `json:"id"`
	Target   string          `json:"target"`
	Decision string          `json:"decision"`
	Agent    ProcessIdentity `json:"agent"`
}

// AdoptionProof is authoritative only when read from the protected coordinator
// directory, never from the service-writable state tree.
type AdoptionProof struct {
	Version  int             `json:"version"`
	ID       string          `json:"id"`
	Target   string          `json:"target"`
	Game     ProcessIdentity `json:"game"`
	Parent   int             `json:"parent"`
	Previous AgentIdentity   `json:"previous"`
	Agent    AgentIdentity   `json:"agent"`
	Updated  time.Time       `json:"updated"`
}

func (r UpgradeRequest) Validate() error {
	if r.Version != Protocol {
		return VersionError(r.Version)
	}
	if !ValidID(r.ID) || !digestPattern.MatchString(r.CandidateSHA256) {
		return errors.New("invalid agent upgrade request identity or checksum")
	}
	if _, err := TargetFor(r.Target); err != nil {
		return err
	}
	if r.Host.Target != r.Target {
		return errors.New("upgrade host target mismatch")
	}
	if err := r.Host.Validate(); err != nil {
		return err
	}
	if len(r.Revision) != 40 || strings.Trim(r.Revision, "0123456789abcdef") != "" {
		return errors.New("upgrade requires a recorded Git revision")
	}
	return nil
}

func (r UpgradeResult) Validate(target, id string) error {
	if r.Version != Protocol {
		return VersionError(r.Version)
	}
	if !ValidID(id) || r.ID != id || r.Target != target {
		return errors.New("agent upgrade result identity mismatch")
	}
	switch r.Status {
	case "running", "succeeded", "failed", "unchanged":
	default:
		return errors.New("invalid agent upgrade result status")
	}
	return nil
}

func upgradeRoot(target string) string {
	return filepath.Join(os.Getenv("ProgramData"), "dzdeploy", "upgrades", target)
}

func validAdoption(proof AdoptionProof, state State, snapshot *hostinspect.Snapshot, target Target, now time.Time) bool {
	if snapshot == nil || len(snapshot.Processes) != 1 || proof.Version != Protocol || proof.Target != state.Target || !ValidID(proof.ID) || proof.Updated.IsZero() || proof.Updated.After(now.Add(15*time.Second)) {
		return false
	}
	configured, err := TargetFor(proof.Target)
	if err != nil || configured.Folder != target.Folder {
		return false
	}
	p := snapshot.Processes[0]
	return proof.Game.PID == state.PID && proof.Game.PID == p.PID && proof.Game.Token != "" && proof.Game.Token == state.ProcessToken && proof.Game.Token == p.Token && strings.EqualFold(proof.Game.Executable, p.Executable) && proof.Parent == p.Parent && proof.Parent > 0 && proof.Previous.PID > 0 && proof.Agent.PID == snapshot.Service.PID && proof.Agent.Token != "" && proof.Agent.Token == snapshot.Service.Token && digestPattern.MatchString(proof.Agent.SHA256) && proof.Agent.SHA256 == snapshot.Service.SHA256
}

// upgradeIdle is called again after the legacy manager has been frozen. Merely
// observing an old heartbeat is insufficient to exclude in-flight work.
func upgradeIdle(c HostConfig) error {
	for _, name := range []string{"journal.json", "restore.json"} {
		if _, err := os.Stat(filepath.Join(c.StateDir, name)); err == nil {
			return fmt.Errorf("agent upgrade blocked by unresolved %s", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	entries, err := os.ReadDir(filepath.Join(c.StateDir, "inbox"))
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("agent upgrade requires an empty inbox, including pending uploads")
	}
	entries, err = os.ReadDir(filepath.Join(c.StateDir, "results"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var result Result
		if err := ReadJSON(filepath.Join(c.StateDir, "results", e.Name()), &result); err != nil {
			return err
		}
		if result.Status == "running" {
			return errors.New("agent operation is still running")
		}
	}
	var state State
	if err := ReadJSON(filepath.Join(c.StateDir, "state.json"), &state); err != nil {
		return err
	}
	if state.Version != Protocol || state.Target != c.Target {
		return errors.New("upgrade state identity mismatch")
	}
	switch state.Phase {
	case "updating", "applying", "starting", "checking", "provisioning", "stopping", "backup", "restoring":
		return fmt.Errorf("agent is busy: %s", state.Phase)
	}
	return nil
}
