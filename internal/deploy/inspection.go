package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jsirianni/dayz_config/internal/hostinspect"
)

type HealthReport struct {
	Target         string                `json:"target"`
	CheckedAt      time.Time             `json:"checkedAt"`
	Status         string                `json:"status"`
	Installed      string                `json:"installedRevision,omitempty"`
	Startup        string                `json:"lastStartupRevision,omitempty"`
	RestartPending bool                  `json:"restartPending"`
	Paused         bool                  `json:"paused"`
	Snapshot       *hostinspect.Snapshot `json:"snapshot,omitempty"`
	History        HistoryReport         `json:"history"`
	DZSA           *DZSAReport           `json:"dzsa,omitempty"`
	Errors         []string              `json:"errors,omitempty"`
	Warnings       []string              `json:"warnings,omitempty"`
}

func remoteInstall(c HostConfig) string {
	return "/" + strings.TrimPrefix(strings.ReplaceAll(c.InstallDir, "\\", "/"), "/")
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("remote inspection exceeds output limit")
	}
	return b.Buffer.Write(p)
}

func (r *Remote) observe(ctx context.Context, c HostConfig, t Target) (*hostinspect.Snapshot, error) {
	command, err := hostinspect.Command(hostinspect.Spec{Service: ServiceName(c.Target), Executable: strings.TrimRight(c.InstallDir, "\\/") + `\DayZServer_x64.exe`, QueryPort: t.QueryPort})
	if err != nil {
		return nil, err
	}
	session, err := r.SSH.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	stop := context.AfterFunc(ctx, func() { session.Close() })
	defer stop()
	out := &boundedBuffer{limit: 64 << 10}
	stderr := &boundedBuffer{limit: 16 << 10}
	session.Stdout = out
	session.Stderr = stderr
	if err = session.Run(command); err != nil {
		return nil, fmt.Errorf("read-only Windows inspection unavailable: %w", err)
	}
	var s hostinspect.Snapshot
	if err = json.Unmarshal(out.Bytes(), &s); err != nil {
		return nil, fmt.Errorf("invalid Windows inspection response: %w", err)
	}
	if s.At.IsZero() {
		return nil, errors.New("Windows inspection omitted observation time")
	}
	return &s, nil
}

// Assess never treats persisted deployment state as proof that a process exists.
func Assess(state State, s *hostinspect.Snapshot, heartbeat time.Time, t Target, now time.Time) (string, []string, []string) {
	var failures, warnings []string
	status := "running"
	if s == nil {
		return "unhealthy", []string{"native process/service inspection is unavailable"}, nil
	}
	if heartbeat.IsZero() || now.Sub(heartbeat) > 15*time.Second || heartbeat.Sub(now) > 15*time.Second {
		failures = append(failures, "agent heartbeat is stale or unavailable")
	}
	if s.Service.State != "Running" {
		failures = append(failures, "managed service is not running")
	}
	if s.Service.StartMode != "Auto" {
		failures = append(failures, "managed service is not automatic-start")
	}
	if state.Error != "" {
		failures = append(failures, "agent: "+state.Error)
	}
	if state.Paused {
		failures = append(failures, "automatic supervision is paused")
	}
	if state.RestartRequired {
		warnings = append(warnings, "installed configuration has pending activation")
	}
	if len(s.Processes) == 0 {
		status = "stopped"
		failures = append(failures, "no game process in the managed installation")
	} else if len(s.Processes) != 1 {
		failures = append(failures, "multiple game processes in the managed installation")
	} else {
		p := s.Processes[0]
		if p.PID != state.PID || p.Token != state.ProcessToken || p.Parent != s.Service.PID {
			failures = append(failures, "game process identity does not match the managed agent")
		}
		for _, port := range []int{t.GamePort, t.GamePort + 2, t.QueryPort} {
			found := false
			for _, v := range s.Ports {
				found = found || port == v
			}
			if !found {
				failures = append(failures, fmt.Sprintf("expected UDP listener %d is missing", port))
			}
		}
		if s.Query.Error != "" {
			failures = append(failures, "local query: "+s.Query.Error)
		}
	}
	switch state.Phase {
	case "updating", "applying", "starting", "checking", "provisioning", "stopping":
		status = state.Phase
	}
	if len(failures) > 0 && status == "running" {
		status = "unhealthy"
	}
	return status, failures, warnings
}

func (r *Remote) Health(ctx context.Context, alias string, since time.Time) HealthReport {
	report := HealthReport{Target: alias, CheckedAt: time.Now().UTC(), Status: "unhealthy"}
	s, err := r.State(alias)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		return report
	}
	var host HostConfig
	if err = r.ReadJSON("host.json", &host); err != nil {
		report.Errors = append(report.Errors, err.Error())
		return report
	}
	if err = host.Validate(); err != nil || host.Target != alias {
		report.Errors = append(report.Errors, "host configuration identity is invalid")
		return report
	}
	t, _ := TargetFor(alias)
	if s.Installed != nil {
		t = s.Installed.Launcher
		report.Installed = s.Installed.Commit
	}
	report.Startup = s.RunningCommit
	report.Paused = s.Paused
	report.RestartPending = s.RestartRequired
	snapshot, err := r.observe(ctx, host, t)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	report.Snapshot = snapshot
	// A native collector can take seconds while Windows initializes modules.
	// Evaluate freshness at the end of collection, not against the earlier read.
	var heartbeat Heartbeat
	_ = r.ReadJSON("heartbeat.json", &heartbeat)
	if heartbeat.Version != Protocol {
		report.Errors = append(report.Errors, "agent heartbeat protocol is incompatible")
	}
	report.CheckedAt = time.Now().UTC()
	changing := false
	if latest, e := r.State(alias); e == nil && (latest.PID != s.PID || latest.ProcessToken != s.ProcessToken || latest.LastOperation != s.LastOperation) {
		changing = true
		report.Errors = append(report.Errors, "managed state changed during inspection; rerun for a coherent snapshot")
	}
	status, failures, warnings := Assess(s, snapshot, heartbeat.Updated, t, report.CheckedAt)
	report.Status = status
	if changing {
		report.Status = "changing"
	}
	report.Errors = append(report.Errors, failures...)
	report.Warnings = append(report.Warnings, warnings...)
	if snapshot != nil && snapshot.Query.Error == "" && len(snapshot.Processes) > 0 {
		f, e := r.SFTP.Open(path.Join(remoteInstall(host), "serverDZ.cfg"))
		if e == nil {
			raw, readErr := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
			f.Close()
			if readErr == nil && len(raw) <= MaxFileBytes {
				name, _ := configuredHostname(raw)
				e = verifyServerIdentity(t, name, snapshot.Query.Name, snapshot.Query.Map)
			} else {
				e = errors.New("cannot read bounded hostname")
			}
		}
		if e != nil {
			report.Errors = append(report.Errors, e.Error())
		}
	}
	report.History = r.History(alias, host, since)
	if report.History.Incomplete {
		report.Warnings = append(report.Warnings, "restart history coverage is incomplete; consult history output")
	}
	if report.DZSA, err = r.CheckDZSA(ctx, alias); err != nil {
		report.Errors = append(report.Errors, "fresh DZSA verification: "+err.Error())
	}
	if len(report.Errors) > 0 && report.Status == "running" {
		report.Status = "unhealthy"
	}
	return report
}

// Fleet bounds concurrent SSH work and preserves failed targets in sorted output.
func Fleet(ctx context.Context, targets map[string]Endpoint, since time.Time) []HealthReport {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	reports := make([]HealthReport, len(names))
	slots := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				reports[i] = HealthReport{Target: name, Status: "unreachable", Errors: []string{ctx.Err().Error()}}
				return
			}
			defer func() { <-slots }()
			hostCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			remote, err := Connect(hostCtx, targets[name])
			if err != nil {
				reports[i] = HealthReport{Target: name, CheckedAt: time.Now().UTC(), Status: "unreachable", Errors: []string{err.Error()}}
				return
			}
			defer remote.Close()
			reports[i] = remote.Health(hostCtx, name, since)
		}(i, name)
	}
	wg.Wait()
	return reports
}
