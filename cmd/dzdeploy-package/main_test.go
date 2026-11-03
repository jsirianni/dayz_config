package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func TestClientDistributionPlatforms(t *testing.T) {
	for _, platform := range []string{"windows", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if !supportedClient(platform, arch) {
				t.Fatalf("supported client rejected: %s/%s", platform, arch)
			}
		}
	}
	for _, platform := range []string{"darwin", "freebsd", "unsupported"} {
		if supportedClient(platform, "amd64") {
			t.Fatalf("unsupported client accepted: %s", platform)
		}
	}
	if supportedClient("windows", "386") {
		t.Fatal("unsupported architecture accepted")
	}
}

func TestPrebuiltPortablePackage(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for name, content := range map[string]string{
		"client": "client bytes", "agent": "Windows companion bytes",
		"scripts/bootstrap-windows.ps1": "public bootstrap", "deploy.example.yaml": "example",
		"docs/deployment.md": "runbook", "docs/operator-workflows.md": "operations",
		"deploy.yaml": "private configuration must never be packaged",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"", "1.2.3"} {
		for _, platform := range []string{"windows", "linux"} {
			output := t.TempDir()
			if err := run([]string{"--os", platform, "--arch", "arm64", "--version", version,
				"--output", output, "--client-binary", "client", "--agent-binary", "agent"}); err != nil {
				t.Fatal(err)
			}
			prefix := "dzdeploy_"
			if version != "" {
				prefix += version + "_"
			}
			archive, err := zip.OpenReader(filepath.Join(output, prefix+platform+"_arm64.zip"))
			if err != nil {
				t.Fatal(err)
			}
			client := "dzdeploy"
			if platform == "windows" {
				client += ".exe"
			}
			expected := map[string]bool{client: true, "agents/windows-amd64/dzdeploy.exe": true,
				"bootstrap-windows.ps1": true, "deploy.example.yaml": true, "deployment.md": true,
				"operator-workflows.md": true, "checksums.json": true}
			actual := map[string][]byte{}
			for _, entry := range archive.File {
				if !expected[entry.Name] {
					t.Fatalf("unexpected archive entry: %s", entry.Name)
				}
				if entry.Name == client && entry.Mode().Perm() != 0755 {
					t.Fatal("client lost executable permissions")
				}
				reader, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				actual[entry.Name] = data
			}
			archive.Close()
			if len(actual) != len(expected) || string(actual[client]) != "client bytes" || string(actual["agents/windows-amd64/dzdeploy.exe"]) != "Windows companion bytes" {
				t.Fatal("package inventory or prebuilt binary bytes changed")
			}
			var manifest struct {
				Protocol int               `json:"protocol"`
				SHA256   map[string]string `json:"sha256"`
			}
			if err := json.Unmarshal(actual["checksums.json"], &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Protocol != deploy.Protocol || len(manifest.SHA256) != len(actual)-1 {
				t.Fatal("invalid package manifest")
			}
			for name, checksum := range manifest.SHA256 {
				if deploy.Digest(actual[name]) != checksum {
					t.Fatalf("invalid checksum: %s", name)
				}
			}
		}
	}
}

func TestRejectInvalidPackageOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--client-binary", "client"}, {"--agent-binary", "agent"},
		{"--version", "../../secret"}, {"--os", "darwin"}, {"unexpected"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("accepted invalid options: %v", args)
		}
	}
}
