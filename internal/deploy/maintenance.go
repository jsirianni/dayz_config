package deploy

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const HardStopCapability = "hard-stop.v1"

type HardStopper interface {
	HardStop(context.Context, State) error
}

func (a *Agent) stopMode(ctx context.Context, hard bool) error {
	if !hard {
		return a.stop(ctx)
	}
	stopper, ok := a.Game.(HardStopper)
	if !ok {
		return errors.New("game implementation does not support guarded hard stops")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := stopper.HardStop(stopCtx, a.Store.State); err != nil {
		return err
	}
	a.event("exit", "hard", nil)
	a.Store.State.PID = 0
	a.Store.State.ProcessToken = ""
	a.Store.State.RunningCommit = ""
	a.Store.State.DZSA = nil
	return a.Store.Save()
}

func validateExpectedState(state State, req Request) error {
	installed := ""
	if state.Installed != nil {
		installed = state.Installed.ID
	}
	if state.PID != req.ExpectedPID || state.ProcessToken != req.ExpectedToken || installed != req.ExpectedRelease {
		return errors.New("managed process or installed release changed since inspection; inspect status and retry")
	}
	return nil
}

func (a *Agent) pauseAndStop(ctx context.Context, r *Result, hard bool) error {
	wasPaused := a.Store.State.Paused
	a.Store.State.Paused = true
	r.StopMethod = "graceful"
	if hard {
		r.StopMethod = "hard"
	}
	if err := a.progress(r, "stopping"); err != nil {
		return err
	}
	if err := a.stopMode(ctx, hard); err != nil {
		if a.Game.Alive(a.Store.State) {
			a.Store.State.Paused = wasPaused
		}
		_ = a.Store.Save()
		return err
	}
	return nil
}

func (r *Remote) PreviewMaintenance(ctx context.Context, alias string) (map[string]any, error) {
	state, err := r.State(alias)
	if err != nil {
		return nil, err
	}
	var host HostConfig
	if err = r.ReadJSON("host.json", &host); err != nil {
		return nil, err
	}
	if err = host.Validate(); err != nil || host.Target != alias {
		return nil, errors.New("invalid host identity")
	}
	target, _ := TargetFor(alias)
	if state.Installed != nil {
		target = state.Installed.Launcher
	}
	observed, err := r.observe(ctx, host, target)
	if err != nil {
		return nil, err
	}
	featureErr := r.RequireFeature(alias, HardStopCapability)
	if featureErr != nil && !errors.Is(featureErr, ErrUnsupportedFeature) {
		return nil, featureErr
	}
	if len(observed.Processes) > 1 {
		return nil, errors.New("duplicate game processes block maintenance")
	}
	if len(observed.Processes) == 1 && (observed.Processes[0].PID != state.PID || observed.Processes[0].Token != state.ProcessToken) {
		return nil, errors.New("native process identity differs from managed state")
	}
	return map[string]any{"target": alias, "hardStop": true, "pid": state.PID, "willStop": len(observed.Processes) > 0, "upgradeRequired": featureErr != nil, "ports": "preserved", "preview": true}, nil
}

func (r *Remote) CheckHardRequest(req Request) error {
	if !req.Hard {
		return nil
	}
	if req.Action != "restart" && !((req.Action == "deploy" || req.Action == "rollback") && req.Restart) {
		return fmt.Errorf("hard stop is invalid for %s without a requested restart", req.Action)
	}
	return r.RequireFeature(req.Target, HardStopCapability)
}
