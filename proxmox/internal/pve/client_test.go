package pve_test

import (
	"context"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/testutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTokenFormAndPending(t *testing.T) {
	a := testutil.New(t)
	c := a.Client()
	if _, e := c.CheckVersion(context.Background()); e != nil {
		t.Fatal(e)
	}
	upid, e := c.Submit(context.Background(), "POST", pve.Path("pve", 9000)+"/config", url.Values{"description": {"line one\nline two & +"}})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Wait(context.Background(), "pve", upid); e != nil {
		t.Fatal(e)
	}
	if a.GetVM(9000).String("description") != "line one\nline two & +" {
		t.Fatal("form encoding damaged")
	}
	pending, e := c.Pending(context.Background(), "pve", 9000)
	if e != nil || len(pending) != 1 {
		t.Fatal(pending, e)
	}
}
func TestTaskFailureAndTimeout(t *testing.T) {
	a := testutil.New(t)
	c := a.Client()
	a.TaskExit = "ERROR: storage unavailable"
	if e := c.Wait(context.Background(), "pve", "UPID:x"); e == nil || !strings.Contains(e.Error(), "fixture task log") {
		t.Fatal(e)
	}
	a.TaskState = "running"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if e := c.Wait(ctx, "pve", "UPID:x"); e == nil {
		t.Fatal("timeout ignored")
	}
}
func TestRejectMajorAndRedactErrors(t *testing.T) {
	a := testutil.New(t)
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/version") {
			testutil.Data(w, map[string]string{"version": "8.4"})
			return true
		}
		http.Error(w, "top-secret-token echoed", 403)
		return true
	}
	c := a.Client()
	if _, e := c.CheckVersion(context.Background()); e == nil {
		t.Fatal("accepted PVE 8")
	}
	var out any
	e := c.Do(context.Background(), "GET", "/test", nil, &out)
	if e == nil || strings.Contains(e.Error(), "top-secret-token") {
		t.Fatal(e)
	}
}
func TestMutationNeverRetried(t *testing.T) {
	a := testutil.New(t)
	a.LoseCloneReply = true
	c := a.Client()
	_, e := c.Submit(context.Background(), "POST", pve.Path("pve", 9000)+"/clone", url.Values{"newid": {"201"}})
	if e == nil {
		t.Fatal("expected lost reply")
	}
	if len(a.Writes()) != 1 {
		t.Fatal("mutation retried")
	}
}
func TestPendingFiltersCurrentValuesButRetainsFalseAndDeletion(t *testing.T) {
	a := testutil.New(t)
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/pending") {
			testutil.Data(w, []map[string]any{{"key": "cores", "value": 2}, {"key": "onboot", "value": true, "pending": false}, {"key": "net1", "value": "virtio=...", "delete": 1}})
			return true
		}
		return false
	}
	rows, err := a.Client().Pending(context.Background(), "pve", 9000)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
}
