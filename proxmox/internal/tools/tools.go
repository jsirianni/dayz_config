// Package tools installs pinned build tools into a user-scoped cache. Nothing
// is installed in the repository or searched through an untrusted working dir.
package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
)

const PackerVersion = "1.16.2"
const PluginVersion = "1.2.4"

type Invocation struct {
	Binary    string
	Args, Env []string
	Dir       string
}
type Runner interface {
	Run(context.Context, Invocation, io.Writer, io.Writer) error
}
type Exec struct{}

func (Exec) Run(ctx context.Context, i Invocation, out, errOut io.Writer) error {
	c := exec.CommandContext(ctx, i.Binary, i.Args...)
	c.Dir = i.Dir
	c.Env = i.Env
	c.Stdout = out
	c.Stderr = errOut
	c.WaitDelay = 5 * time.Second
	cleanup, e := startManaged(c)
	if e != nil {
		return e
	}
	defer cleanup()
	err := c.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

type Manager struct {
	Dir    string
	Runner Runner
	HTTP   *http.Client
}

func (m Manager) Packer() string {
	name := "packer"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(m.Dir, "packer-"+PackerVersion, name)
}
func (m Manager) PluginPath() string { return filepath.Join(m.Dir, "plugins") }
func (m Manager) Env() []string {
	return CleanEnv(map[string]string{"PACKER_PLUGIN_PATH": m.PluginPath(), "CHECKPOINT_DISABLE": "1", "PACKER_LOG": "0", "PACKER_LOG_PATH": ""})
}
func CleanEnv(values map[string]string) []string {
	out := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if _, ok := values[k]; ok {
			continue
		}
		if strings.HasPrefix(k, "PACKER_") || strings.HasPrefix(k, "PKR_VAR_") || strings.HasPrefix(k, "PROXMOX_") {
			continue
		}
		out = append(out, v)
	}
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out
}
func (m Manager) Check() error {
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "windows" && runtime.GOOS != "linux") {
		return errors.New("template builds support Windows/Linux amd64")
	}
	if _, e := os.Stat(m.Packer()); e != nil {
		return errors.New("pinned Packer missing; run pvecli tools install")
	}
	marker, e := os.ReadFile(filepath.Join(m.Dir, "installed.txt"))
	if e != nil || strings.TrimSpace(string(marker)) != PackerVersion+"/"+PluginVersion {
		return errors.New("pinned plugin missing; run pvecli tools install")
	}
	plugin := fmt.Sprintf("packer-plugin-proxmox_v%s_x5.0_%s_amd64", PluginVersion, runtime.GOOS)
	if runtime.GOOS == "windows" {
		plugin += ".exe"
	}
	if _, e = os.Stat(filepath.Join(m.PluginPath(), "github.com", "hashicorp", "proxmox", plugin)); e != nil {
		return errors.New("pinned plugin binary missing; run pvecli tools install")
	}
	iso := "xorriso"
	if runtime.GOOS == "windows" {
		iso = "oscdimg"
	}
	if _, e = exec.LookPath(iso); e != nil {
		return fmt.Errorf("template builds require %s on PATH", iso)
	}
	return nil
}
func (m Manager) fetch(ctx context.Context, u string, max int64) ([]byte, error) {
	h := m.HTTP
	if h == nil {
		h = &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if r.URL.Scheme != "https" {
				return errors.New("download redirect must use HTTPS")
			}
			if len(via) > 5 {
				return errors.New("too many redirects")
			}
			return nil
		}}
	}
	r, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, e
	}
	resp, e := h.Do(r)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download %s: HTTP %d", u, resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, errors.New("download exceeds size limit")
	}
	return b, nil
}
func Checksum(manifest []byte, name string) (string, error) {
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			b, e := hex.DecodeString(fields[0])
			if e != nil || len(b) != 32 {
				return "", errors.New("invalid SHA-256 manifest")
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksum missing for %s", name)
}
func Verify(b []byte, expected string) error {
	s := sha256.Sum256(b)
	if hex.EncodeToString(s[:]) != strings.ToLower(expected) {
		return errors.New("SHA-256 mismatch")
	}
	return nil
}
func extractBinary(b []byte, name, dest string) error {
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return e
	}
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > 300<<20 {
			return errors.New("binary exceeds extraction limit")
		}
		r, e := f.Open()
		if e != nil {
			return e
		}
		defer r.Close()
		out, e := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, io.LimitReader(r, 300<<20))
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	}
	return fmt.Errorf("archive missing %s", name)
}
func (m Manager) Install(ctx context.Context, out io.Writer) error {
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "windows" && runtime.GOOS != "linux") {
		return errors.New("tools support Windows/Linux amd64")
	}
	if e := operation.SecureDir(m.Dir); e != nil {
		return e
	}
	tmp, e := os.MkdirTemp(m.Dir, ".install-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	packerZip := fmt.Sprintf("packer_%s_%s_amd64.zip", PackerVersion, runtime.GOOS)
	base := "https://releases.hashicorp.com/packer/" + PackerVersion + "/"
	manifest, e := m.fetch(ctx, base+"packer_"+PackerVersion+"_SHA256SUMS", 1<<20)
	if e != nil {
		return e
	}
	sum, e := Checksum(manifest, packerZip)
	if e != nil {
		return e
	}
	b, e := m.fetch(ctx, base+packerZip, 200<<20)
	if e != nil {
		return e
	}
	if e = Verify(b, sum); e != nil {
		return e
	}
	packer := filepath.Join(tmp, "packer"+suffix)
	if e = extractBinary(b, "packer"+suffix, packer); e != nil {
		return e
	}
	pluginZip := fmt.Sprintf("packer-plugin-proxmox_v%s_x5.0_%s_amd64.zip", PluginVersion, runtime.GOOS)
	pluginBase := "https://github.com/hashicorp/packer-plugin-proxmox/releases/download/v" + PluginVersion + "/"
	manifest, e = m.fetch(ctx, pluginBase+"packer-plugin-proxmox_v"+PluginVersion+"_SHA256SUMS", 1<<20)
	if e != nil {
		return e
	}
	sum, e = Checksum(manifest, pluginZip)
	if e != nil {
		return e
	}
	b, e = m.fetch(ctx, pluginBase+pluginZip, 100<<20)
	if e != nil {
		return e
	}
	if e = Verify(b, sum); e != nil {
		return e
	}
	pluginName := strings.TrimSuffix(pluginZip, ".zip") + suffix
	plugin := filepath.Join(tmp, pluginName)
	if e = extractBinary(b, pluginName, plugin); e != nil {
		return e
	}
	if e = operation.SecureDir(filepath.Dir(m.Packer())); e != nil {
		return e
	}
	pb, e := os.ReadFile(packer)
	if e != nil {
		return e
	}
	if e = os.WriteFile(m.Packer(), pb, 0700); e != nil {
		return e
	}
	runner := m.Runner
	if runner == nil {
		runner = Exec{}
	}
	if e = runner.Run(ctx, Invocation{Binary: m.Packer(), Args: []string{"plugins", "install", "--force", "--path", plugin, "github.com/hashicorp/proxmox"}, Env: m.Env(), Dir: tmp}, out, out); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(m.Dir, "installed.txt"), []byte(PackerVersion+"/"+PluginVersion+"\n"), 0600)
}
