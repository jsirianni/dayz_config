package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) Preflight(b *Bundle) error {
	changed, deleted, err := s.Diff(b)
	if err != nil {
		return err
	}
	var required uint64 = 16 << 20
	for _, f := range b.Manifest.Files {
		required += uint64(f.Size) * 3
	}
	for _, rel := range append(append([]string{}, changed...), deleted...) {
		p, err := SafeJoin(s.Config.InstallDir, rel)
		if err != nil {
			return err
		}
		if info, err := os.Stat(p); err == nil {
			required += uint64(info.Size())
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, dir := range []string{s.Config.InstallDir, s.Config.StateDir} {
		available, err := freeSpace(dir)
		if err != nil {
			return err
		}
		if available < required {
			return fmt.Errorf("insufficient free space in %s (need at least %d bytes)", dir, required)
		}
	}
	for _, rel := range append(changed, deleted...) {
		p, err := SafeJoin(s.Config.InstallDir, rel)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		probe, err := os.CreateTemp(filepath.Dir(p), ".dzdeploy-preflight-*")
		if err != nil {
			return fmt.Errorf("destination is not writable: %s: %w", rel, err)
		}
		name := probe.Name()
		closeErr := probe.Close()
		removeErr := os.Remove(name)
		if err = errors.Join(closeErr, removeErr); err != nil {
			return err
		}
	}
	return nil
}
