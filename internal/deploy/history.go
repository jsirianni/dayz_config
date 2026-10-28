package deploy

import (
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/lifecycle"
)

type HistoryReport struct {
	Cycles        []RestartCycle    `json:"cycles"`
	Since         time.Time         `json:"since"`
	CoverageStart time.Time         `json:"coverageStart,omitzero"`
	Incomplete    bool              `json:"incomplete"`
	Warnings      []string          `json:"warnings,omitempty"`
	Events        []lifecycle.Event `json:"events"`
}

type RestartCycle struct {
	Shutdown  time.Time `json:"shutdown"`
	Ready     time.Time `json:"ready"`
	Cause     string    `json:"cause"`
	Operation string    `json:"operation"`
}

func correlateCycles(events []lifecycle.Event) []RestartCycle {
	var cycles []RestartCycle
	for _, exit := range events {
		if exit.Kind != "clean-exit" {
			continue
		}
		for _, ready := range events {
			if ready.Kind == "ready" && !ready.At.Before(exit.At) && ready.At.Sub(exit.At) <= 45*time.Minute {
				cycles = append(cycles, RestartCycle{Shutdown: exit.At, Ready: ready.At, Cause: exit.Cause, Operation: ready.Operation})
				break
			}
		}
	}
	return cycles
}

func (r *Remote) History(alias string, host HostConfig, since time.Time) HistoryReport {
	h := HistoryReport{Since: since, Events: []lifecycle.Event{}}
	if info, e := r.SFTP.Stat(r.remote("logs", "lifecycle-error.txt")); e == nil && !info.ModTime().Before(since) {
		h.Incomplete = true
		h.Warnings = append(h.Warnings, "agent recorded a lifecycle event persistence failure")
	}
	files, err := r.SFTP.ReadDir(r.remote("events"))
	if err != nil {
		h.Incomplete = true
		h.Warnings = append(h.Warnings, "durable event history unavailable on this agent")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	if len(files) > 2000 {
		files = files[:2000]
		h.Incomplete = true
		h.Warnings = append(h.Warnings, "event scan limited to latest 2000 files")
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") || !file.Mode().IsRegular() {
			continue
		}
		f, e := r.SFTP.Open(r.remote("events", file.Name()))
		if e != nil {
			h.Incomplete = true
			continue
		}
		event, e := lifecycle.Decode(f)
		f.Close()
		if e != nil || event.Target != alias {
			h.Incomplete = true
			continue
		}
		if h.CoverageStart.IsZero() || event.At.Before(h.CoverageStart) {
			h.CoverageStart = event.At
		}
		if !event.At.Before(since) {
			h.Events = append(h.Events, event)
		}
	}
	if h.CoverageStart.IsZero() || h.CoverageStart.After(since) {
		h.Incomplete = true
	}
	// Historical results and explicit log markers provide evidence on older agents.
	results, e := r.SFTP.ReadDir(r.remote("results"))
	if e != nil {
		h.Incomplete = true
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name() > results[j].Name() })
	if len(results) > 200 {
		results = results[:200]
		h.Incomplete = true
	}
	for _, file := range results {
		id := strings.TrimSuffix(file.Name(), ".json")
		if !ValidID(id) {
			continue
		}
		var result Result
		if r.ReadJSON("results/"+file.Name(), &result) != nil || result.Target != alias || result.ID != id || result.Version != Protocol || result.Updated.IsZero() {
			h.Incomplete = true
			continue
		}
		if result.Updated.Before(since) {
			continue
		}
		if result.Status == "failed" {
			h.Events = append(h.Events, lifecycle.Event{At: result.Updated, Target: alias, Kind: "operation-failed", Operation: id, Detail: result.Error, Source: "results/" + file.Name()})
		} else if result.DZSA != nil {
			if result.DZSA.CheckedAt.IsZero() {
				h.Incomplete = true
				continue
			}
			h.Events = append(h.Events, lifecycle.Event{At: result.DZSA.CheckedAt, Target: alias, Kind: "ready", Cause: result.Cause, Operation: id, Commit: result.Commit, Source: "results/" + file.Name()})
		}
	}
	directory := path.Join(remoteInstall(host), "profiles")
	logs, e := r.SFTP.ReadDir(directory)
	if e != nil {
		h.Incomplete = true
		h.Warnings = append(h.Warnings, "DayZ logs unavailable")
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].ModTime().After(logs[j].ModTime()) })
	examined := 0
	for _, file := range logs {
		if !strings.EqualFold(path.Ext(file.Name()), ".rpt") || !file.Mode().IsRegular() || file.ModTime().Before(since) {
			continue
		}
		examined++
		if examined > 24 {
			h.Incomplete = true
			break
		}
		f, e := r.SFTP.Open(path.Join(directory, file.Name()))
		if e != nil {
			h.Incomplete = true
			continue
		}
		start := file.Size() - 65536
		if start < 0 {
			start = 0
		}
		_, e = f.Seek(start, io.SeekStart)
		var b []byte
		if e == nil {
			b, e = io.ReadAll(io.LimitReader(f, 65536))
		}
		f.Close()
		if e != nil {
			h.Incomplete = true
			continue
		}
		if event, ok := LogExit(alias, file.Name(), file.ModTime(), string(b)); ok {
			h.Events = append(h.Events, event)
		}
	}
	sort.SliceStable(h.Events, func(i, j int) bool { return h.Events[i].At.Before(h.Events[j].At) })
	h.Cycles = correlateCycles(h.Events)
	return h
}

// LogExit uses explicit termination evidence, never just a file's age.
func LogExit(alias, name string, modified time.Time, tail string) (lifecycle.Event, bool) {
	if !strings.Contains(tail, "--- Termination successfully completed ---") {
		return lifecycle.Event{}, false
	}
	cause := "unknown"
	if strings.Contains(tail, "[Shutdown] Shutting down") && strings.Contains(tail, "[Shutdown] Saving players") {
		cause = "scheduled"
	}
	return lifecycle.Event{At: modified.UTC(), Target: alias, Kind: "clean-exit", Cause: cause, Source: name, Detail: "explicit successful termination marker; timestamp is log modification time"}, true
}

var ErrUnsupportedFeature = errors.New("installed agent requires an upgrade for this operation")

type Heartbeat struct {
	Version      int            `json:"version"`
	Updated      time.Time      `json:"updated"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Agent        *AgentIdentity `json:"agent,omitempty"`
}

func (r *Remote) RequireFeature(alias, feature string) error {
	if _, err := r.Ready(alias); err != nil {
		return err
	}
	var h Heartbeat
	if err := r.ReadJSON("heartbeat.json", &h); err != nil {
		return err
	}
	for _, v := range h.Capabilities {
		if v == feature {
			return nil
		}
	}
	return ErrUnsupportedFeature
}
