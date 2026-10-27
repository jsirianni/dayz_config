package deploycli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
	"github.com/jsirianni/dayz_config/internal/hostinspect"
)

func TestHealthTextShowsObservedValuesAndUnknownCoverage(t *testing.T) {
	var out bytes.Buffer
	now := time.Now().UTC()
	r := deploy.HealthReport{Target: "nasdara", Status: "running", Installed: "installed", Startup: "started", History: deploy.HistoryReport{Incomplete: true}, Snapshot: &hostinspect.Snapshot{At: now, Processes: []hostinspect.Process{{Started: now.Add(-2 * time.Hour)}}}, DZSA: &deploy.DZSAReport{Server: deploy.DZSAServer{Players: 1, MaxPlayers: 60}}}
	if err := renderHealth(&out, []deploy.HealthReport{r}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"2h0m0s", "1/60", "unknown", "Installed: installed", "Last managed startup: started"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("missing %q in %s", expected, out.String())
		}
	}
}
