package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/hostinspect"
	"github.com/jsirianni/dayz_config/internal/lifecycle"
)

func TestPartialDiagnosticsRedactOnOutputFailureAndCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "output directory is a file", true: "canceled collection"}[canceled], func(t *testing.T) {
			endpoint, root := sshExecFixture(t, func(_ string, _ io.Reader, w io.Writer) error {
				return json.NewEncoder(w).Encode(hostinspect.Snapshot{At: time.Now().UTC(), Query: hostinspect.Query{Error: "fixture offline"}})
			})
			host := testConfig(t)
			startup := strings.Repeat("b", 40)
			state := State{Version: Protocol, Target: host.Target, RunningCommit: startup, Error: "password=review-sensitive-value"}
			if err := WriteJSON(filepath.Join(root, "host.json"), host); err != nil {
				t.Fatal(err)
			}
			if err := WriteJSON(filepath.Join(root, "state.json"), state); err != nil {
				t.Fatal(err)
			}
			if err := lifecycle.Write(root, lifecycle.Event{Target: host.Target, Kind: "failure", Detail: "password=history-sensitive-value"}); err != nil {
				t.Fatal(err)
			}
			writeTest(t, root, "logs/server-console.log", "password=log-sensitive-value\n")
			directory := filepath.Join(t.TempDir(), "already-a-file")
			if err := os.WriteFile(directory, []byte("existing file"), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := Connect(context.Background(), endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			out, err := r.Diagnose(ctx, host.Target, time.Now().Add(-time.Hour), directory)
			if err == nil || canceled && !errors.Is(err, context.Canceled) {
				t.Fatal("expected collection failure", err)
			}
			if out.Health.Startup != startup || len(out.Health.History.Events) != 1 || out.Bundle != "" {
				t.Fatal("partial evidence or revision attribution lost", out)
			}
			// Marshal exactly as the CLI does; do not sanitize again in the test.
			b, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "sensitive-value") || !strings.Contains(string(b), "REDACTED") {
				t.Fatalf("partial diagnostics leaked collected evidence: %s", b)
			}
		})
	}
}
