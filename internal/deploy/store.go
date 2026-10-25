package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Preimage struct {
	Destination string `json:"destination"`
	Existed     bool   `json:"existed"`
	Mode        uint32 `json:"mode"`
	SHA256      string `json:"sha256,omitempty"`
}
type Journal struct {
	Version   int        `json:"version"`
	Operation string     `json:"operation"`
	Before    *Manifest  `json:"before,omitempty"`
	Changes   []Preimage `json:"changes"`
	Committed bool       `json:"committed"`
}
type Store struct {
	Config      HostConfig
	State       State
	BeforeWrite func(string) error
}

func OpenStore(c HostConfig) (*Store, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.InstallDir, 0700); err != nil {
		return nil, err
	}
	for _, dir := range []string{"inbox", "results", "releases", "backups", "logs"} {
		p, err := SafeJoin(c.StateDir, dir)
		if err != nil {
			return nil, err
		}
		if err = os.MkdirAll(p, 0700); err != nil {
			return nil, err
		}
	}
	s := &Store{Config: c, State: State{Version: Protocol, Target: c.Target, Phase: "idle", Paused: true}}
	err := ReadJSON(filepath.Join(c.StateDir, "state.json"), &s.State)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if s.State.Version != Protocol {
		return nil, VersionError(s.State.Version)
	}
	if s.State.Target != c.Target {
		return nil, errors.New("state belongs to another server")
	}
	if s.State.Installed != nil {
		if err = checkOwnership(*s.State.Installed, c.Target); err != nil {
			return nil, err
		}
	}
	var bound HostConfig
	if err := ReadJSON(filepath.Join(c.StateDir, "host.json"), &bound); err == nil && !strings.EqualFold(filepath.Clean(bound.InstallDir), filepath.Clean(c.InstallDir)) {
		return nil, errors.New("state directory is bound to a different installation; use a new state directory")
	}
	return s, nil
}
func (s *Store) Save() error {
	s.State.Updated = time.Now().UTC()
	return WriteJSON(filepath.Join(s.Config.StateDir, "state.json"), s.State)
}
func checkOwnership(m Manifest, alias string) error {
	if m.Version != Protocol {
		return VersionError(m.Version)
	}
	canonical, err := TargetFor(alias)
	if err != nil {
		return err
	}
	if m.Target != alias || !ValidID(m.ID) || m.Launcher.Folder != canonical.Folder || m.Launcher.Mission != canonical.Mission {
		return errors.New("invalid ownership manifest")
	}
	if err = m.Launcher.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		dest, ok := Destination(m.Launcher, f.Source)
		if !ok || dest != f.Destination || seen[strings.ToLower(dest)] {
			return errors.New("unsafe or colliding ownership record")
		}
		seen[strings.ToLower(dest)] = true
	}
	return nil
}
func (s *Store) Diff(b *Bundle) ([]string, []string, error) {
	if err := b.Verify(s.Config.Target); err != nil {
		return nil, nil, err
	}
	var changed, deleted []string
	next := map[string]bool{}
	for _, f := range b.Manifest.Files {
		next[f.Destination] = true
		p, err := SafeJoin(s.Config.InstallDir, f.Destination)
		if err != nil {
			return nil, nil, err
		}
		if info, statErr := os.Stat(p); statErr == nil && (!info.Mode().IsRegular() || info.Size() > MaxFileBytes) {
			return nil, nil, fmt.Errorf("managed live file is non-regular or too large: %s", f.Destination)
		}
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			changed = append(changed, f.Destination)
		} else if err != nil {
			return nil, nil, err
		} else if Digest(data) != f.SHA256 {
			changed = append(changed, f.Destination)
		}
	}
	if s.State.Installed != nil {
		if err := checkOwnership(*s.State.Installed, s.Config.Target); err != nil {
			return nil, nil, err
		}
		for _, f := range s.State.Installed.Files {
			if !next[f.Destination] {
				deleted = append(deleted, f.Destination)
			}
		}
	}
	sort.Strings(changed)
	sort.Strings(deleted)
	return changed, deleted, nil
}

// Apply journals all preimages before the first live-file mutation. A successful
// state update and committed journal are persisted before the journal is retired.
func (s *Store) Apply(b *Bundle, operation string) (changed, deleted []string, err error) {
	if !ValidID(operation) {
		return nil, nil, errors.New("invalid operation ID")
	}
	changed, deleted, err = s.Diff(b)
	if err != nil {
		return nil, nil, err
	}
	journal := Journal{Version: Protocol, Operation: operation, Before: s.State.Installed}
	for _, dest := range append(append([]string{}, changed...), deleted...) {
		p, e := SafeJoin(s.Config.InstallDir, dest)
		if e != nil {
			return nil, nil, e
		}
		fi, e := os.Stat(p)
		pre := Preimage{Destination: dest}
		if e == nil {
			if !fi.Mode().IsRegular() || fi.Size() > MaxFileBytes {
				return nil, nil, fmt.Errorf("invalid live file: %s", dest)
			}
			pre.Existed = true
			pre.Mode = uint32(fi.Mode().Perm())
			data, e := os.ReadFile(p)
			if e != nil {
				return nil, nil, e
			}
			pre.SHA256 = Digest(data)
			backup, e := SafeJoin(s.Config.StateDir, "backups/"+operation+"/"+dest)
			if e != nil {
				return nil, nil, e
			}
			if e = AtomicWrite(backup, data, 0600); e != nil {
				return nil, nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, nil, e
		}
		journal.Changes = append(journal.Changes, pre)
	}
	if err = WriteJSON(filepath.Join(s.Config.StateDir, "journal.json"), journal); err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil && !journal.Committed {
			if recovery := s.restore(journal); recovery != nil {
				err = fmt.Errorf("%w; recovery failed: %v; instance remains blocked", err, recovery)
			} else {
				_ = os.Remove(filepath.Join(s.Config.StateDir, "journal.json"))
			}
		}
	}()
	byDest := map[string]File{}
	for _, f := range b.Manifest.Files {
		byDest[f.Destination] = f
	}
	for _, dest := range changed {
		p, e := SafeJoin(s.Config.InstallDir, dest)
		if e != nil {
			err = e
			return
		}
		if s.BeforeWrite != nil {
			if err = s.BeforeWrite(dest); err != nil {
				return
			}
		}
		f := byDest[dest]
		if err = AtomicWrite(p, b.Content[f.Source], 0600); err != nil {
			return
		}
	}
	for _, dest := range deleted {
		p, e := SafeJoin(s.Config.InstallDir, dest)
		if e != nil {
			err = e
			return
		}
		if s.BeforeWrite != nil {
			if err = s.BeforeWrite(dest); err != nil {
				return
			}
		}
		if e = os.Remove(p); e != nil && !os.IsNotExist(e) {
			err = e
			return
		}
	}
	for _, f := range b.Manifest.Files {
		p, e := SafeJoin(s.Config.InstallDir, f.Destination)
		if e != nil {
			err = e
			return
		}
		data, e := os.ReadFile(p)
		if e != nil {
			err = e
			return
		}
		if Digest(data) != f.SHA256 {
			err = fmt.Errorf("installed hash mismatch: %s", f.Destination)
			return
		}
	}
	installed := b.Manifest
	s.State.Installed = &installed
	if err = s.Save(); err != nil {
		return
	}
	journal.Committed = true
	if err = WriteJSON(filepath.Join(s.Config.StateDir, "journal.json"), journal); err != nil {
		return
	}
	err = os.Remove(filepath.Join(s.Config.StateDir, "journal.json"))
	return
}
func (s *Store) restore(j Journal) error {
	if j.Version != Protocol {
		return VersionError(j.Version)
	}
	if !ValidID(j.Operation) {
		return errors.New("invalid journal ID")
	}
	if j.Before != nil {
		if err := checkOwnership(*j.Before, s.Config.Target); err != nil {
			return err
		}
	}
	canonical, _ := TargetFor(s.Config.Target)
	seen := map[string]bool{}
	// Validate the entire recovery plan before restoring any file.
	for _, pre := range j.Changes {
		if SafeRelative(pre.Destination) != nil {
			return errors.New("invalid recovery destination")
		}
		source := pre.Destination
		if canonical.Mission == "empty.deerisle" && strings.HasPrefix(source, "mpmissions/empty.deerisle/") {
			source = strings.TrimPrefix(source, "mpmissions/")
		}
		dest, owned := Destination(canonical, source)
		if !owned || dest != pre.Destination || seen[strings.ToLower(dest)] {
			return errors.New("unsafe recovery ownership record")
		}
		seen[strings.ToLower(dest)] = true
		if _, err := SafeJoin(s.Config.InstallDir, pre.Destination); err != nil {
			return err
		}
		if pre.Existed {
			backup, err := SafeJoin(s.Config.StateDir, "backups/"+j.Operation+"/"+pre.Destination)
			if err != nil {
				return err
			}
			info, err := os.Stat(backup)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || info.Size() > MaxFileBytes {
				return errors.New("invalid recovery backup")
			}
			raw, err := os.ReadFile(backup)
			if err != nil {
				return err
			}
			if Digest(raw) != pre.SHA256 {
				return errors.New("recovery backup hash mismatch; leaving live files untouched")
			}
		}
	}
	var failures []error
	for _, pre := range j.Changes {
		p, _ := SafeJoin(s.Config.InstallDir, pre.Destination)
		if pre.Existed {
			backup, _ := SafeJoin(s.Config.StateDir, "backups/"+j.Operation+"/"+pre.Destination)
			err := AtomicCopy(backup, p, os.FileMode(pre.Mode))
			if err != nil {
				failures = append(failures, err)
			}
		} else if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return err
	}
	s.State.Installed = j.Before
	return s.Save()
}
func (s *Store) Recover() error {
	var j Journal
	err := ReadJSON(filepath.Join(s.Config.StateDir, "journal.json"), &j)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !j.Committed {
		if err = s.restore(j); err != nil {
			return err
		}
		result := Result{Version: Protocol, ID: j.Operation, Target: s.Config.Target, Status: "failed", Phase: "recovered", Error: "agent interrupted; pre-deployment configuration restored", Updated: time.Now().UTC()}
		if err = WriteJSON(filepath.Join(s.Config.StateDir, "results", j.Operation+".json"), result); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(s.Config.StateDir, "journal.json"))
}
func (s *Store) Retain() error {
	entries, err := os.ReadDir(filepath.Join(s.Config.StateDir, "releases"))
	if err != nil {
		return err
	}
	var ids []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".success.json") {
			id := strings.TrimSuffix(e.Name(), ".success.json")
			if ValidID(id) {
				ids = append(ids, id)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	for i, id := range ids {
		if i < 10 || s.State.Installed != nil && s.State.Installed.ID == id {
			continue
		}
		for _, suffix := range []string{".zip", ".success.json"} {
			if err = os.Remove(filepath.Join(s.Config.StateDir, "releases", id+suffix)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	// Backup deletion is confined to validated operation-ID directories and keeps ten successful operations.
	results, err := os.ReadDir(filepath.Join(s.Config.StateDir, "results"))
	if err != nil {
		return err
	}
	var successes []string
	for _, e := range results {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if !ValidID(id) {
			continue
		}
		var r Result
		if ReadJSON(filepath.Join(s.Config.StateDir, "results", e.Name()), &r) == nil && r.Status == "succeeded" {
			successes = append(successes, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(successes)))
	for i, id := range successes {
		if i < 10 {
			continue
		}
		p, err := SafeJoin(s.Config.StateDir, "backups/"+id)
		if err != nil {
			return err
		}
		if err = os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
