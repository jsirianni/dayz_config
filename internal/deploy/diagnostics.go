package deploy

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/logreader"
)

type Diagnostics struct {
	Target   string            `json:"target"`
	Bundle   string            `json:"bundle,omitempty"`
	Health   HealthReport      `json:"health"`
	Logs     []LogExcerpt      `json:"logs"`
	Issues   []logreader.Issue `json:"issues"`
	Warnings []string          `json:"warnings,omitempty"`
}

func sanitizeValue(v any) any {
	switch value := v.(type) {
	case string:
		return logreader.CleanText(value)
	case []any:
		for i := range value {
			value[i] = sanitizeValue(value[i])
		}
	case map[string]any:
		for k, v := range value {
			value[k] = sanitizeValue(v)
		}
	}
	return v
}
func sanitizedJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var values any
	if err = json.Unmarshal(b, &values); err != nil {
		return nil, err
	}
	return json.MarshalIndent(sanitizeValue(values), "", "  ")
}

func (r *Remote) Diagnose(ctx context.Context, alias string, since time.Time, directory string) (Diagnostics, error) {
	out := Diagnostics{Target: alias, Health: r.Health(ctx, alias, since)}
	var host HostConfig
	if err := r.ReadJSON("host.json", &host); err != nil {
		return out, err
	}
	if err := host.Validate(); err != nil {
		return out, err
	}
	if host.Target != alias {
		return out, errors.New("host target mismatch")
	}
	stamp := ""
	if out.Health.Snapshot != nil && len(out.Health.Snapshot.Processes) == 1 {
		stamp = out.Health.Snapshot.Processes[0].SessionStamp
	}
	total := 0
	for _, kind := range []string{"console", "updates", "agent", "rpt", "script", "crash"} {
		sources, err := r.LogSources(host, kind)
		if err != nil {
			out.Warnings = append(out.Warnings, kind+": source unavailable")
			continue
		}
		if len(sources) == 0 {
			out.Warnings = append(out.Warnings, kind+": no matching log")
		}
		if len(sources) > 3 {
			sources = sources[:3]
			out.Warnings = append(out.Warnings, kind+": limited to latest three files")
		}
		for _, source := range sources {
			if source.Modified.Before(since) {
				continue
			}
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			if total >= 25<<20 {
				out.Warnings = append(out.Warnings, "25 MiB excerpt limit reached")
				break
			}
			snippet, e := r.ReadLog(source, 2000, false)
			if e != nil {
				snippet.Error = logreader.CleanText(e.Error())
			}
			snippet.Session = "unknown"
			if kind == "console" || kind == "updates" || kind == "agent" {
				snippet.Session = "mixed"
			} else if stamp != "" {
				snippet.Session = "historical"
				if strings.Contains(source.Name, stamp) {
					snippet.Session = "current"
				}
			}
			if total+len(snippet.Text) > 25<<20 {
				snippet.Text = ""
				snippet.Truncated = true
				out.Warnings = append(out.Warnings, "excerpt skipped at total byte limit")
			}
			total += len(snippet.Text)
			out.Issues = append(out.Issues, logreader.Summarize(snippet.Text, source.Kind+"/"+source.Name, snippet.Session)...)
			out.Logs = append(out.Logs, snippet)
		}
	}
	out.Issues = logreader.MergeIssues(out.Issues)
	if len(out.Issues) >= 512 {
		out.Warnings = append(out.Warnings, "issue summary limited to 512 groups")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return out, err
	}
	f, err := os.CreateTemp(directory, ".diagnostic-*.zip")
	if err != nil {
		return out, err
	}
	defer os.Remove(f.Name())
	z := zip.NewWriter(f)
	add := func(name string, b []byte) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write(b)
		return e
	}
	index := out
	index.Logs = append([]LogExcerpt{}, out.Logs...)
	for i := range index.Logs {
		index.Logs[i].Text = ""
	}
	metadata, err := sanitizedJSON(index)
	if err == nil {
		err = add("report.json", metadata)
	}
	for _, snippet := range out.Logs {
		if err != nil {
			break
		}
		err = add("logs/"+snippet.Source.Kind+"/"+snippet.Source.Name+".txt", []byte(snippet.Text))
	}
	err = errors.Join(err, z.Close(), f.Sync(), f.Close())
	if err != nil {
		return out, err
	}
	dest := filepath.Join(directory, fmt.Sprintf("%s-%s.zip", alias, time.Now().UTC().Format("20060102T150405.000000000Z")))
	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		return out, errors.New("diagnostic destination already exists or cannot be inspected")
	}
	if err = os.Rename(f.Name(), dest); err != nil {
		return out, err
	}
	out.Bundle = dest
	// Returned operator summaries are redacted too, not just the archive.
	b, err := sanitizedJSON(out)
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	return out, err
}
