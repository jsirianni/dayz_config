package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func main() {
	target := flag.String("os", runtime.GOOS, "windows or linux")
	version := flag.String("version", "dev", "archive/binary version")
	flag.Parse()
	if e := run(*target, *version); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(target, version string) error {
	if target != "windows" && target != "linux" {
		return fmt.Errorf("unsupported OS %s", target)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`).MatchString(version) {
		return fmt.Errorf("invalid version")
	}
	mod, e := os.ReadFile("go.mod")
	if e != nil || !strings.Contains(strings.ReplaceAll(string(mod), "\r\n", "\n"), "module github.com/jsirianni/dayz_config/proxmox\n") {
		return fmt.Errorf("run packaging from the standalone proxmox module")
	}
	if e = os.MkdirAll("dist", 0755); e != nil {
		return e
	}
	tmp, e := os.MkdirTemp("dist", ".package-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	binary := "pvecli"
	if target == "windows" {
		binary += ".exe"
	}
	binaryPath := filepath.Join(tmp, binary)
	cmd := exec.Command("go", "build", "-buildvcs=false", "-trimpath", "-ldflags", "-X github.com/jsirianni/dayz_config/proxmox/internal/cli.Version="+version, "-o", binaryPath, "./cmd/pvecli")
	cmd.Env = append(os.Environ(), "GOOS="+target, "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		return e
	}
	files := map[string][]byte{}
	files[binary], e = os.ReadFile(binaryPath)
	if e != nil {
		return e
	}
	for _, dir := range []string{"examples", "docs"} {
		e = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			files[filepath.ToSlash(path)] = b
			return nil
		})
		if e != nil {
			return e
		}
	}
	files["README.md"], e = os.ReadFile("README.md")
	if e != nil {
		return e
	}
	archive := "pvecli-" + version + "-" + target + "-amd64.zip"
	path := filepath.Join("dist", archive)
	if e = writeArchive(path, files, binary); e != nil {
		return e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(b)
	if e = os.WriteFile(path+".sha256", []byte(hex.EncodeToString(sum[:])+"  "+archive+"\n"), 0644); e != nil {
		return e
	}
	fmt.Println(path)
	return nil
}
func writeArchive(path string, files map[string][]byte, binary string) error {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var manifest strings.Builder
	for _, k := range keys {
		sum := sha256.Sum256(files[k])
		fmt.Fprintf(&manifest, "%x  %s\n", sum, k)
	}
	files["SHA256SUMS"] = []byte(manifest.String())
	keys = append(keys, "SHA256SUMS")
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	z := zip.NewWriter(f)
	for _, k := range keys {
		h := &zip.FileHeader{Name: k, Method: zip.Deflate}
		h.SetMode(0644)
		if k == binary {
			h.SetMode(0755)
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			z.Close()
			f.Close()
			return err
		}
		if _, err = w.Write(files[k]); err != nil {
			z.Close()
			f.Close()
			return err
		}
	}
	if e = z.Close(); e != nil {
		f.Close()
		return e
	}
	return f.Close()
}
