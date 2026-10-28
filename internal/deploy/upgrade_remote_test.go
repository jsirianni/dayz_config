package deploy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"
)

func decodeUpgradeCollector(t *testing.T, command string) string {
	t.Helper()
	prefix := "powershell.exe -NoProfile -NonInteractive -EncodedCommand "
	if !strings.HasPrefix(command, prefix) {
		t.Fatal("unexpected remote command", command)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(command, prefix))
	if err != nil {
		t.Fatal(err)
	}
	words := make([]uint16, len(raw)/2)
	for i := range words {
		words[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	source := string(utf16.Decode(words))
	const compressedPrefix = "$b=[Convert]::FromBase64String('"
	if strings.HasPrefix(source, compressedPrefix) {
		encoded, _, _ := strings.Cut(strings.TrimPrefix(source, compressedPrefix), "')")
		payload, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil {
			t.Fatal(e)
		}
		reader, e := gzip.NewReader(bytes.NewReader(payload))
		if e != nil {
			t.Fatal(e)
		}
		defer reader.Close()
		decoded, e := io.ReadAll(reader)
		if e != nil {
			t.Fatal(e)
		}
		source = string(decoded)
	}
	return source
}

func parseUpgradeCollector(t *testing.T, source string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	check := fmt.Sprintf("$s=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s'));$tokens=$null;$errors=$null;[Management.Automation.Language.Parser]::ParseInput($s,[ref]$tokens,[ref]$errors)|Out-Null;if($errors.Count -gt 0){$errors|ConvertTo-Json -Depth 3;exit 1}", base64.StdEncoding.EncodeToString([]byte(source)))
	encoded := strings.TrimPrefix(upgradePowerShell(check), "powershell.exe -NoProfile -NonInteractive -EncodedCommand ")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	prepareCommand(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invalid Windows PowerShell collector: %v\n%s", err, out)
	}
}

func TestAgentUpgradeUsesTrustedWindowsPowerShellModules(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell module discovery")
	}
	untrusted := t.TempDir()
	for _, module := range []struct{ name, exports string }{{"Microsoft.PowerShell.Security", "'Get-Acl','Set-Acl'"}, {"CimCmdlets", "'Get-CimInstance'"}} {
		directory := filepath.Join(untrusted, module.name)
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		manifest := "@{ModuleVersion='99.0';PowerShellVersion='99.0';RootModule='poison.psm1';FunctionsToExport=@(" + module.exports + ")}"
		if err := os.WriteFile(filepath.Join(directory, module.name+".psd1"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "poison.psm1"), []byte("throw 'incompatible inherited module was loaded'"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := `$trusted=[IO.Path]::Combine($PSHOME,'Modules');if($env:PSModulePath -ne $trusted){throw 'Inherited module search path was retained'};foreach($name in @('Get-Acl','Set-Acl','Get-CimInstance')){$command=Get-Command $name -ErrorAction Stop;$manifest=$command.Module.Path.StartsWith($trusted+'\',[StringComparison]::OrdinalIgnoreCase);$binaryBase=$command.Module.ModuleBase.StartsWith($trusted+'\',[StringComparison]::OrdinalIgnoreCase);if(-not($manifest -or $binaryBase)){throw ('Untrusted module loaded for '+$name)}};Write-Output 'trusted built-in commands loaded'`
	command := upgradeSpecScript(map[string]string{}, source)
	encoded := strings.TrimPrefix(command, "powershell.exe -NoProfile -NonInteractive -EncodedCommand ")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	prepareCommand(cmd)
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.EqualFold(key, "PSModulePath") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "PSModulePath="+untrusted)
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "trusted built-in commands loaded") {
		t.Fatalf("upgrade did not isolate inherited PowerShell modules: %v\n%s", err, out)
	}
}

func TestAgentUpgradeEvidenceWithoutRunningAgent(t *testing.T) {
	id := NewID()
	result := UpgradeResult{Version: Protocol, ID: id, Target: "nasdara-staging", Status: "succeeded", Phase: "complete", Updated: time.Now().UTC(), Continuity: true}
	var requests atomic.Int32
	endpoint, root := sshExecFixture(t, func(command string, _ io.Reader, out io.Writer) error {
		requests.Add(1)
		source := decodeUpgradeCollector(t, command)
		parseUpgradeCollector(t, source)
		for _, required := range []string{"$env:ProgramData", "dzdeploy", "Get-Acl", "GetOwner", "GetAccessRules", "ReparsePoint", "Assert-Protected", "ReadAllText"} {
			if !strings.Contains(source, required) {
				t.Error("evidence collector omitted authority check", required)
			}
		}
		for _, mutation := range []string{"Set-Acl", "Start-Service", "New-Item", "WriteAllText", "Stop-Process"} {
			if strings.Contains(source, mutation) {
				t.Error("status collector mutates host", mutation)
			}
		}
		return json.NewEncoder(out).Encode(result)
	})
	for _, name := range []string{"state.json", "heartbeat.json"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(endpoint.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	remote, err := Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	got, err := remote.WaitAgentUpgrade(ctx, result.Target, id, nil)
	if err != nil || got.ID != id || !got.Continuity {
		t.Fatal(got, err)
	}
	if requests.Load() != 1 {
		t.Fatal("terminal upgrade reattached more than once")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("status modified remote tree", entries, err)
	}
	after, err := os.ReadFile(endpoint.KnownHosts)
	if err != nil || string(before) != string(after) {
		t.Fatal("status changed SSH trust", err)
	}
}

func TestAgentUpgradeSubmissionProtectsBeforeExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("SFTP execution fixture needs a native drive-qualified Windows host root")
	}
	var called atomic.Int32
	endpoint, root := sshExecFixture(t, func(command string, _ io.Reader, out io.Writer) error {
		called.Add(1)
		if len(command) > 8191 {
			t.Error("upgrade bridge exceeds cmd.exe command limit", len(command))
		}
		source := decodeUpgradeCollector(t, command)
		parseUpgradeCollector(t, source)
		for _, required := range []string{"Read-Captured $spec.binary $spec.binarySHA256", "Read-Captured $spec.request $spec.requestSHA256", "SetOwner", "Assert-Private $path", "$env:ProgramData", "& $exe agent-upgrade-submit --request $requestFile"} {
			if !strings.Contains(source, required) {
				t.Error("bridge omitted upload isolation", required)
			}
		}
		if strings.Index(source, "Read-Captured $spec.binary") > strings.Index(source, "& $exe agent-upgrade-submit") {
			t.Error("bridge executes before capturing verified bytes")
		}
		_, err := io.WriteString(out, `{"status":"submitted"}`)
		return err
	})
	host := testConfig(t)
	host.StateDir = root
	host.RConPasswordFile = filepath.Join(root, "secret")
	if err := WriteJSON(filepath.Join(root, "host.json"), host); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	remote, err := Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	image := []byte("fixture candidate is captured but never executed by this SSH fixture")
	req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: host.Target, CandidateSHA256: Digest(image), Revision: strings.Repeat("a", 40), Host: host}
	if err = remote.SubmitAgentUpgrade(ctx, req, image); err != nil {
		t.Fatal(err)
	}
	if called.Load() != 1 {
		t.Fatal("bridge was not submitted exactly once")
	}
	staged, err := os.ReadFile(filepath.Join(root, "setup", req.ID, "dzdeploy.exe"))
	if err != nil || string(staged) != string(image) {
		t.Fatal("staged upload differs from client bytes", err)
	}
}

func TestAgentUpgradeBridgeRejectsRuntimeOverlapBeforeACLChanges(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell path semantics")
	}
	var programData string
	var executed atomic.Bool
	endpoint, root := sshExecFixture(t, func(command string, _ io.Reader, _ io.Writer) error {
		executed.Store(true)
		encoded := strings.TrimPrefix(command, "powershell.exe -NoProfile -NonInteractive -EncodedCommand ")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
		prepareCommand(cmd)
		for _, env := range os.Environ() {
			key, _, _ := strings.Cut(env, "=")
			if !strings.EqualFold(key, "ProgramData") {
				cmd.Env = append(cmd.Env, env)
			}
		}
		cmd.Env = append(cmd.Env, "ProgramData="+programData)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "overlaps") {
			t.Errorf("bridge did not reject runtime overlap before mutation: %v\n%s", err, out)
		}
		return errors.New("expected protected-root overlap rejection")
	})
	programData = root // protected base would be inside this service state root.
	host := testConfig(t)
	host.StateDir = root
	host.RConPasswordFile = filepath.Join(root, "secret")
	if err := WriteJSON(filepath.Join(root, "host.json"), host); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	remote, err := Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	image := []byte("overlap regression candidate bytes")
	req := UpgradeRequest{Version: Protocol, ID: NewID(), Target: host.Target, CandidateSHA256: Digest(image), Revision: strings.Repeat("a", 40), Host: host}
	if err = remote.SubmitAgentUpgrade(ctx, req, image); err == nil {
		t.Fatal("overlapping upgrade accepted")
	}
	if !executed.Load() {
		t.Fatal("overlap bridge was not executed")
	}
	if _, err = os.Stat(filepath.Join(root, "dzdeploy")); !os.IsNotExist(err) {
		t.Fatal("overlap check modified protected-root ancestry", err)
	}
}

func TestAgentUpgradeEvidenceRejectsMissingAndForeignResults(t *testing.T) {
	for _, test := range []struct {
		name, body string
		missing    bool
	}{{"missing", `{"missing":true}`, true}, {"foreign", `{"version":1,"id":"20261027T000000-0000000000000000","target":"deer-isle","status":"succeeded"}`, false}} {
		t.Run(test.name, func(t *testing.T) {
			endpoint, _ := sshExecFixture(t, func(_ string, _ io.Reader, out io.Writer) error { _, err := io.WriteString(out, test.body); return err })
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remote, err := Connect(ctx, endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer remote.Close()
			_, err = remote.AgentUpgradeStatus(ctx, "nasdara-staging", "20261027T000000-0000000000000000")
			if err == nil || errors.Is(err, os.ErrNotExist) != test.missing {
				t.Fatal(err)
			}
		})
	}
	remote := &Remote{}
	if _, err := remote.AgentUpgradeStatus(context.Background(), "nasdara", "../../bad"); err == nil {
		t.Fatal("accepted traversal operation ID")
	}
}
