package main

import (
	"testing"
)

func TestRejectsAmbiguousCLIInput(t *testing.T) {
	for _, args := range [][]string{{"deploy"}, {"deploy", "unknown"}, {"deploy", "nasdara", "--offline"}, {"restart", "nasdara", "--dry-run"}, {"status", "nasdara", "--timeout", "0"}, {"agent"}, {"tools", "unknown"}, {"rollback", "nasdara", "--deployment", "../../bad"}} {
		if err := run(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
