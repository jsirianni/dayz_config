package operation

import (
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"testing"
)

func TestJournalReplacementAndLock(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	r := &Result{APIVersion: spec.Version, OperationID: NewID(), Status: "running"}
	if e := s.Save(r); e != nil {
		t.Fatal(e)
	}
	r.Status = "succeeded"
	if e := s.Save(r); e != nil {
		t.Fatal(e)
	}
	loaded, e := s.Load(r.OperationID)
	if e != nil || loaded.Status != "succeeded" {
		t.Fatal(loaded, e)
	}
	unlock, e := s.Lock("lab", 201)
	if e != nil {
		t.Fatal(e)
	}
	if release, e := s.Lock("lab", 201); e == nil {
		release()
		t.Fatal("duplicate lock")
	}
	unlock()
	release, e := s.Lock("lab", 201)
	if e != nil {
		t.Fatal(e)
	}
	release()
	if _, e = s.Load("../escape"); e == nil {
		t.Fatal("traversal")
	}
}
