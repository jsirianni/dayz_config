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
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	flags := flag.NewFlagSet("dzdeploy-package", flag.ContinueOnError)
	platform := flags.String("os", runtime.GOOS, "client OS")
	arch := flags.String("arch", runtime.GOARCH, "client architecture")
	output := flags.String("output", ".tools/packages", "output directory")
	version := flags.String("version", "", "optional archive version")
	clientBinary := flags.String("client-binary", "", "prebuilt client (requires --agent-binary)")
	agentBinary := flags.String("agent-binary", "", "prebuilt Windows amd64 companion (requires --client-binary)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if !supportedClient(*platform, *arch) {
		return fmt.Errorf("unsupported client platform")
	}
	if (*clientBinary == "") != (*agentBinary == "") {
		return fmt.Errorf("--client-binary and --agent-binary must be supplied together")
	}
	if *version != "" && !regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]*$`).MatchString(*version) {
		return fmt.Errorf("invalid archive version")
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
	companion := filepath.Join(stage, "agent.exe")
	if *clientBinary != "" {
		client, companion = *clientBinary, *agentBinary
	} else {
		if err = build(*platform, *arch, client); err != nil {
			return err
		}
		if err = build("windows", "amd64", companion); err != nil {
			return err
		}
	}
	files := map[string]string{clientName: client, "agents/windows-amd64/dzdeploy.exe": companion, "bootstrap-windows.ps1": "scripts/bootstrap-windows.ps1", "deploy.example.yaml": "deploy.example.yaml", "deployment.md": "docs/deployment.md", "operator-workflows.md": "docs/operator-workflows.md"}
	prefix := "dzdeploy_"
	if *version != "" {
		prefix += *version + "_"
	}
	archive := filepath.Join(*output, prefix+*platform+"_"+*arch+".zip")
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	defer z.Close()
	checksums := map[string]string{}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		source := files[name]
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
