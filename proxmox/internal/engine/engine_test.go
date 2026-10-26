package engine_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jsirianni/dayz_config/proxmox/internal/engine"
	"github.com/jsirianni/dayz_config/proxmox/internal/guest"
	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/testutil"
)

type checker struct {
	calls int
	err   error
}

func (g *checker) Ready(context.Context, *pve.Client, string, int, spec.Network, spec.Guest) (guest.Access, error) {
	g.calls++
	return guest.Access{Endpoint: "pveadmin@192.0.2.20:22", Fingerprint: "SHA256:verified"}, g.err
}
func setup(t *testing.T) (*engine.Engine, *testutil.API, *checker) {
	a := testutil.New(t)
	g := &checker{}
	e := &engine.Engine{Client: a.Client(), Store: operation.Store{Dir: t.TempDir()}, Context: "lab", Identity: "context-identity", Guest: g}
	return e, a, g
}
func TestDeployDryRunHasNoSideEffects(t *testing.T) {
	e, a, g := setup(t)
	r, err := e.Deploy(context.Background(), testutil.Deploy(t), true)
	if err != nil || r.Status != "planned" {
		t.Fatal(r, err)
	}
	if len(a.Writes()) != 0 || g.calls != 0 {
		t.Fatal("dry-run mutated or executed guest commands")
	}
}
func TestDeployAndResumeReadiness(t *testing.T) {
	e, a, g := setup(t)
	g.err = errors.New("SSH not yet reachable")
	s := testutil.Deploy(t)
	r, err := e.Deploy(context.Background(), s, false)
	if err == nil || r.OperationID == "" {
		t.Fatal(r, err)
	}
	firstWrites := len(a.Writes())
	g.err = nil
	r, err = e.Resume(context.Background(), r.OperationID)
	if err != nil || r.Status != "succeeded" || r.SSHEndpoint == "" {
		t.Fatal(r, err)
	}
	if len(a.Writes()) != firstWrites {
		t.Fatal("resume repeated completed mutation")
	}
	cfg := a.GetVM(s.VMID)
	if !strings.Contains(cfg.String("net0"), "queues=4") || !strings.Contains(cfg.String("net0"), "firewall=1") {
		t.Fatal(cfg)
	}
	if cfg.String("ciuser") != "pveadmin" || cfg.String("citype") != "nocloud" {
		t.Fatal(cfg)
	}
	if !strings.Contains(cfg.String("sshkeys"), "%20") {
		t.Fatal("public key not URI encoded")
	}
}
func TestCollisionAndOwnership(t *testing.T) {
	e, a, g := setup(t)
	s := testutil.Deploy(t)
	a.SetVM(201, pve.Config{"name": "foreign", "digest": "x"})
	if _, err := e.Deploy(context.Background(), s, false); err == nil {
		t.Fatal("collision accepted")
	}
	if len(a.Writes()) != 0 {
		t.Fatal("collision wrote state")
	}
	delete(a.VMs, 201)
	g.err = errors.New("offline")
	r, _ := e.Deploy(context.Background(), s, false)
	cfg := a.GetVM(201)
	cfg["description"] = "foreign"
	a.SetVM(201, cfg)
	writes := len(a.Writes())
	if _, err := e.Resume(context.Background(), r.OperationID); err == nil {
		t.Fatal("ownership change accepted")
	}
	if len(a.Writes()) != writes {
		t.Fatal("changed foreign VM")
	}
}
func TestLostCloneReplyNeverResubmits(t *testing.T) {
	e, a, _ := setup(t)
	a.LoseCloneReply = true
	r, err := e.Deploy(context.Background(), testutil.Deploy(t), false)
	if !engine.IsUnknown(err) || r.Status != "unknown" {
		t.Fatal(r, err)
	}
	if _, err = e.Resume(context.Background(), r.OperationID); err == nil {
		t.Fatal("unresolved submission resumed")
	}
	if len(a.Writes()) != 1 {
		t.Fatal("clone resubmitted")
	}
}
func TestLostConfigureReplyCannotCompleteUsingCloneTask(t *testing.T) {
	e, a, _ := setup(t)
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/201/config") {
			http.Error(w, "proxy unavailable", 503)
			return true
		}
		return false
	}
	r, err := e.Deploy(context.Background(), testutil.Deploy(t), false)
	if !engine.IsUnknown(err) {
		t.Fatal(err)
	}
	writes := len(a.Writes())
	if _, err = e.Resume(context.Background(), r.OperationID); err == nil {
		t.Fatal("uncertain configure task silently completed")
	}
	if len(a.Writes()) != writes {
		t.Fatal("resume wrote after uncertain submission")
	}
}
func TestMutationsDryRunAndShrinkRejection(t *testing.T) {
	e, a, _ := setup(t)
	cfg := a.GetVM(9000)
	cfg["template"] = 0
	a.SetVM(201, cfg)
	for _, test := range []struct {
		action, disk, size string
		changes            url.Values
	}{{action: "start"}, {action: "shutdown"}, {action: "reboot"}, {action: "stop"}, {disk: "scsi0", size: "64G"}, {changes: url.Values{"memory": {"4096"}}}} {
		if _, err := e.Mutate(context.Background(), 201, "test", test.changes, test.action, test.disk, test.size, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Mutate(context.Background(), 201, "resize", nil, "", "scsi0", "16G", false); err == nil {
		t.Fatal("shrink accepted")
	}
	if len(a.Writes()) != 0 {
		t.Fatal(a.Writes())
	}
}
func TestConcurrentDigestChange(t *testing.T) {
	e, a, _ := setup(t)
	cfg := a.GetVM(9000)
	cfg["template"] = 0
	a.SetVM(201, cfg)
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/config") {
			http.Error(w, "digest mismatch", 409)
			return true
		}
		return false
	}
	r, err := e.Mutate(context.Background(), 201, "update", url.Values{"memory": {"4096"}}, "", "", "", false)
	if err == nil || r.Status != "failed" {
		t.Fatal(r, err)
	}
	if len(a.Writes()) != 1 {
		t.Fatal("digest failure retried")
	}
}
func TestJournalTamperRejected(t *testing.T) {
	e, _, g := setup(t)
	g.err = errors.New("offline")
	r, _ := e.Deploy(context.Background(), testutil.Deploy(t), false)
	r.Deploy.Name = "tampered"
	if err := e.Store.Save(r); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Resume(context.Background(), r.OperationID); err == nil {
		t.Fatal("tampered input accepted")
	}
}
func TestPermissionsAreCheckedBeforeClone(t *testing.T) {
	e, a, _ := setup(t)
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/access/permissions") && r.URL.Query().Get("path") == "/pool/lab" {
			testutil.Data(w, map[string]any{"/pool/lab": map[string]int{"VM.Allocate": 1}})
			return true
		}
		return false
	}
	if _, err := e.Deploy(context.Background(), testutil.Deploy(t), false); err == nil {
		t.Fatal("missing configuration permissions accepted")
	}
	if len(a.Writes()) != 0 {
		t.Fatal("cloned before checking permissions")
	}
}
func TestUnchangedUpdateStillReportsPendingRestart(t *testing.T) {
	e, a, _ := setup(t)
	cfg := a.GetVM(9000)
	cfg["template"] = 0
	a.SetVM(201, cfg)
	r, err := e.Mutate(context.Background(), 201, "update", url.Values{}, "", "", "", false)
	if err != nil || r.Phase != "unchanged" || len(r.Pending.([]pve.Pending)) == 0 {
		t.Fatal(r, err)
	}
	if len(a.Writes()) != 0 {
		t.Fatal("unchanged update wrote")
	}
}
func TestResumeRejectsChangedSSHIdentity(t *testing.T) {
	e, a, g := setup(t)
	g.err = errors.New("offline")
	s := testutil.Deploy(t)
	r, _ := e.Deploy(context.Background(), s, false)
	replacement, err := os.ReadFile(testutil.Key(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(s.Guest.KeyFile, replacement, 0600); err != nil {
		t.Fatal(err)
	}
	writes := len(a.Writes())
	if _, err = e.Resume(context.Background(), r.OperationID); err == nil {
		t.Fatal("changed SSH identity accepted")
	}
	if len(a.Writes()) != writes {
		t.Fatal("mutation after key change")
	}
}
func TestPowerWaitsForObservedStateAfterTaskSuccess(t *testing.T) {
	e, a, _ := setup(t)
	cfg := a.GetVM(9000)
	cfg["template"] = 0
	cfg["status"] = "running"
	a.SetVM(201, cfg)
	observations := 0
	a.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/cluster/resources") && len(a.Writes()) > 0 {
			observations++
			if observations < 3 {
				testutil.Data(w, []pve.Resource{{VMID: 201, Node: "pve", Type: "qemu", Status: "running"}})
				return true
			}
		}
		return false
	}
	r, err := e.Mutate(context.Background(), 201, "shutdown", nil, "shutdown", "", "", false)
	if err != nil || r.Status != "succeeded" || observations < 3 {
		t.Fatal(r, err, observations)
	}
}
