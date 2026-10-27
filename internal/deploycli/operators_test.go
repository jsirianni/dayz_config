package deploycli

import "testing"

func TestOperatorHelpIsOffline(t *testing.T) {
	for _, args := range [][]string{{"health", "--help"}, {"history", "--help"}, {"logs", "--help"}, {"diagnose", "--help"}, {"backup", "--help"}, {"backup", "create", "--help"}} {
		if err := run(args); err != nil {
			t.Fatal(args, err)
		}
	}
}

func TestOperatorInvalidInputsRejectedBeforeConnecting(t *testing.T) {
	cases := [][]string{
		{"health", "all", "--since", "0"}, {"history", "all", "--since", "-1h"},
		{"logs", "all"}, {"logs", "nasdara", "--tail", "0"}, {"logs", "nasdara", "--kind", "credentials"}, {"diagnose", "all", "--follow"},
		{"status", "nasdara", "--hard"}, {"deploy", "nasdara", "--hard"}, {"rollback", "nasdara", "--hard"},
		{"backup", "create", "all"}, {"backup", "create", "nasdara", "--hard"}, {"backup", "restore", "nasdara", "--backup", "../../bad"},
		{"backup", "list", "nasdara", "--stop"}, {"backup", "create", "nasdara", "--dry-run", "--no-wait"},
		{"backup", "download", "nasdara", "--backup", "20261027T000000-0000000000000000"},
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Fatal("accepted invalid args", args)
		}
	}
}
