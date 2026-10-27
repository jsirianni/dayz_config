// Package logreader bounds and redacts operator-visible log excerpts.
package logreader

import (
	"errors"
	"io"
	"regexp"
	"strings"
)

const MaxBytes = 2 << 20

var sensitive = regexp.MustCompile(`(?i)password|passwd|\brcon\b|rconpassword|secret|\btoken\b|authorization|steam.?guard|private.key|two.?factor|verification.code|\+login\b`)
var keyBody = regexp.MustCompile(`^[A-Za-z0-9+/]{32,}={0,2}$`)

func CleanText(text string) string {
	lines := strings.Split(text, "\n")
	upperText := strings.ToUpper(text)
	firstEnd := strings.Index(upperText, "-----END ")
	firstBegin := strings.Index(upperText, "-----BEGIN ")
	private := firstEnd >= 0 && (firstBegin < 0 || firstEnd < firstBegin)
	for i, line := range lines {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "-----BEGIN ") && strings.Contains(upper, "PRIVATE KEY-----") {
			private = true
		}
		hide := private || sensitive.MatchString(line) || keyBody.MatchString(strings.TrimSpace(line)) || len(line) > 16384
		if strings.Contains(upper, "-----END ") && strings.Contains(upper, "PRIVATE KEY-----") {
			private = false
		}
		if hide {
			lines[i] = "[REDACTED sensitive or oversized log line]"
		}
	}
	return strings.Join(lines, "\n")
}

type Excerpt struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
	BytesRead int    `json:"bytesRead"`
}

func Tail(r io.ReadSeeker, size int64, lines int, completeOnly bool) (Excerpt, error) {
	var out Excerpt
	if size < 0 || lines < 1 || lines > 10000 {
		return out, errors.New("tail requires 1..10000 lines and a nonnegative file size")
	}
	start := size - MaxBytes
	if start < 0 {
		start = 0
	}
	out.Truncated = start > 0
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return out, err
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes))
	if err != nil {
		return out, err
	}
	out.BytesRead = len(data)
	text := string(data)
	if start > 0 {
		if n := strings.IndexByte(text, '\n'); n >= 0 {
			text = text[n+1:]
		} else {
			text = ""
		}
	}
	if completeOnly {
		if n := strings.LastIndexByte(text, '\n'); n >= 0 {
			text = text[:n+1]
		} else {
			text = ""
		}
	}
	parts := strings.SplitAfter(text, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
		out.Truncated = true
	}
	out.Text = CleanText(strings.Join(parts, ""))
	return out, nil
}

func MergeIssues(groups []Issue) []Issue {
	var merged []Issue
	index := map[string]int{}
	for _, issue := range groups {
		key := issue.Session + "\x00" + issue.Message
		if at, ok := index[key]; ok {
			merged[at].Count += issue.Count
			for _, source := range issue.Sources {
				found := false
				for _, old := range merged[at].Sources {
					found = found || old == source
				}
				if !found {
					merged[at].Sources = append(merged[at].Sources, source)
				}
			}
		} else if len(merged) < 512 {
			index[key] = len(merged)
			merged = append(merged, issue)
		}
	}
	return merged
}

type Issue struct {
	Message string   `json:"message"`
	Count   int      `json:"count"`
	Sources []string `json:"sources"`
	Session string   `json:"session"`
}

var clockPrefix = regexp.MustCompile(`^\s*(?:\d{1,2}:\d{2}:\d{2}(?:[.:]\d+)?|\d{4}-\d\d-\d\d[T ][^ ]+)\s*`)

func Summarize(text, source, session string) []Issue {
	var issues []Issue
	index := map[string]int{}
	for _, line := range strings.Split(CleanText(text), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "exception") && !strings.Contains(lower, "warning") && !strings.HasPrefix(lower, "reason:") {
			continue
		}
		message := clockPrefix.ReplaceAllString(strings.TrimSpace(line), "")
		if len(message) > 2048 {
			message = message[:2048] + " [truncated]"
		}
		if at, ok := index[message]; ok {
			issues[at].Count++
		} else {
			if len(issues) >= 256 {
				continue
			}
			index[message] = len(issues)
			issues = append(issues, Issue{Message: message, Count: 1, Sources: []string{source}, Session: session})
		}
	}
	return issues
}
