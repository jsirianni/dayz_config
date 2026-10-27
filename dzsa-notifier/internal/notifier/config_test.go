package notifier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := values[k]; return v, ok }
}

func configFile(t *testing.T, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigurationPrecedence(t *testing.T) {
	p := configFile(t, "interval: 2m\nrequest_timeout: 3s\nendpoints: [203.0.113.10:27016]\n")
	c, err := Load(p, true, env(map[string]string{"DZSA_NOTIFIER_INTERVAL": "7m", "DZSA_NOTIFIER_ENDPOINTS": "203.0.113.11:27018, 203.0.113.11:27019", "DZSA_NOTIFIER_METRICS_LISTEN": ":9091"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Interval != 7*time.Minute || c.RequestTimeout != 3*time.Second || c.MetricsListen != ":9091" || len(c.Endpoints) != 2 || c.Endpoints[1] != "203.0.113.11:27019" {
		t.Fatalf("wrong precedence: %+v", c)
	}
}

func TestMissingConfiguration(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing")
	values := env(map[string]string{"DZSA_NOTIFIER_ENDPOINTS": "203.0.113.10:27016"})
	c, err := Load(p, false, values)
	if err != nil || c.Interval != 5*time.Minute || c.RequestTimeout != 10*time.Second {
		t.Fatalf("environment-only defaults: %+v %v", c, err)
	}
	if _, err := Load(p, true, values); err == nil {
		t.Fatal("explicit missing path accepted")
	}
	if _, err := Load(p, false, env(nil)); err == nil {
		t.Fatal("missing file without endpoints accepted")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for name, input := range map[string]string{
		"unknown":          "ticker: 5m\nendpoints: [203.0.113.10:27016]",
		"multiple":         "endpoints: [203.0.113.10:27016]\n---\ninterval: 5m",
		"duplicate":        "endpoints: [203.0.113.10:27016, 203.0.113.10:27016]",
		"private":          "endpoints: [192.168.1.196:27016]",
		"loopback":         "endpoints: [127.0.0.1:27016]",
		"ipv6":             "endpoints: ['[2001:db8::1]:27016']",
		"dns":              "endpoints: [example.com:27016]",
		"zero port":        "endpoints: [203.0.113.10:0]",
		"large port":       "endpoints: [203.0.113.10:65536]",
		"empty":            "endpoints: []",
		"zero duration":    "interval: 0s\nendpoints: [203.0.113.10:27016]",
		"negative timeout": "request_timeout: -1s\nendpoints: [203.0.113.10:27016]",
		"bad metrics":      "metrics_listen: :0\nendpoints: [203.0.113.10:27016]",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(configFile(t, input), true, env(nil)); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	for _, name := range []string{"INTERVAL", "REQUEST_TIMEOUT", "ENDPOINTS", "METRICS_LISTEN"} {
		values := map[string]string{"DZSA_NOTIFIER_ENDPOINTS": "203.0.113.10:27016", "DZSA_NOTIFIER_" + name: ""}
		if _, err := Load(filepath.Join(t.TempDir(), "missing"), false, env(values)); err == nil {
			t.Fatal("empty override accepted:", name)
		}
	}
}

func TestYAMLTypoNotHiddenByEnvironment(t *testing.T) {
	_, err := Load(configFile(t, "endponts: []"), true, env(map[string]string{"DZSA_NOTIFIER_ENDPOINTS": "203.0.113.10:27016"}))
	if err == nil || !strings.Contains(err.Error(), "endponts") {
		t.Fatal(err)
	}
}

func TestOversizedConfigIsNotTruncated(t *testing.T) {
	_, err := Load(configFile(t, "endpoints: [203.0.113.10:27016]\n"+strings.Repeat(" ", 1<<20)), true, env(nil))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal(err)
	}
}
