// Package operation persists recoverable intent before sending remote writes.
package operation

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
)

type Result struct {
	APIVersion        string            `json:"apiVersion"`
	Status            string            `json:"status"`
	OperationID       string            `json:"operation_id,omitempty"`
	Context           string            `json:"context,omitempty"`
	VMID              int               `json:"vmid,omitempty"`
	Node              string            `json:"node,omitempty"`
	Kind              string            `json:"kind,omitempty"`
	Phase             string            `json:"phase,omitempty"`
	InputDigest       string            `json:"input_digest,omitempty"`
	PublicKey         string            `json:"public_key,omitempty"`
	ContextIdentity   string            `json:"context_identity,omitempty"`
	Tasks             []string          `json:"task_ids,omitempty"`
	ActiveTask        string            `json:"active_task,omitempty"`
	ActiveTaskPhase   string            `json:"active_task_phase,omitempty"`
	SubmissionPending bool              `json:"submission_pending,omitempty"`
	Completed         []string          `json:"completed_phases,omitempty"`
	Changes           map[string]string `json:"changes,omitempty"`
	Pending           any               `json:"pending_changes,omitempty"`
	SSHEndpoint       string            `json:"ssh_endpoint,omitempty"`
	HostKey           string            `json:"host_key_fingerprint,omitempty"`
	Error             string            `json:"error,omitempty"`
	Data              any               `json:"data,omitempty"`
	Deploy            *spec.Deploy      `json:"deployment,omitempty"`
	Updated           *time.Time        `json:"updated,omitempty"`
	Residual          []string          `json:"residual_artifacts,omitempty"`
}
type Store struct{ Dir string }

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func NewID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func (s Store) Path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("invalid operation ID")
	}
	return filepath.Join(s.Dir, "operations", id+".json"), nil
}
func (s Store) Save(r *Result) error {
	if e := SecureDir(s.Dir); e != nil {
		return e
	}
	p, e := s.Path(r.OperationID)
	if e != nil {
		return e
	}
	if e = SecureDir(filepath.Dir(p)); e != nil {
		return e
	}
	now := time.Now().UTC()
	r.Updated = &now
	r.APIVersion = spec.Version
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".operation-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func (s Store) Load(id string) (*Result, error) {
	p, e := s.Path(id)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	var r Result
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	if r.OperationID != id || r.APIVersion != spec.Version {
		return nil, errors.New("invalid operation journal identity/version")
	}
	return &r, nil
}
func (s Store) Lock(context string, vmid int) (func(), error) {
	if !spec.Identifier(context) || !spec.VMID(vmid) {
		return nil, errors.New("invalid lock identity")
	}
	if e := SecureDir(s.Dir); e != nil {
		return nil, e
	}
	dir := filepath.Join(s.Dir, "locks")
	if e := SecureDir(dir); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%s-%d.lock", context, vmid)), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = lockFile(f); e != nil {
		f.Close()
		return nil, fmt.Errorf("another local operation holds VM %d: %w", vmid, e)
	}
	return func() { unlockFile(f); f.Close() }, nil
}
func Has(r *Result, phase string) bool {
	for _, v := range r.Completed {
		if v == phase {
			return true
		}
	}
	return false
}
func Complete(r *Result, phase string) {
	if !Has(r, phase) {
		r.Completed = append(r.Completed, phase)
	}
}
