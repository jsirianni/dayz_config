package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type hardFake struct {
	*fakeGame
	hardStops int
	hardErr   error
}

func (g *hardFake) HardStop(_ context.Context, state State) error {
	g.hardStops++
	if !g.store.State.Paused {
		return errors.New("supervision was not paused")
	}
	if state.PID != g.store.State.PID {
		return errors.New("unexpected process")
	}
	if g.hardErr != nil {
		return g.hardErr
	}
	g.alive = false
	g.events = append(g.events, "hard-stop")
	return nil
}

func TestHardRestartModesAndRepeatedDelivery(t *testing.T) {
	for _, action := range []string{"restart", "deploy", "rollback"} {
		t.Run(action, func(t *testing.T) {
			s := testStore(t)
			g := &hardFake{fakeGame: &fakeGame{store: s}}
			a := Agent{Store: s, Game: g}
			b := testBundle(t, nil)
			if r := a.Process(context.Background(), submitTest(t, s, b, true)); r.Status != "succeeded" {
				t.Fatal(r)
			}
			g.events = nil
			req := Request{Version: Protocol, ID: NewID(), Target: s.Config.Target, Action: action, Restart: true, Hard: true, ExpectedPID: s.State.PID, ExpectedToken: s.State.ProcessToken, ExpectedRelease: s.State.Installed.ID}
			if action == "deploy" {
				next := testBundle(t, nil)
				req = submitTest(t, s, next, true)
				req.Hard = true
				req.ExpectedPID = s.State.PID
				req.ExpectedToken = s.State.ProcessToken
				req.ExpectedRelease = s.State.Installed.ID
			}
			if action == "rollback" {
				req.Deployment = b.Manifest.ID
			}
			before := g.hardStops
			result := a.Process(context.Background(), req)
			if result.Status != "succeeded" || result.StopMethod != "hard" || g.hardStops != before+1 {
				t.Fatal(result, g.events)
			}
			for _, event := range g.events {
				if event == "stop" {
					t.Fatal("hard request used graceful/RCon stop")
				}
			}
			if again := a.Process(context.Background(), req); again.Status != "succeeded" || g.hardStops != before+1 {
				t.Fatal("repeated request killed another process", again)
			}
		})
	}
}

func TestHardStopRefusalsPrecedeTermination(t *testing.T) {
	s := testStore(t)
	g := &hardFake{fakeGame: &fakeGame{store: s}}
	a := Agent{Store: s, Game: g}
	b := testBundle(t, nil)
	if r := a.Process(context.Background(), submitTest(t, s, b, true)); r.Status != "succeeded" {
		t.Fatal(r)
	}
	req := Request{Version: Protocol, ID: NewID(), Target: s.Config.Target, Action: "restart", Hard: true, ExpectedPID: s.State.PID, ExpectedToken: "reused", ExpectedRelease: s.State.Installed.ID}
	r := a.Process(context.Background(), req)
	if r.Status != "failed" || g.hardStops != 0 || !strings.Contains(r.Error, "changed since inspection") {
		t.Fatal(r)
	}
	req.ID = NewID()
	req.ExpectedToken = s.State.ProcessToken
	g.hardErr = errors.New("identity guard rejected")
	r = a.Process(context.Background(), req)
	if r.Status != "failed" || !g.alive || s.State.Paused {
		t.Fatal(r, s.State)
	}
}

func TestUnsupportedAgentCannotReceiveHardRequest(t *testing.T) {
	endpoint, root := sshFixture(t)
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	req := Request{Version: Protocol, ID: NewID(), Target: "nasdara-staging", Action: "restart", Hard: true}
	if err = r.Submit(req, nil); !errors.Is(err, ErrUnsupportedFeature) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "inbox")); !os.IsNotExist(err) {
		t.Fatal("unsupported request was uploaded", err)
	}
}
