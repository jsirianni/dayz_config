package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLTemplateAndRelativeKeyPaths(t *testing.T) {
	source := filepath.Join("..", "..", "deploy.example.yaml")
	c, err := LoadClient(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Targets) != 3 {
		t.Fatal("template missing servers")
	}
	for alias, e := range c.Targets {
		if !filepath.IsAbs(e.KnownHosts) || !filepath.IsAbs(e.KeyFile) || !filepath.IsAbs(e.PublicKeyFile) {
			t.Fatal("SSH paths must resolve relative to deploy.yaml")
		}
		if e.Host == nil || e.Host.Version != Protocol || e.Host.Target != alias {
			t.Fatal("host identity not derived from YAML target key")
		}
	}
}
func TestYAMLRejectsUnknownDuplicateAndMultipleDocuments(t *testing.T) {
	for _, data := range []string{"version: 1\ntargets: {}\nsecret_password: no\n", "version: 1\nversion: 1\ntargets: {}\n", "version: 1\ntargets: {}\n---\nversion: 1\n", "version: 99\ntargets: {}\n", "version: 1\ntargets:\n  unknown: {}\n"} {
		p := filepath.Join(t.TempDir(), "deploy.yaml")
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadClient(p); err == nil {
			t.Fatalf("invalid YAML accepted: %s", data)
		}
	}
}
