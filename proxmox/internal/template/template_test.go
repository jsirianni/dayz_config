package template

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/engine"
	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/testutil"
	"github.com/jsirianni/dayz_config/proxmox/internal/tools"
	"go.yaml.in/yaml/v3"
)

func buildSpec(osName string) spec.Build {
	s := spec.Build{APIVersion: spec.Version, Kind: "TemplateBuild", Context: "lab", OS: osName, VMID: 9100, Name: "template", Node: "pve", Storage: "local-lvm", ISOStorage: "local", Pool: "lab", Bridge: "vmbr0", Hardware: spec.Hardware{Cores: 2, MemoryMiB: 4096, DiskGiB: 64}, ISO: spec.Media{Volume: "local:iso/os.iso"}}
	if strings.HasPrefix(osName, "windows") {
		s.WindowsImage = "Windows Server 2022 SERVERSTANDARD"
		s.VirtIO = spec.Media{Volume: "local:iso/virtio.iso"}
		s.Cloudbase = spec.Artifact{URL: "https://example.invalid/cloudbase.msi", SHA256: strings.Repeat("a", 64)}
	}
	return s
}
func TestRecipesAndAnswerMedia(t *testing.T) {
	for _, osName := range []string{"ubuntu-24.04", "windows-2022", "windows-2025"} {
		t.Run(osName, func(t *testing.T) {
			s := buildSpec(osName)
			dir := t.TempDir()
			_, key, e := BuildKey()
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteSeed(dir, s, key, "Secret<&>123"); e != nil {
				t.Fatal(e)
			}
			if osName == "ubuntu-24.04" {
				raw, _ := os.ReadFile(filepath.Join(dir, "user-data"))
				var doc map[string]any
				if e = yaml.Unmarshal(raw, &doc); e != nil {
					t.Fatal(e)
				}
				if !bytes.Contains(raw, []byte("qemu-guest-agent")) {
					t.Fatal("missing guest agent")
				}
			} else {
				raw, _ := os.ReadFile(filepath.Join(dir, "Autounattend.xml"))
				d := xml.NewDecoder(bytes.NewReader(raw))
				for {
					_, e = d.Token()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				if !bytes.Contains(raw, []byte("Secret&lt;&amp;&gt;123")) {
					t.Fatal("unescaped password")
				}
			}
			b, e := RenderPacker(s, filepath.Join(dir, "seed.iso"), strings.Repeat("b", 64), `C:\Program Files\pvecli.exe`, `C:\config path\config.yaml`, filepath.Join(dir, "worker.json"), time.Hour)
			if e != nil {
				t.Fatal(e)
			}
			var recipe struct {
				Builders     []map[string]any `json:"builders"`
				Provisioners []map[string]any `json:"provisioners"`
			}
			if e = json.Unmarshal(b, &recipe); e != nil {
				t.Fatal(e)
			}
			builder := recipe.Builders[0]
			seed := builder["additional_iso_files"].([]any)
			last := seed[len(seed)-1].(map[string]any)
			if last["iso_target_path"] != filepath.ToSlash(filepath.Join(dir, "seed.iso")) {
				t.Fatal("seed upload filename is not stable")
			}
			if builder["communicator"] != "none" || builder["insecure_skip_tls_verify"] != false {
				t.Fatal(builder)
			}
			argv := recipe.Provisioners[0]["execute_command"].([]any)
			if argv[0] != `C:\Program Files\pvecli.exe` || argv[len(argv)-2] != "--file" {
				t.Fatal(argv)
			}
			if bytes.Contains(b, []byte("Secret<&>123")) {
				t.Fatal("password leaked into Packer recipe")
			}
		})
	}
}
func TestSealingAssets(t *testing.T) {
	linux, _ := assets.ReadFile("assets/ubuntu-prepare.sh")
	for _, s := range []string{"cloud-init clean --logs --machine-id", "/etc/ssh/ssh_host_*", "userdel --force --remove pvebuild", "99-pvecli-build"} {
		if !bytes.Contains(linux, []byte(s)) {
			t.Fatal(s)
		}
	}
	win, _ := assets.ReadFile("assets/windows-prepare.ps1")
	for _, s := range []string{"/generalize", "/oobe", "/quit", "DefaultPassword", "ssh_host_*", "SHA-256 mismatch", "Disable-LocalUser"} {
		if !bytes.Contains(win, []byte(s)) {
			t.Fatal(s)
		}
	}
	first, _ := assets.ReadFile("assets/windows-firstboot.ps1")
	for _, s := range []string{"administrators_authorized_keys", "S-1-5-18", "S-1-5-32-544", "ssh-keygen.exe"} {
		if !bytes.Contains(first, []byte(s)) {
			t.Fatal(s)
		}
	}
}

type fakeRunner struct {
	calls   []tools.Invocation
	api     *testutil.API
	failure error
	proof   bool
}

func (f *fakeRunner) Run(ctx context.Context, i tools.Invocation, out, errOut io.Writer) error {
	f.calls = append(f.calls, i)
	if strings.Contains(filepath.Base(i.Binary), "packer") {
		if f.failure != nil {
			return f.failure
		}
		raw, e := os.ReadFile(i.Args[len(i.Args)-1])
		if e != nil {
			return e
		}
		var p struct {
			Provisioners []struct {
				Args []string `json:"execute_command"`
			} `json:"provisioners"`
		}
		if e = json.Unmarshal(raw, &p); e != nil {
			return e
		}
		path := p.Provisioners[0].Args[len(p.Provisioners[0].Args)-1]
		raw, e = os.ReadFile(path)
		if e != nil {
			return e
		}
		var w WorkerInput
		if e = json.Unmarshal(raw, &w); e != nil {
			return e
		}
		if f.proof {
			_ = os.WriteFile(w.DoneFile, []byte(w.OperationID), 0600)
		}
		f.api.SetVM(w.Build.VMID, pve.Config{"template": 1, "name": w.Build.Name, "tags": "pvecli;build-" + w.OperationID, "digest": "1"})
		return nil
	}
	dest := i.Args[len(i.Args)-1]
	if runtime.GOOS != "windows" {
		for index, arg := range i.Args {
			if arg == "-o" {
				dest = i.Args[index+1]
			}
		}
	}
	return os.WriteFile(dest, []byte("fixture ISO"), 0600)
}
func setupBuilder(t *testing.T) (Builder, *testutil.API, *fakeRunner) {
	api := testutil.New(t)
	dir := t.TempDir()
	manager := tools.Manager{Dir: filepath.Join(dir, "tools")}
	_ = os.MkdirAll(filepath.Dir(manager.Packer()), 0700)
	_ = os.WriteFile(manager.Packer(), []byte("fixture"), 0700)
	_ = os.WriteFile(filepath.Join(manager.Dir, "installed.txt"), []byte(tools.PackerVersion+"/"+tools.PluginVersion), 0600)
	pluginDir := filepath.Join(manager.PluginPath(), "github.com", "hashicorp", "proxmox")
	_ = os.MkdirAll(pluginDir, 0700)
	plugin := "packer-plugin-proxmox_v" + tools.PluginVersion + "_x5.0_" + runtime.GOOS + "_amd64"
	if runtime.GOOS == "windows" {
		plugin += ".exe"
	}
	_ = os.WriteFile(filepath.Join(pluginDir, plugin), []byte("fixture"), 0700)
	tool := "xorriso"
	if runtime.GOOS == "windows" {
		tool = "oscdimg.exe"
	}
	_ = os.WriteFile(filepath.Join(dir, tool), []byte("fixture"), 0700)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	config := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(config, []byte("apiVersion: pvecli/v1\nkind: Config\ndefault_context: lab\ncontexts:\n  lab:\n    endpoint: "+api.Server.URL+"\n    token_id: ops@pve!cli\n    token_secret_env: TEST_TEMPLATE_TOKEN\n"), 0600)
	t.Setenv("TEST_TEMPLATE_TOKEN", "top-secret-token")
	api.Override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/storage/local/content") {
			if r.Method == "GET" {
				testutil.Data(w, []any{map[string]string{"volid": "local:iso/os.iso"}, map[string]string{"volid": "local:iso/virtio.iso"}})
			} else {
				testutil.Data(w, nil)
			}
			return true
		}
		return false
	}
	e := &engine.Engine{Client: api.Client(), Store: operation.Store{Dir: filepath.Join(dir, "state")}, Context: "lab", Identity: "identity"}
	runner := &fakeRunner{api: api, proof: true}
	return Builder{Engine: e, Tools: manager, Runner: runner, ConfigPath: config}, api, runner
}
func TestBuilderDryRunNoProcessesOrWrites(t *testing.T) {
	b, api, runner := setupBuilder(t)
	r, e := b.Build(context.Background(), buildSpec("ubuntu-24.04"), true)
	if e != nil || r.Status != "planned" {
		t.Fatal(r, e)
	}
	if len(runner.calls) != 0 || len(api.Writes()) != 0 {
		t.Fatal("preview executed build")
	}
	if _, e = os.Stat(b.Engine.Store.Dir); !os.IsNotExist(e) {
		t.Fatal("preview wrote journal")
	}
}
func TestBuilderRequiresWorkerProofAndCleansKeys(t *testing.T) {
	for _, proof := range []bool{true, false} {
		t.Run(string(rune('0'+boolInt(proof))), func(t *testing.T) {
			b, _, runner := setupBuilder(t)
			runner.proof = proof
			r, e := b.Build(context.Background(), buildSpec("ubuntu-24.04"), false)
			if (e == nil) != proof {
				t.Fatal(r, e)
			}
			if len(runner.calls) != 2 {
				t.Fatal(runner.calls)
			}
			dir := filepath.Join(b.Engine.Store.Dir, "builds", r.OperationID)
			if _, e = os.Stat(dir); !os.IsNotExist(e) {
				t.Fatal("temporary credentials retained")
			}
		})
	}
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func TestPackerFailureAndCancellation(t *testing.T) {
	for _, failure := range []error{errors.New("packer failed"), context.Canceled} {
		b, _, runner := setupBuilder(t)
		runner.failure = failure
		ctx := context.Background()
		if failure == context.Canceled {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			ctx = cancelCtx
		}
		r, e := b.Build(ctx, buildSpec("ubuntu-24.04"), false)
		if e == nil {
			t.Fatal(r)
		}
	}
}
func TestRedactsSplitConcurrentOutput(t *testing.T) {
	var out bytes.Buffer
	w := &redactedWriter{out: &out, secrets: []string{"secret-token"}}
	_, _ = w.Write([]byte("value secret-"))
	_, _ = w.Write([]byte("token\n"))
	w.Flush()
	if strings.Contains(out.String(), "secret-token") || !strings.Contains(out.String(), "[redacted]") {
		t.Fatal(out.String())
	}
}
func TestRealPackerValidation(t *testing.T) {
	binary := os.Getenv("PVECLI_TEST_PACKER")
	plugins := os.Getenv("PVECLI_TEST_PLUGINS")
	if binary == "" {
		t.Skip("set PVECLI_TEST_PACKER and PVECLI_TEST_PLUGINS for local pinned-tool validation")
	}
	for _, name := range []string{"ubuntu-24.04", "windows-2022", "windows-2025"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			seed := filepath.Join(dir, "seed.iso")
			_ = os.WriteFile(seed, []byte("seed"), 0600)
			b, e := RenderPacker(buildSpec(name), seed, strings.Repeat("b", 64), binary, "config.yaml", filepath.Join(dir, "worker.json"), time.Hour)
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(dir, "recipe.json")
			_ = os.WriteFile(path, b, 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "validate", path)
			cmd.Env = tools.CleanEnv(map[string]string{"PACKER_PLUGIN_PATH": plugins, "PROXMOX_URL": "https://pve.example.invalid:8006/api2/json", "PROXMOX_USERNAME": "ops@pve!cli", "PROXMOX_TOKEN": "unused-validation-token", "CHECKPOINT_DISABLE": "1"})
			output, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("packer validate: %v\n%s", e, output)
			}
		})
	}
}
