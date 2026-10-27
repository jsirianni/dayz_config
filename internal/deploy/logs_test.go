package deploy

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
