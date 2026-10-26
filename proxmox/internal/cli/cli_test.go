package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/testutil"
	"go.yaml.in/yaml/v3"
)

func app(t *testing.T) (*App, *testutil.API, *bytes.Buffer) {
	a := testutil.New(t)
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(config, []byte("apiVersion: pvecli/v1\nkind: Config\ndefault_context: lab\ncontexts:\n  lab:\n    endpoint: "+a.Server.URL+"\n    token_id: ops@pve!cli\n    token_secret_env: TEST_PVE_TOKEN\n"), 0600)
	t.Setenv("TEST_PVE_TOKEN", "top-secret-token")
	out := &bytes.Buffer{}
	return &App{ConfigPath: config, StateDir: filepath.Join(dir, "state"), Out: out, Err: &bytes.Buffer{}, ClientHook: func(c *pve.Client) { c.HTTP = a.Server.Client() }}, a, out
}
func TestOneJSONResultAndDryRun(t *testing.T) {
	for _, args := range [][]string{{"vm", "update", "201", "--memory-mib", "4096"}, {"vm", "nic", "update", "201", "--bridge", "vmbr0", "--vlan", "none"}, {"vm", "disk", "resize", "201", "--size", "100G"}, {"vm", "start", "201"}, {"vm", "shutdown", "201"}, {"vm", "reboot", "201"}, {"vm", "stop", "201", "--force"}} {
		a, api, out := app(t)
		cfg := api.GetVM(9000)
		cfg["template"] = 0
		api.SetVM(201, cfg)
		args = append(args, "--dry-run", "--json")
		if code := a.Execute(context.Background(), args); code != 0 {
			t.Fatal(args, code, out.String())
		}
		var v map[string]any
		d := json.NewDecoder(out)
		if e := d.Decode(&v); e != nil {
			t.Fatal(e)
		}
		if d.More() {
			t.Fatal("multiple JSON results")
		}
		if v["status"] != "planned" || len(api.Writes()) != 0 {
			t.Fatal(v, api.Writes())
		}
	}
}
func TestDeployDryRunDoesNotCreateState(t *testing.T) {
	a, api, out := app(t)
	s := testutil.Deploy(t)
	b, _ := yaml.Marshal(s)
	file := filepath.Join(t.TempDir(), "deploy.yaml")
	_ = os.WriteFile(file, b, 0600)
	code := a.Execute(context.Background(), []string{"vm", "deploy", "--file", file, "--dry-run", "--json"})
	if code != 0 {
		t.Fatal(code, out.String())
	}
	if _, e := os.Stat(a.StateDir); !os.IsNotExist(e) {
		t.Fatal("dry-run created state")
	}
	if len(api.Writes()) != 0 {
		t.Fatal("dry-run writes")
	}
}
func TestForceAndInvalidArguments(t *testing.T) {
	a, api, out := app(t)
	cfg := api.GetVM(9000)
	cfg["template"] = 0
	api.SetVM(201, cfg)
	if code := a.Execute(context.Background(), []string{"vm", "stop", "201", "--json"}); code != 2 {
		t.Fatal(code, out.String())
	}
	if len(api.Writes()) != 0 {
		t.Fatal("hard stop submitted without force")
	}
	a, _, out = app(t)
	if code := a.Execute(context.Background(), []string{"vm", "show", "../bad", "--json"}); code != 2 {
		t.Fatal(code, out.String())
	}
}
func TestToolsDryRunDoesNotRequireContext(t *testing.T) {
	out := &bytes.Buffer{}
	a := App{Out: out, Err: &bytes.Buffer{}, ToolsDir: filepath.Join(t.TempDir(), "missing")}
	if code := a.Execute(context.Background(), []string{"tools", "install", "--dry-run", "--json"}); code != 0 {
		t.Fatal(code, out.String())
	}
	if _, e := os.Stat(a.ToolsDir); !os.IsNotExist(e) {
		t.Fatal("tools dry-run wrote files")
	}
}
