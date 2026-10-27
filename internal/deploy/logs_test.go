package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/lifecycle"
	"github.com/jsirianni/dayz_config/internal/logreader"
)

func TestLogAllowlistRedactionAndNoRemoteWrites(t *testing.T) {
	endpoint, root := sshFixture(t)
	host := testConfig(t)
	writeTest(t, root, "logs/server-console.log", "ordinary log line\npassword = supersecret\n")
	writeTest(t, root, "logs/credentials.txt", "not permitted")
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	sources, err := r.LogSources(host, "console")
	if err != nil || len(sources) != 1 {
		t.Fatal(sources, err)
	}
	out, err := r.ReadLog(sources[0], 200, false)
	if err != nil || strings.Contains(out.Text, "supersecret") || !strings.Contains(out.Text, "ordinary log line") {
		t.Fatal(out, err)
	}
	if _, err = r.LogSources(host, "credentials"); err == nil {
		t.Fatal("arbitrary source accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "inbox")); !os.IsNotExist(err) {
		t.Fatal("read wrote requests")
	}
}

func TestFollowAppendsOnceRotatesAndCancels(t *testing.T) {
	endpoint, root := sshFixture(t)
	host := testConfig(t)
	writeTest(t, root, "logs/server-console.log", "initial line\n")
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	// The writer only runs on this goroutine; updates/cancellation run separately.
	go func() {
		time.Sleep(200 * time.Millisecond)
		f, e := os.OpenFile(filepath.Join(root, "logs/server-console.log"), os.O_APPEND|os.O_WRONLY, 0600)
		if e == nil {
			_, _ = f.WriteString("password=hidden\nappended line\n")
			f.Close()
		}
		time.Sleep(1200 * time.Millisecond)
		cancel()
	}()
	if err = r.Logs(ctx, host, "console", 200, true, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "initial line") != 1 || strings.Count(out.String(), "appended line") != 1 || strings.Contains(out.String(), "hidden") {
		t.Fatal(out.String())
	}
}

func TestSanitizedReportsKeepJSONAndRemoveSecrets(t *testing.T) {
	b, err := sanitizedJSON(map[string]any{"errors": []string{"password=hidden", "ordinary"}, "pid": 42})
	if err != nil || strings.Contains(string(b), "hidden") || !strings.Contains(string(b), "ordinary") {
		t.Fatal(string(b), err)
	}
}

func TestDiagnosticRevisionFieldsSurviveWithoutExemptingLogText(t *testing.T) {
	installed, startup, event := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	report := HealthReport{Installed: installed, Startup: startup,
		History: HistoryReport{Events: []lifecycle.Event{{Commit: event, Detail: "password=event-secret"}}},
		Errors:  []string{"password=report-secret", installed},
	}
	b, err := sanitizedJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	var actual HealthReport
	if err = json.Unmarshal(b, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Installed != installed || actual.Startup != startup || actual.History.Events[0].Commit != event {
		t.Fatalf("revision identity lost: %s", b)
	}
	if strings.Contains(string(b), "event-secret") || strings.Contains(string(b), "report-secret") || strings.Contains(actual.Errors[1], installed) {
		t.Fatalf("free-form text bypassed redaction: %s", b)
	}
	b, err = sanitizedJSON(map[string]string{"commit": "password=invalid-revision", "installedRevision": strings.Repeat("Z", 40)})
	if err != nil || strings.Contains(string(b), "invalid-revision") || strings.Contains(string(b), strings.Repeat("Z", 40)) {
		t.Fatal(string(b), err)
	}
}

type logWriteFunc func([]byte) (int, error)

func (f logWriteFunc) Write(p []byte) (int, error) { return f(p) }

func TestSSHFollowTailOneEmitsRepeatedAndRedactionEquivalentOccurrences(t *testing.T) {
	for _, tt := range []struct{ first, append, expected string }{
		{"repeated error\n", "repeated error\n", "repeated error\nrepeated error\n"},
		{"password=first\n", "password=second\n", "[REDACTED sensitive or oversized log line]\n[REDACTED sensitive or oversized log line]\n"},
	} {
		t.Run(tt.first, func(t *testing.T) {
			endpoint, root := sshFixture(t)
			writeTest(t, root, "logs/server-console.log", tt.first)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			r, err := Connect(ctx, endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			var out bytes.Buffer
			writes := 0
			writer := logWriteFunc(func(p []byte) (int, error) {
				out.Write(p)
				writes++
				if writes == 1 {
					f, e := os.OpenFile(filepath.Join(root, "logs/server-console.log"), os.O_APPEND|os.O_WRONLY, 0600)
					if e != nil {
						return 0, e
					}
					_, e = f.WriteString(tt.append)
					closeErr := f.Close()
					if e != nil {
						return 0, e
					}
					if closeErr != nil {
						return 0, closeErr
					}
				} else {
					cancel()
				}
				return len(p), nil
			})
			if err = r.Logs(ctx, testConfig(t), "console", 1, true, writer); err != nil {
				t.Fatal(err)
			}
			if out.String() != tt.expected {
				t.Fatalf("appended occurrence lost: %q", out.String())
			}
		})
	}
}

func TestSSHFollowCursorHandlesReplacedFileAndPartialCredential(t *testing.T) {
	endpoint, root := sshFixture(t)
	writeTest(t, root, "logs/server-console.log", "old safe\npass")
	r, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var cursor logreader.Follower
	read := func() logreader.FollowExcerpt {
		t.Helper()
		sources, e := r.LogSources(testConfig(t), "console")
		if e != nil || len(sources) != 1 {
			t.Fatal(sources, e)
		}
		out, e := r.followLog(sources[0], &cursor, 1)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	if out := read(); out.Text != "old safe\n" {
		t.Fatal(out)
	}
	writeTest(t, root, "logs/server-console.log", "old safe\npassword=split-secret\n")
	if out := read(); strings.Contains(out.Text, "split-secret") || strings.Count(out.Text, "REDACTED") != 1 {
		t.Fatal(out)
	}
	name := filepath.Join(root, "logs/server-console.log")
	if err := os.Rename(name, name+".old"); err != nil {
		t.Fatal("follow retained a handle that blocked rotation", err)
	}
	writeTest(t, root, "logs/server-console.log", "replacement safe\n")
	if out := read(); !out.Rotated || out.Text != "replacement safe\n" {
		t.Fatal(out)
	}
}
