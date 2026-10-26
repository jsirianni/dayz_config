// Package spec defines the versioned, strictly decoded CLI inputs.
package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const Version = "pvecli/v1"

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Identifier(v string) bool { return identifier.MatchString(v) }
func VMID(v int) bool          { return v >= 100 && v <= 999999999 }

type Context struct {
	Endpoint       string `yaml:"endpoint" json:"endpoint"`
	TokenID        string `yaml:"token_id" json:"token_id"`
	TokenSecretEnv string `yaml:"token_secret_env" json:"token_secret_env"`
}
type Config struct {
	APIVersion     string             `yaml:"apiVersion" json:"apiVersion"`
	Kind           string             `yaml:"kind" json:"kind"`
	DefaultContext string             `yaml:"default_context" json:"default_context"`
	Contexts       map[string]Context `yaml:"contexts" json:"contexts"`
}
type Hardware struct {
	CPU       string `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Cores     int    `yaml:"cores,omitempty" json:"cores,omitempty"`
	Sockets   int    `yaml:"sockets,omitempty" json:"sockets,omitempty"`
	MemoryMiB int    `yaml:"memory_mib,omitempty" json:"memory_mib,omitempty"`
	DiskGiB   int    `yaml:"disk_gib,omitempty" json:"disk_gib,omitempty"`
	Disk      string `yaml:"disk,omitempty" json:"disk,omitempty"`
}
type Network struct {
	Interface string `yaml:"interface" json:"interface"`
	Bridge    string `yaml:"bridge" json:"bridge"`
	VLAN      int    `yaml:"vlan,omitempty" json:"vlan,omitempty"`
	IPv4      string `yaml:"ipv4" json:"ipv4"`
	Gateway   string `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DNS       string `yaml:"dns,omitempty" json:"dns,omitempty"`
}
type Guest struct {
	OS            string `yaml:"os" json:"os"`
	User          string `yaml:"user" json:"user"`
	KeyFile       string `yaml:"key_file" json:"key_file"`
	PassphraseEnv string `yaml:"passphrase_env,omitempty" json:"passphrase_env,omitempty"`
	Port          int    `yaml:"port,omitempty" json:"port,omitempty"`
}
type Deploy struct {
	APIVersion   string   `yaml:"apiVersion" json:"apiVersion"`
	Kind         string   `yaml:"kind" json:"kind"`
	Context      string   `yaml:"context" json:"context"`
	TemplateVMID int      `yaml:"template_vmid" json:"template_vmid"`
	VMID         int      `yaml:"vmid" json:"vmid"`
	Name         string   `yaml:"name" json:"name"`
	Node         string   `yaml:"node" json:"node"`
	Storage      string   `yaml:"storage" json:"storage"`
	Pool         string   `yaml:"pool" json:"pool"`
	Hardware     Hardware `yaml:"hardware" json:"hardware"`
	Network      Network  `yaml:"network" json:"network"`
	Guest        Guest    `yaml:"guest" json:"guest"`
}
type Media struct {
	File   string `yaml:"file,omitempty" json:"file,omitempty"`
	Volume string `yaml:"volume,omitempty" json:"volume,omitempty"`
	SHA256 string `yaml:"sha256,omitempty" json:"sha256,omitempty"`
}
type Artifact struct {
	URL    string `yaml:"url" json:"url"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}
type Build struct {
	APIVersion   string   `yaml:"apiVersion" json:"apiVersion"`
	Kind         string   `yaml:"kind" json:"kind"`
	Context      string   `yaml:"context" json:"context"`
	OS           string   `yaml:"os" json:"os"`
	VMID         int      `yaml:"vmid" json:"vmid"`
	Name         string   `yaml:"name" json:"name"`
	Node         string   `yaml:"node" json:"node"`
	Storage      string   `yaml:"storage" json:"storage"`
	ISOStorage   string   `yaml:"iso_storage" json:"iso_storage"`
	Pool         string   `yaml:"pool" json:"pool"`
	Bridge       string   `yaml:"bridge" json:"bridge"`
	VLAN         int      `yaml:"vlan,omitempty" json:"vlan,omitempty"`
	Hardware     Hardware `yaml:"hardware" json:"hardware"`
	ISO          Media    `yaml:"iso" json:"iso"`
	VirtIO       Media    `yaml:"virtio,omitempty" json:"virtio,omitempty"`
	Cloudbase    Artifact `yaml:"cloudbase_init,omitempty" json:"cloudbase_init,omitempty"`
	WindowsImage string   `yaml:"windows_image,omitempty" json:"windows_image,omitempty"`
}

func Read(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("exactly one YAML document is required")
	}
	return nil
}
func DefaultConfigPath() (string, error) {
	p, e := os.UserConfigDir()
	return filepath.Join(p, "pvecli", "config.yaml"), e
}
func DefaultStateDir() (string, error) {
	p, e := os.UserConfigDir()
	return filepath.Join(p, "pvecli", "state"), e
}
func DefaultToolsDir() (string, error) {
	p, e := os.UserCacheDir()
	return filepath.Join(p, "pvecli", "tools"), e
}
func (c Config) Select(name string) (Context, string, error) {
	if c.APIVersion != Version || c.Kind != "Config" {
		return Context{}, "", errors.New("expected apiVersion pvecli/v1 and kind Config")
	}
	if name == "" {
		name = c.DefaultContext
	}
	v, ok := c.Contexts[name]
	if !ok || !Identifier(name) {
		return Context{}, "", errors.New("select a configured context with --context or default_context")
	}
	u, e := url.Parse(v.Endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (strings.TrimRight(u.Path, "/") != "" && strings.TrimRight(u.Path, "/") != "/api2/json") {
		return Context{}, "", errors.New("endpoint must be an HTTPS origin or /api2/json URL, without credentials/query")
	}
	if !regexp.MustCompile(`^[^\s=]+@[^\s!=]+![^\s=]+$`).MatchString(v.TokenID) || !envName.MatchString(v.TokenSecretEnv) {
		return Context{}, "", errors.New("context requires USER@REALM!TOKEN and a token_secret_env name")
	}
	return v, name, nil
}
func Resolve(path, relative string) string {
	if relative == "" || filepath.IsAbs(relative) {
		return relative
	}
	a, _ := filepath.Abs(filepath.Join(filepath.Dir(path), relative))
	return a
}
func LoadDeploy(path string) (Deploy, error) {
	var s Deploy
	if e := Read(path, &s); e != nil {
		return s, e
	}
	s.Guest.KeyFile = Resolve(path, s.Guest.KeyFile)
	if s.Guest.Port == 0 {
		s.Guest.Port = 22
	}
	if s.Network.Interface == "" {
		s.Network.Interface = "net0"
	}
	if s.Hardware.Disk == "" {
		s.Hardware.Disk = "scsi0"
	}
	return s, s.Validate()
}
func LoadBuild(path string) (Build, error) {
	var s Build
	if e := Read(path, &s); e != nil {
		return s, e
	}
	s.ISO.File = Resolve(path, s.ISO.File)
	s.VirtIO.File = Resolve(path, s.VirtIO.File)
	if s.Hardware.Cores == 0 {
		s.Hardware.Cores = 2
	}
	if s.Hardware.MemoryMiB == 0 {
		s.Hardware.MemoryMiB = 4096
	}
	if s.Hardware.DiskGiB == 0 {
		s.Hardware.DiskGiB = 64
	}
	return s, s.Validate()
}
func (h Hardware) Validate() error {
	if h.Cores < 0 || h.Cores > 8192 || h.Sockets < 0 || h.Sockets > 16 || h.MemoryMiB < 0 || (h.MemoryMiB > 0 && h.MemoryMiB < 128) || h.DiskGiB < 0 {
		return errors.New("invalid hardware size")
	}
	if h.CPU != "" && !Identifier(h.CPU) {
		return errors.New("invalid CPU model")
	}
	if h.Disk != "" && !regexp.MustCompile(`^(scsi|sata|virtio|ide)[0-9]+$`).MatchString(h.Disk) {
		return errors.New("invalid disk name")
	}
	return nil
}
func (n Network) Validate() error {
	if !regexp.MustCompile(`^net[0-9]+$`).MatchString(n.Interface) || !Identifier(n.Bridge) || n.VLAN < 0 || n.VLAN > 4094 {
		return errors.New("invalid network interface, bridge or VLAN")
	}
	if n.IPv4 == "dhcp" {
		if n.Gateway != "" {
			return errors.New("DHCP must not specify gateway")
		}
	} else {
		ip, _, e := net.ParseCIDR(n.IPv4)
		if e != nil || ip.To4() == nil {
			return errors.New("ipv4 must be dhcp or an IPv4 CIDR")
		}
		gw := net.ParseIP(n.Gateway)
		if gw == nil || gw.To4() == nil {
			return errors.New("static IPv4 requires an IPv4 gateway")
		}
	}
	if n.DNS != "" {
		for _, s := range strings.Fields(n.DNS) {
			if net.ParseIP(s) == nil {
				return errors.New("DNS must contain IP addresses")
			}
		}
	}
	return nil
}
func (g Guest) Validate() error {
	if g.OS != "ubuntu-24.04" && g.OS != "windows-2022" && g.OS != "windows-2025" {
		return errors.New("unsupported guest OS")
	}
	if !Identifier(g.User) || g.KeyFile == "" || g.Port < 1 || g.Port > 65535 || (g.PassphraseEnv != "" && !envName.MatchString(g.PassphraseEnv)) {
		return errors.New("guest requires user, key_file and valid port/passphrase_env")
	}
	return nil
}
func (s Deploy) Validate() error {
	if s.APIVersion != Version || s.Kind != "VMDeployment" {
		return errors.New("expected apiVersion pvecli/v1 and kind VMDeployment")
	}
	if !VMID(s.VMID) || !VMID(s.TemplateVMID) || s.VMID == s.TemplateVMID || !Identifier(s.Name) || !Identifier(s.Node) || !Identifier(s.Storage) || !Identifier(s.Pool) || !Identifier(s.Context) {
		return errors.New("deployment requires valid explicit VMIDs, name, context, node, storage and pool")
	}
	if e := s.Hardware.Validate(); e != nil {
		return e
	}
	if e := s.Network.Validate(); e != nil {
		return e
	}
	return s.Guest.Validate()
}
func validHash(v string) bool { b, e := hex.DecodeString(v); return e == nil && len(b) == 32 }
func (m Media) Validate() error {
	if (m.File == "") == (m.Volume == "") {
		return errors.New("media requires exactly one of file or volume")
	}
	if m.File != "" && !validHash(m.SHA256) {
		return errors.New("local media requires SHA-256")
	}
	if m.Volume != "" && !regexp.MustCompile(`^[A-Za-z0-9_.-]+:iso/[A-Za-z0-9_.-]+\.iso$`).MatchString(m.Volume) {
		return errors.New("media volume must be storage:iso/name.iso")
	}
	return nil
}
func (s Build) Validate() error {
	if s.Hardware.Cores > 255 || s.Hardware.Disk != "" {
		return errors.New("template build supports at most 255 cores and a single automatically named boot disk")
	}
	if s.APIVersion != Version || s.Kind != "TemplateBuild" {
		return errors.New("expected apiVersion pvecli/v1 and kind TemplateBuild")
	}
	if !VMID(s.VMID) || !Identifier(s.Name) || !Identifier(s.Context) || !Identifier(s.Node) || !Identifier(s.Storage) || !Identifier(s.ISOStorage) || !Identifier(s.Pool) || !Identifier(s.Bridge) || s.VLAN < 0 || s.VLAN > 4094 {
		return errors.New("template build requires explicit VMID, context, name, node, storage, iso_storage, pool and bridge")
	}
	if s.OS != "ubuntu-24.04" && s.OS != "windows-2022" && s.OS != "windows-2025" {
		return errors.New("unsupported template OS")
	}
	if e := s.Hardware.Validate(); e != nil {
		return e
	}
	if e := s.ISO.Validate(); e != nil {
		return e
	}
	if strings.HasPrefix(s.OS, "windows") {
		if e := s.VirtIO.Validate(); e != nil {
			return fmt.Errorf("virtio: %w", e)
		}
		u, e := url.Parse(s.Cloudbase.URL)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !validHash(s.Cloudbase.SHA256) || s.WindowsImage == "" || strings.ContainsAny(s.WindowsImage, "\r\n") {
			return errors.New("Windows requires windows_image and HTTPS cloudbase_init URL with SHA-256")
		}
	} else if s.WindowsImage != "" || s.VirtIO.File != "" || s.VirtIO.Volume != "" || s.Cloudbase.URL != "" {
		return errors.New("Windows options cannot be used for Ubuntu")
	}
	return nil
}
func Digest(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
