package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/logreader"
)

type LogSource struct {
	Kind     string    `json:"kind"`
	Name     string    `json:"name"`
	Path     string    `json:"-"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}
type LogExcerpt struct {
	Source  LogSource `json:"source"`
	Session string    `json:"session"`
	logreader.Excerpt
	Error string `json:"error,omitempty"`
}

func ValidLogKind(kind string) bool {
	switch kind {
	case "console", "updates", "agent", "rpt", "script", "crash":
		return true
	}
	return false
}

func (r *Remote) LogSources(host HostConfig, kind string) ([]LogSource, error) {
	if !ValidLogKind(kind) {
		return nil, errors.New("unrecognized log kind")
	}
	directory := r.remote("logs")
	fixed := ""
	switch kind {
	case "console":
		fixed = "server-console.log"
	case "updates":
		fixed = "updates.log"
	case "rpt", "script", "crash":
		directory = path.Join(remoteInstall(host), "profiles")
	}
	if err := r.rejectRemoteLinks(directory); err != nil {
		return nil, err
	}
	entries, err := r.SFTP.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var sources []LogSource
	for _, file := range entries {
		name := file.Name()
		lower := strings.ToLower(name)
		if !file.Mode().IsRegular() || SafeRelative(name) != nil || path.Base(name) != name {
			continue
		}
		match := name == fixed && fixed != ""
		switch kind {
		case "agent":
			match = lower == "agent-error.txt" || lower == "lifecycle-error.txt"
		case "rpt":
			match = strings.HasSuffix(lower, ".rpt")
		case "script":
			match = strings.HasPrefix(lower, "script_") && strings.HasSuffix(lower, ".log")
		case "crash":
			match = strings.HasPrefix(lower, "crash_") && strings.HasSuffix(lower, ".log")
		}
		if match {
			sources = append(sources, LogSource{Kind: kind, Name: name, Path: path.Join(directory, name), Modified: file.ModTime().UTC(), Size: file.Size()})
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Modified.Equal(sources[j].Modified) {
			return sources[i].Name > sources[j].Name
		}
		return sources[i].Modified.After(sources[j].Modified)
	})
	return sources, nil
}

func (r *Remote) ReadLog(source LogSource, lines int, completeOnly bool) (LogExcerpt, error) {
	out := LogExcerpt{Source: source, Session: "unknown"}
	info, err := r.SFTP.Lstat(source.Path)
	if err != nil {
		return out, err
	}
	if !info.Mode().IsRegular() {
		return out, errors.New("log is no longer a regular file")
	}
	f, err := r.SFTP.Open(source.Path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	out.Excerpt, err = logreader.Tail(f, info.Size(), lines, completeOnly)
	return out, err
}

func (r *Remote) Logs(ctx context.Context, host HostConfig, kind string, lines int, follow bool, w io.Writer) error {
	var lastPath, lastText string
	var lastSize int64
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		sources, err := r.LogSources(host, kind)
		if err != nil {
			return err
		}
		if len(sources) == 0 {
			return errors.New("no matching log source")
		}
		source := sources[0]
		out, err := r.ReadLog(source, lines, follow)
		if err != nil {
			return err
		}
		text := out.Text
		if lastPath == source.Path && source.Size >= lastSize {
			// Compare complete redacted lines, not raw byte slices that could expose a
			// credential split across reads. The excerpt window may rotate as it grows.
			var moved bool
			text, moved = logreader.Delta(lastText, text)
			if moved {
				fmt.Fprintln(w, "\n--- log rotated or bounded excerpt window moved ---")
			}
		} else if lastPath != "" {
			fmt.Fprintf(w, "\n--- log rotated: %s ---\n", source.Name)
		}
		if _, err = io.WriteString(w, text); err != nil {
			return err
		}
		lastPath = source.Path
		lastSize = source.Size
		lastText = out.Text
		if !follow {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
