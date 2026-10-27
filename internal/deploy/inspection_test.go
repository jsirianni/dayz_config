package deploy

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/hostinspect"
	"github.com/jsirianni/dayz_config/internal/lifecycle"
)

func TestAssessActualIdentityAndReadiness(t *testing.T) {
	now := time.Now().UTC()
	target, _ := TargetFor("nasdara-staging")
	state := State{PID: 42, ProcessToken: "one", Phase: "complete"}
	good := hostinspect.Snapshot{At: now, Service: hostinspect.Service{State: "Running", StartMode: "Auto", PID: 7}, Processes: []hostinspect.Process{{PID: 42, Parent: 7, Token: "one"}}, Ports: []int{2901, 2903, 27018}}
	for _, tt := range []struct {
		name   string
		change func(*State, *hostinspect.Snapshot, *time.Time)
		bad    bool
	}{
		{"healthy", func(*State, *hostinspect.Snapshot, *time.Time) {}, false},
		{"stale", func(_ *State, _ *hostinspect.Snapshot, h *time.Time) { *h = now.Add(-time.Minute) }, true},
		{"pid reused", func(_ *State, s *hostinspect.Snapshot, _ *time.Time) { s.Processes[0].Token = "another" }, true},
		{"wrong agent", func(_ *State, s *hostinspect.Snapshot, _ *time.Time) { s.Processes[0].Parent = 8 }, true},
		{"duplicate", func(_ *State, s *hostinspect.Snapshot, _ *time.Time) {
			s.Processes = append(s.Processes, s.Processes[0])
		}, true},
		{"query", func(_ *State, s *hostinspect.Snapshot, _ *time.Time) { s.Query.Error = "timeout" }, true},
		{"listener", func(_ *State, s *hostinspect.Snapshot, _ *time.Time) { s.Ports = nil }, true},
		{"paused", func(s *State, _ *hostinspect.Snapshot, _ *time.Time) { s.Paused = true }, true},
		{"pending warning", func(s *State, _ *hostinspect.Snapshot, _ *time.Time) { s.RestartRequired = true }, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := state
			v := good
			v.Processes = append([]hostinspect.Process{}, good.Processes...)
			h := now
			tt.change(&s, &v, &h)
			_, failures, _ := Assess(s, &v, h, target, now)
			if (len(failures) > 0) != tt.bad {
				t.Fatal(failures)
			}
		})
	}
	if status, errs, _ := Assess(state, nil, now, target, now); status != "unhealthy" || len(errs) == 0 {
		t.Fatal("missing collector was accepted")
	}
}

func TestLogExitRequiresEvidence(t *testing.T) {
	if _, ok := LogExit("nasdara", "one.rpt", time.Now(), "old log without termination"); ok {
		t.Fatal("inferred clean exit")
	}
	e, ok := LogExit("nasdara", "one.rpt", time.Now(), "[Shutdown] Shutting down\n[Shutdown] Saving players\n--- Termination successfully completed ---")
	if !ok || e.Cause != "scheduled" {
		t.Fatal(e)
	}
	e, _ = LogExit("nasdara", "one.rpt", time.Now(), "--- Termination successfully completed ---")
	if e.Cause != "unknown" {
		t.Fatal(e)
	}
}

func TestObservationAndHistoryReadOnly(t *testing.T) {
	now := time.Now().UTC()
	endpoint, root := sshExecFixture(t, func(cmd string, _ io.Reader, w io.Writer) error {
		if !strings.HasPrefix(cmd, "powershell.exe ") {
			t.Error(cmd)
		}
		return json.NewEncoder(w).Encode(hostinspect.Snapshot{At: now})
	})
	host := testConfig(t)
	if err := lifecycle.Write(root, lifecycle.Event{At: now, Target: "nasdara-staging", Kind: "launch", PID: 42}); err != nil {
		t.Fatal(err)
	}
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	target, _ := TargetFor(host.Target)
	if _, err = r.observe(context.Background(), host, target); err != nil {
		t.Fatal(err)
	}
	h := r.History(host.Target, host, now.Add(-time.Hour))
	if len(h.Events) != 1 || !h.Incomplete {
		t.Fatalf("history %+v", h)
	}
	if _, err = os.Stat(filepath.Join(root, "inbox")); !os.IsNotExist(err) {
		t.Fatal("inspection wrote remote requests")
	}
	if err = r.RequireFeature(host.Target, "hard-stop.v1"); err != ErrUnsupportedFeature {
		t.Fatal(err)
	}
}

func TestFleetRetainsAllFailuresSorted(t *testing.T) {
	endpoint, _ := sshFixture(t)
	reports := Fleet(context.Background(), map[string]Endpoint{"nasdara-staging": endpoint, "nasdara": {}, "deer-isle": {}}, time.Now().Add(-time.Hour))
	if len(reports) != 3 || reports[0].Target != "deer-isle" || reports[2].Target != "nasdara-staging" {
		t.Fatal(reports)
	}
	for _, r := range reports {
		if len(r.Errors) == 0 {
			t.Fatal("lost failure", r)
		}
	}
}
