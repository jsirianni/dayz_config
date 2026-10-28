package deploycli

import "testing"

func TestAgentUpgradeHelpAndInvalidArgumentsAreOffline(t *testing.T) {
	for _, args := range [][]string{{"agent", "upgrade", "--help"}, {"agent", "upgrade", "nasdara", "--help"}} {
		if err := run(args); err != nil {
			t.Fatal(args, err)
		}
	}
	for _, args := range [][]string{{"agent", "upgrade"}, {"agent", "upgrade", "all"}, {"agent", "upgrade", "nasdara", "--restart"}, {"agent", "upgrade", "nasdara", "--hard"}, {"agent", "upgrade", "nasdara", "--timeout", "0"}, {"agent", "upgrade", "nasdara", "--operation", "../../bad"}, {"agent", "upgrade", "nasdara", "--dry-run", "--no-wait"}, {"agent", "upgrade", "nasdara", "--dry-run", "--operation", "20261027T000000-0000000000000000"}, {"agent-upgrade-submit"}, {"agent-upgrade-coordinator", "--request", "file", "extra"}} {
		if err := run(args); err == nil {
			t.Fatal("accepted invalid upgrade invocation", args)
		}
	}
}
