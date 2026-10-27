// dzdeploy-package builds a portable client distribution including its Windows
// host companion. Go is needed by the builder, never by the remote server.
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	platform := flag.String("os", runtime.GOOS, "client OS")
	arch := flag.String("arch", runtime.GOARCH, "client architecture")
	output := flag.String("output", ".tools/packages", "output directory")
	flag.Parse()
	if !supportedClient(*platform, *arch) {
		return fmt.Errorf("unsupported client platform")
	}
	if err := os.MkdirAll(*output, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(*output, "build-")
	if err != nil {
		return err
	}
	outputRoot, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	stage, err = filepath.Abs(stage)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(outputRoot, stage)
	if err != nil || !strings.HasPrefix(rel, "build-") || strings.ContainsAny(rel, `/\`) {
		return fmt.Errorf("unsafe package staging directory")
	}
	defer os.RemoveAll(stage)
	build := func(platform, arch, dest string) error {
		cmd := exec.Command("go", "build", "-buildvcs=false", "-trimpath", "-o", dest, "./cmd/dzdeploy")
		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "GOOS=") && !strings.HasPrefix(env, "GOARCH=") && !strings.HasPrefix(env, "CGO_ENABLED=") {
				cmd.Env = append(cmd.Env, env)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS="+platform, "GOARCH="+arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	clientName := "dzdeploy"
	if *platform == "windows" {
		clientName += ".exe"
	}
	client := filepath.Join(stage, clientName)
	if err = build(*platform, *arch, client); err != nil {
		return err
	}
	companion := filepath.Join(stage, "agent.exe")
	if err = build("windows", "amd64", companion); err != nil {
		return err
	}
	files := map[string]string{clientName: client, "agents/windows-amd64/dzdeploy.exe": companion, "bootstrap-windows.ps1": "scripts/bootstrap-windows.ps1", "deploy.example.yaml": "deploy.example.yaml", "deployment.md": "docs/deployment.md", "operator-workflows.md": "docs/operator-workflows.md"}
	archive := filepath.Join(*output, "dzdeploy_"+*platform+"_"+*arch+".zip")
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	checksums := map[string]string{}
	for name, source := range files {
		raw, err := os.ReadFile(source)
		if err != nil {
			z.Close()
			return err
		}
		checksums[name] = deploy.Digest(raw)
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0644)
		if name == clientName || strings.HasSuffix(name, ".exe") {
			h.SetMode(0755)
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err = w.Write(raw); err != nil {
			return err
		}
	}
	manifest, err := json.MarshalIndent(map[string]any{"protocol": deploy.Protocol, "sha256": checksums}, "", "  ")
	if err != nil {
		return err
	}
	w, err := z.Create("checksums.json")
	if err != nil {
		return err
	}
	if _, err = w.Write(manifest); err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	fmt.Println(archive)
	return nil
}

func supportedClient(platform, arch string) bool {
	return (platform == "windows" || platform == "linux") && (arch == "amd64" || arch == "arm64")
}
