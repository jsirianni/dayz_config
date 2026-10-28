package deploy

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentUpgradeReattachmentResumesOnlyUnstartedSubmission(t *testing.T) {
	for _, phase := range []string{"submitted", "starting", "complete"} {
		t.Run(phase, func(t *testing.T) {
			id := NewID()
			var calls atomic.Int32
			endpoint, root := sshExecFixture(t, func(command string, _ io.Reader, out io.Writer) error {
				if calls.Add(1) == 1 {
					status := "running"
					if phase == "complete" {
						status = "succeeded"
					}
					return json.NewEncoder(out).Encode(UpgradeResult{Version: Protocol, ID: id, Target: "nasdara-staging", Status: status, Phase: phase})
				}
				source := decodeUpgradeCollector(t, command)
				parseUpgradeCollector(t, source)
				if !strings.Contains(source, id) || !strings.Contains(source, "Get-FileHash") || !strings.Contains(source, "agent-upgrade-submit --request $request") {
					t.Error("resume did not bind protected coordinator to existing operation")
				}
				return nil
			})
			for _, name := range []string{"state.json", "heartbeat.json"} {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
			remote, err := Connect(context.Background(), endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer remote.Close()
			if err = remote.ResumeAgentUpgradeSubmission(context.Background(), "nasdara-staging", id); err != nil {
				t.Fatal(err)
			}
			want := int32(1)
			if phase == "submitted" {
				want = 2
			}
			if calls.Load() != want {
				t.Fatalf("%s executed %d commands, expected %d", phase, calls.Load(), want)
			}
			if entries, e := os.ReadDir(root); e != nil || len(entries) != 0 {
				t.Fatal("reattachment uploaded or rewrote files", entries, e)
			}
		})
	}
}
