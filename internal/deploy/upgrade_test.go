package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/hostinspect"
)

func TestUpgradeIdleRejectsUnfinishedWork(t *testing.T) {
	for _, kind := range []string{"idle", "upload", "journal", "restore", "result", "phase"} {
		t.Run(kind, func(t *testing.T) {
			s := testStore(t)
			if err := s.Save(); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "upload":
				writeTest(t, s.Config.StateDir, "inbox/package.upload-pending", "partial")
			case "journal":
				writeTest(t, s.Config.StateDir, "journal.json", "{}")
			case "restore":
				writeTest(t, s.Config.StateDir, "restore.json", "{}")
			case "result":
				if err := WriteJSON(filepath.Join(s.Config.StateDir, "results", NewID()+".json"), Result{Status: "running"}); err != nil {
					t.Fatal(err)
				}
			case "phase":
				s.State.Phase = "updating"
				if err := s.Save(); err != nil {
					t.Fatal(err)
				}
			}
			if err := upgradeIdle(s.Config); (err != nil) != (kind != "idle") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}

func TestCooperativeHandoffPreservesGameAndConfigurationState(t *testing.T) {
	s := testStore(t)
	s.State.PID = 42
	s.State.ProcessToken = "same-game"
	s.State.RunningCommit = strings.Repeat("b", 40)
	s.State.RestartRequired = true
	s.State.Paused = false
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	g := &fakeGame{store: s, alive: true}
	requests := make(chan struct{}, 1)
	requests <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunAgent(withHandoff(ctx, (<-chan struct{})(requests)), s.Config, g); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 0 || !g.alive {
		t.Fatalf("handoff performed game actions: %v", g.events)
	}
	var got State
	if err := ReadJSON(filepath.Join(s.Config.StateDir, "state.json"), &got); err != nil {
		t.Fatal(err)
	}
	if got.PID != 42 || got.ProcessToken != "same-game" || got.RunningCommit != s.State.RunningCommit || !got.RestartRequired || got.Paused {
		t.Fatalf("handoff changed game state: %+v", got)
	}
}

func TestCooperativeHandoffRejectsQueuedWorkWithoutStoppingGame(t *testing.T) {
	s := testStore(t)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	writeTest(t, s.Config.StateDir, "inbox/pending.upload", "partial")
	g := &fakeGame{store: s, alive: true}
	requests := make(chan struct{}, 1)
	requests <- struct{}{}
	if err := RunAgent(withHandoff(context.Background(), (<-chan struct{})(requests)), s.Config, g); err == nil {
		t.Fatal("accepted pending upload")
	}
	if len(g.events) != 0 || !g.alive {
		t.Fatalf("rejection acted on game: %v", g.events)
	}
}

func TestAdoptionRequiresExactProtectedProcessEvidence(t *testing.T) {
	now := time.Now().UTC()
	target, _ := TargetFor("nasdara-staging")
	s := State{Target: "nasdara-staging", PID: 42, ProcessToken: "game"}
	hash := strings.Repeat("a", 64)
	proof := AdoptionProof{Version: Protocol, ID: NewID(), Target: s.Target, Parent: 7, Game: ProcessIdentity{PID: 42, Token: "game", Executable: `C:\DayZ\DayZServer_x64.exe`}, Previous: AgentIdentity{PID: 9}, Agent: AgentIdentity{PID: 11, Token: "manager", SHA256: hash}, Updated: now}
	snapshot := hostinspect.Snapshot{Service: hostinspect.Service{PID: 11, Token: "manager", SHA256: hash}, Processes: []hostinspect.Process{{PID: 42, Token: "game", Parent: 7, Executable: proof.Game.Executable}}}
	for _, kind := range []string{"valid successive handoff", "reused game pid", "reused manager pid", "different binary", "different parent", "different target", "future proof", "unrelated executable"} {
		t.Run(kind, func(t *testing.T) {
			p := proof
			switch kind {
			case "reused game pid":
				p.Game.Token = "other"
			case "reused manager pid":
				p.Agent.Token = "other"
			case "different binary":
				p.Agent.SHA256 = strings.Repeat("b", 64)
			case "different parent":
				p.Parent = 12
			case "different target":
				p.Target = "nasdara"
			case "future proof":
				p.Updated = now.Add(time.Hour)
			case "unrelated executable":
				p.Game.Executable = `C:\other\DayZServer_x64.exe`
			}
			if got := validAdoption(p, s, &snapshot, target, now); got != (kind == "valid successive handoff") {
				t.Fatalf("%s: %t", kind, got)
			}
		})
	}
}

func TestUpgradeRequestRejectsForeignIdentity(t *testing.T) {
	c := testConfig(t)
	r := UpgradeRequest{Version: Protocol, ID: NewID(), Target: c.Target, Host: c, CandidateSHA256: strings.Repeat("a", 64), Revision: strings.Repeat("b", 40)}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Host.Target = "deer-isle"
	if err := r.Validate(); err == nil {
		t.Fatal("accepted foreign host")
	}
	result := UpgradeResult{Version: Protocol, ID: NewID(), Target: c.Target, Status: "succeeded"}
	if err := result.Validate(c.Target, NewID()); err == nil {
		t.Fatal("accepted foreign operation")
	}
	// Merely reading an operation's identity does not initialize host data.
	if _, err := os.Stat(c.StateDir); !os.IsNotExist(err) {
		t.Fatalf("validation wrote host state: %v", err)
	}
}
