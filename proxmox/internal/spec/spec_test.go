package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/testutil"
)

func TestStrictDocuments(t *testing.T) {
	for _, body := range []string{"apiVersion: pvecli/v1\nkind: Config\nunknown: true\n", "apiVersion: pvecli/v1\n---\nkind: Config\n", "kind: Config\nkind: Config\n"} {
		p := filepath.Join(t.TempDir(), "bad.yaml")
		_ = os.WriteFile(p, []byte(body), 0600)
		var c spec.Config
		if err := spec.Read(p, &c); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}
func TestContextRejectsUnsafeURLs(t *testing.T) {
	for _, endpoint := range []string{"http://pve", "https://user:pass@pve", "https://pve/?secret=x", "https://pve/other"} {
		c := spec.Config{APIVersion: spec.Version, Kind: "Config", DefaultContext: "lab", Contexts: map[string]spec.Context{"lab": {Endpoint: endpoint, TokenID: "ops@pve!cli", TokenSecretEnv: "PVE_TOKEN"}}}
		if _, _, e := c.Select(""); e == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
}
func TestDeploymentValidation(t *testing.T) {
	s := testutil.Deploy(t)
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.Network.IPv4 = "192.0.2.20/24"
	if e := s.Validate(); e == nil {
		t.Fatal("static address accepted without gateway")
	}
	s.Network.Gateway = "192.0.2.1"
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.VMID = s.TemplateVMID
	if e := s.Validate(); e == nil {
		t.Fatal("same source and destination")
	}
}
func TestMediaAndRelativePaths(t *testing.T) {
	if e := (spec.Media{File: "x.iso"}).Validate(); e == nil {
		t.Fatal("unchecked local ISO")
	}
	if e := (spec.Media{Volume: "local:iso/../x.iso"}).Validate(); e == nil {
		t.Fatal("traversal")
	}
	s := testutil.Deploy(t)
	s.Guest.KeyFile = "keys/id"
	p := filepath.Join(t.TempDir(), "deploy.yaml")
	body := `apiVersion: pvecli/v1
kind: VMDeployment
context: lab
template_vmid: 9000
vmid: 201
name: guest
node: pve
storage: local-lvm
pool: lab
network: {bridge: vmbr0, ipv4: dhcp}
guest: {os: ubuntu-24.04, user: pveadmin, key_file: keys/id}
`
	_ = os.WriteFile(p, []byte(body), 0600)
	got, e := spec.LoadDeploy(p)
	if e != nil {
		t.Fatal(e)
	}
	if !filepath.IsAbs(got.Guest.KeyFile) || !strings.HasSuffix(filepath.ToSlash(got.Guest.KeyFile), "keys/id") {
		t.Fatal(got.Guest.KeyFile)
	}
}
