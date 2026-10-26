// Package template owns versioned unattended recipes and the Packer boundary.
package template

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf16"

	"github.com/jsirianni/dayz_config/proxmox/internal/engine"
	"github.com/jsirianni/dayz_config/proxmox/internal/guest"
	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/tools"
	"golang.org/x/crypto/ssh"
)

//go:embed assets/*
var assets embed.FS

const RecipeVersion = "1"

type Builder struct {
	Engine     *engine.Engine
	Tools      tools.Manager
	Runner     tools.Runner
	ConfigPath string
	Progress   io.Writer
}
type WorkerInput struct {
	APIVersion               string `json:"apiVersion"`
	OperationID              string `json:"operation_id"`
	Context, ContextIdentity string
	Build                    spec.Build
	KeyFile, DoneFile        string
}

func (b Builder) Build(ctx context.Context, s spec.Build, dry bool) (result *operation.Result, retErr error) {
	e := b.Engine
	if er := s.Validate(); er != nil {
		return nil, engine.Input(er)
	}
	if s.Context != e.Context {
		return nil, engine.Input(errors.New("build context does not match selected context"))
	}
	if er := b.Tools.Check(); er != nil {
		return nil, engine.Input(er)
	}
	for _, m := range []spec.Media{s.ISO, s.VirtIO} {
		if m.File != "" {
			if er := verifyFile(m.File, m.SHA256); er != nil {
				return nil, engine.Input(er)
			}
		}
	}
	if er := e.Preflight(ctx, s.Node, s.Storage, s.Pool, s.Bridge, s.VLAN, s.VMID); er != nil {
		return nil, er
	}
	isoStores, er := e.Client.Rows(ctx, "/nodes/"+url.PathEscape(s.Node)+"/storage")
	if er != nil {
		return nil, er
	}
	isoOK := false
	for _, v := range isoStores {
		if v.String("storage") == s.ISOStorage && v.Bool("active") && v.Bool("enabled") && strings.Contains(","+v.String("content")+",", ",iso,") {
			isoOK = true
		}
	}
	if !isoOK {
		return nil, engine.Input(errors.New("iso_storage is unavailable or does not accept ISO images"))
	}
	for _, m := range []spec.Media{s.ISO, s.VirtIO} {
		if m.Volume != "" {
			store, _, _ := strings.Cut(m.Volume, ":")
			rows, err := e.Client.Rows(ctx, "/nodes/"+url.PathEscape(s.Node)+"/storage/"+url.PathEscape(store)+"/content")
			if err != nil {
				return nil, err
			}
			found := false
			for _, row := range rows {
				if row.String("volid") == m.Volume {
					found = true
				}
			}
			if !found {
				return nil, engine.Input(fmt.Errorf("ISO %s is not visible", m.Volume))
			}
		}
	}
	if er = e.Require(ctx, "/storage/"+s.Storage, "Datastore.AllocateSpace"); er != nil {
		return nil, er
	}
	if er = e.Require(ctx, "/storage/"+s.ISOStorage, "Datastore.AllocateTemplate"); er != nil {
		return nil, er
	}
	if er = e.Require(ctx, "/pool/"+s.Pool, "VM.Audit", "VM.Config.Cloudinit", "VM.Config.CDROM", "VM.Config.Options", "VM.PowerMgmt"); er != nil {
		return nil, er
	}
	if er = e.RequireGuest(ctx, "/pool/"+s.Pool); er != nil {
		return nil, er
	}
	r := &operation.Result{APIVersion: spec.Version, Status: "planned", Kind: "template-build", Context: e.Context, ContextIdentity: e.Identity, VMID: s.VMID, Node: s.Node, InputDigest: spec.Digest(s), Changes: map[string]string{"os": s.OS, "name": s.Name, "recipe_version": RecipeVersion, "workflow": "ISO install, native SSH preparation, generalize, template"}}
	if dry {
		return r, nil
	}
	unlock, er := e.Store.Lock(e.Context, s.VMID)
	if er != nil {
		return nil, engine.Input(er)
	}
	defer unlock()
	r.OperationID = operation.NewID()
	r.Status = "running"
	r.Phase = "prepare-media"
	if er = e.Save(r); er != nil {
		return r, er
	}
	dir := filepath.Join(e.Store.Dir, "builds", r.OperationID)
	if er = operation.SecureDir(dir); er != nil {
		return e.Fail(r, er, false)
	}
	// Always remove local answer files and temporary keys, even on a failed build.
	defer func() {
		if ce := os.RemoveAll(dir); ce != nil {
			r.Residual = append(r.Residual, dir)
			if retErr == nil {
				retErr = fmt.Errorf("remove temporary credentials: %w", ce)
				r.Status = "failed"
				r.Error = retErr.Error()
			}
			_ = e.Save(r)
		}
	}()
	key, pub, er := BuildKey()
	if er != nil {
		return e.Fail(r, er, false)
	}
	keyFile := filepath.Join(dir, "build.key")
	if er = os.WriteFile(keyFile, key, 0600); er != nil {
		return e.Fail(r, er, false)
	}
	passwordBytes := make([]byte, 24)
	if _, er = rand.Read(passwordBytes); er != nil {
		return e.Fail(r, er, false)
	}
	password := "Pve!" + hex.EncodeToString(passwordBytes)
	seedDir := filepath.Join(dir, "seed")
	if er = operation.SecureDir(seedDir); er != nil {
		return e.Fail(r, er, false)
	}
	if er = WriteSeed(seedDir, s, pub, password); er != nil {
		return e.Fail(r, er, false)
	}
	seedName := "pvecli-" + r.OperationID + ".iso"
	seedISO := filepath.Join(dir, seedName)
	seedVolume := s.ISOStorage + ":iso/" + seedName
	runner := b.Runner
	if runner == nil {
		runner = tools.Exec{}
	}
	progress := b.Progress
	if progress == nil {
		progress = io.Discard
	}
	safeProgress := &redactedWriter{out: progress, secrets: []string{e.Client.Secret, password}}
	defer safeProgress.Flush()
	isoTool, args := ISOCommand(seedDir, seedISO)
	if er = runner.Run(ctx, tools.Invocation{Binary: isoTool, Args: args, Dir: dir, Env: tools.CleanEnv(nil)}, safeProgress, safeProgress); er != nil {
		return e.Fail(r, fmt.Errorf("answer ISO creation: %w", er), false)
	}
	seedHash, er := hashFile(seedISO)
	if er != nil {
		return e.Fail(r, er, false)
	}
	workerFile := filepath.Join(dir, "worker.json")
	doneFile := filepath.Join(dir, "sealed.json")
	wi := WorkerInput{APIVersion: spec.Version, OperationID: r.OperationID, Context: e.Context, ContextIdentity: e.Identity, Build: s, KeyFile: keyFile, DoneFile: doneFile}
	wb, _ := json.Marshal(wi)
	if er = os.WriteFile(workerFile, wb, 0600); er != nil {
		return e.Fail(r, er, false)
	}
	executable, er := os.Executable()
	if er != nil {
		return e.Fail(r, er, false)
	}
	configPath, er := filepath.Abs(b.ConfigPath)
	if er != nil {
		return e.Fail(r, er, false)
	}
	remaining := 50 * time.Minute
	if d, ok := ctx.Deadline(); ok {
		remaining = time.Until(d)
	}
	recipe, er := RenderPacker(s, seedISO, seedHash, executable, configPath, workerFile, remaining)
	if er != nil {
		return e.Fail(r, er, false)
	}
	recipeFile := filepath.Join(dir, "recipe.json")
	if er = os.WriteFile(recipeFile, recipe, 0600); er != nil {
		return e.Fail(r, er, false)
	}
	// Explicitly restore a selected secret-env name even when it has a reserved prefix.
	var config spec.Config
	if er = spec.Read(configPath, &config); er != nil {
		return e.Fail(r, er, false)
	}
	conn, _, er := config.Select(e.Context)
	if er != nil {
		return e.Fail(r, er, false)
	}
	env := tools.CleanEnv(map[string]string{"PROXMOX_URL": e.Client.Base, "PROXMOX_USERNAME": e.Client.TokenID, "PROXMOX_TOKEN": e.Client.Secret, "PACKER_PLUGIN_PATH": b.Tools.PluginPath(), "PACKER_LOG": "0", "PACKER_LOG_PATH": "", "CHECKPOINT_DISABLE": "1", conn.TokenSecretEnv: e.Client.Secret})
	r.Phase = "packer-build"
	r.Residual = []string{seedVolume}
	if er = e.Save(r); er != nil {
		return r, er
	}
	// Only operation-owned answer media is ever eligible for automatic deletion.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if r.SubmissionPending {
			r.Residual = []string{seedVolume}
			r.Error += "; answer media retained because task submission is unresolved"
			if retErr == nil {
				retErr = errors.New(r.Error)
				r.Status = "unknown"
			}
			_ = e.Save(r)
			return
		}
		if r.ActiveTask != "" {
			if ce := e.Client.Wait(cleanupCtx, r.Node, r.ActiveTask); ce != nil {
				var failure *pve.TaskFailure
				if !errors.As(ce, &failure) {
					r.Residual = []string{seedVolume}
					r.Error += "; answer media cleanup awaits task reconciliation"
					if retErr == nil {
						retErr = ce
						r.Status = "unknown"
					}
					_ = e.Save(r)
					return
				}
			}
		}
		if ce := cleanupSeed(cleanupCtx, e.Client, s.Node, s.VMID, r.OperationID, seedVolume); ce != nil {
			r.Residual = []string{seedVolume}
			if r.Error != "" {
				r.Error += "; "
			}
			r.Error += "answer media cleanup pending: " + ce.Error()
			if retErr == nil {
				retErr = errors.New(r.Error)
				r.Status = "failed"
			}
		} else {
			r.Residual = nil
		}
		if se := e.Save(r); se != nil && retErr == nil {
			retErr = se
			r.Status = "failed"
		}
	}()
	er = runner.Run(ctx, tools.Invocation{Binary: b.Tools.Packer(), Args: []string{"build", "-color=false", "-on-error=abort", recipeFile}, Env: env, Dir: dir}, safeProgress, safeProgress)
	// Preserve task IDs written by the native worker even when Packer fails.
	workerUnknown := false
	if saved, se := e.Store.Load(r.OperationID); se == nil {
		r.Tasks = saved.Tasks
		r.ActiveTask = saved.ActiveTask
		r.ActiveTaskPhase = saved.ActiveTaskPhase
		r.SubmissionPending = saved.SubmissionPending
		workerUnknown = saved.Status == "unknown"
		if saved.Phase != "packer-build" {
			r.Phase = saved.Phase
		}
	}
	if er != nil {
		return e.Fail(r, fmt.Errorf("Packer build failed; inspect retained VM %d: %w", s.VMID, er), ctx.Err() != nil || workerUnknown)
	}
	proof, er := os.ReadFile(doneFile)
	if er != nil || strings.TrimSpace(string(proof)) != r.OperationID {
		return e.Fail(r, errors.New("native worker did not attest successful sealing; template is not accepted"), false)
	}
	cfg, er := e.Client.VMConfig(ctx, s.Node, s.VMID)
	if er != nil {
		return e.Fail(r, er, true)
	}
	if !cfg.Bool("template") {
		return e.Fail(r, errors.New("Packer returned without a template"), false)
	}
	if !strings.Contains(cfg.String("tags"), "build-"+r.OperationID) {
		return e.Fail(r, errors.New("template build ownership tag missing"), false)
	}
	r.Status = "succeeded"
	r.Phase = "complete"
	r.Completed = []string{"install", "prepare", "generalize", "shutdown", "template"}
	return r, e.Save(r)
}
func BuildKey() ([]byte, string, error) {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return nil, "", e
	}
	block, e := ssh.MarshalPrivateKey(key, "pvecli temporary build key")
	if e != nil {
		return nil, "", e
	}
	signer, e := ssh.NewSignerFromKey(key)
	if e != nil {
		return nil, "", e
	}
	return pem.EncodeToMemory(block), strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}
func hashFile(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func verifyFile(path, want string) error {
	got, e := hashFile(path)
	if e != nil {
		return e
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA-256 mismatch for %s", filepath.Base(path))
	}
	return nil
}
func ISOCommand(seed, out string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "oscdimg", []string{"-n", "-m", "-lCIDATA", seed, out}
	}
	return "xorriso", []string{"-as", "mkisofs", "-J", "-R", "-V", "CIDATA", "-o", out, seed}
}
func fileURL(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
func mediaBlock(m spec.Media, store string, index string) map[string]any {
	b := map[string]any{"type": "sata", "index": index, "unmount": true}
	if m.Volume != "" {
		b["iso_file"] = m.Volume
	} else {
		b["iso_url"] = fileURL(m.File)
		b["iso_checksum"] = "sha256:" + strings.ToLower(m.SHA256)
		b["iso_storage_pool"] = store
		// Stable upload names are necessary to identify and remove secret seed media.
		b["iso_target_path"] = filepath.ToSlash(m.File)
	}
	return b
}
func RenderPacker(s spec.Build, seed, seedHash, executable, config, worker string, timeout time.Duration) ([]byte, error) {
	seedBlock := mediaBlock(spec.Media{File: seed, SHA256: seedHash}, s.ISOStorage, "2")
	builder := map[string]any{"type": "proxmox-iso", "node": s.Node, "vm_id": s.VMID, "vm_name": s.Name, "template_name": s.Name, "pool": s.Pool, "communicator": "none", "insecure_skip_tls_verify": false, "task_timeout": "15m", "bios": "ovmf", "machine": "q35", "efi_config": map[string]any{"efi_storage_pool": s.Storage, "efi_type": "4m", "pre_enrolled_keys": false}, "cores": s.Hardware.Cores, "memory": s.Hardware.MemoryMiB, "scsi_controller": "virtio-scsi-single", "qemu_agent": true, "cloud_init": true, "cloud_init_storage_pool": s.Storage, "cloud_init_disk_type": "sata", "boot": "order=sata0;scsi0", "boot_iso": mediaBlock(s.ISO, s.ISOStorage, "0"), "disks": []any{map[string]any{"type": "scsi", "disk_size": strconv.Itoa(s.Hardware.DiskGiB) + "G", "storage_pool": s.Storage}}, "network_adapters": []any{map[string]any{"model": "virtio", "bridge": s.Bridge, "vlan_tag": strconv.Itoa(s.VLAN)}}, "template_description": fmt.Sprintf("pvecli recipe=%s os=%s user=pveadmin", RecipeVersion, s.OS)}
	builder["tags"] = "pvecli;build-" + filepath.Base(filepath.Dir(worker))
	// An empty boot disk falls through to the installer; after installation it
	// wins on reboot, preventing another unattended install from the ISO.
	builder["boot"] = "order=scsi0;sata0"
	if s.VLAN == 0 {
		builder["network_adapters"].([]any)[0].(map[string]any)["vlan_tag"] = ""
	}
	if s.Hardware.CPU != "" {
		builder["cpu_type"] = s.Hardware.CPU
	}
	if s.Hardware.Sockets > 0 {
		builder["sockets"] = s.Hardware.Sockets
	}
	if s.OS == "ubuntu-24.04" {
		builder["os"] = "l26"
		builder["boot_wait"] = "10s"
		builder["boot_command"] = []string{"<esc><wait>c<wait>", "linux /casper/vmlinuz autoinstall ds=nocloud ---<enter>", "initrd /casper/initrd<enter>", "boot<enter>"}
		builder["additional_iso_files"] = []any{seedBlock}
	} else {
		builder["os"] = "win11"
		builder["boot_wait"] = "2s"
		builder["boot_command"] = []string{strings.Repeat("<spacebar><wait1s>", 12)}
		builder["additional_iso_files"] = []any{mediaBlock(s.VirtIO, s.ISOStorage, "1"), seedBlock}
	}
	// The local provisioner invokes a binary/argv vector directly, ignoring its
	// required placeholder script. No shell interpolation is used for user paths.
	provisioner := map[string]any{"type": "shell-local", "inline": []string{"unused"}, "execute_command": []string{executable, "--config", config, "--timeout", timeout.String(), "_template-worker", "--file", worker}}
	return json.MarshalIndent(map[string]any{"builders": []any{builder}, "provisioners": []any{provisioner}}, "", "  ")
}
func WriteSeed(dir string, s spec.Build, key, password string) error {
	if s.OS == "ubuntu-24.04" {
		user := fmt.Sprintf("#cloud-config\nautoinstall:\n  version: 1\n  identity:\n    hostname: pvecli-build\n    username: pvebuild\n    password: '!'\n  ssh:\n    install-server: true\n    allow-pw: false\n    authorized-keys:\n      - %s\n  packages: [qemu-guest-agent, cloud-init, openssh-server]\n  storage:\n    layout:\n      name: direct\n  late-commands:\n    - curtin in-target --target=/target -- systemctl enable qemu-guest-agent\n    - \"echo 'pvebuild ALL=(ALL) NOPASSWD:ALL' > /target/etc/sudoers.d/99-pvecli-build\"\n    - chmod 0440 /target/etc/sudoers.d/99-pvecli-build\n", strconv.Quote(key))
		if e := os.WriteFile(filepath.Join(dir, "user-data"), []byte(user), 0600); e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(dir, "meta-data"), []byte("instance-id: pvecli-build\n"), 0600)
	}
	boot, e := assets.ReadFile("assets/windows-bootstrap.ps1")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "BOOTSTRAP.PS1"), boot, 0600); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "BUILDKEY.PUB"), []byte(key+"\n"), 0600); e != nil {
		return e
	}
	answer := WindowsAnswer(s, password)
	return os.WriteFile(filepath.Join(dir, "Autounattend.xml"), []byte(answer), 0600)
}
func escaped(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func encodedPS(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[2*i] = byte(v)
		b[2*i+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
func WindowsAnswer(s spec.Build, password string) string {
	drivers := ""
	i := 1
	version := "2k22"
	if s.OS == "windows-2025" {
		version = "2k25"
	}
	// Fixed SATA media ordering gives the VirtIO ISO drive E during Setup.
	for _, drive := range []string{"E"} {
		for _, driver := range []string{"vioscsi", "NetKVM"} {
			drivers += fmt.Sprintf(`<PathAndCredentials wcm:action="add" wcm:keyValue="%d"><Path>%s:\%s\%s\amd64</Path></PathAndCredentials>`, i, drive, driver, version)
			i++
		}
	}
	bootstrap := `$ErrorActionPreference='Stop'; $v=Get-Volume | Where-Object { $_.DriveLetter -and (Test-Path "$($_.DriveLetter):\BOOTSTRAP.PS1") } | Select-Object -First 1; if(-not $v){throw 'pvecli seed missing'}; & powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$($v.DriveLetter):\BOOTSTRAP.PS1"; if($LASTEXITCODE -ne 0){throw 'bootstrap failed'}`
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
<settings pass="windowsPE">
<component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS"><SetupUILanguage><UILanguage>en-US</UILanguage></SetupUILanguage><InputLocale>en-US</InputLocale><SystemLocale>en-US</SystemLocale><UILanguage>en-US</UILanguage><UserLocale>en-US</UserLocale></component>
<component name="Microsoft-Windows-PnpCustomizationsWinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS"><DriverPaths>%s</DriverPaths></component>
<component name="Microsoft-Windows-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
<DiskConfiguration><Disk wcm:action="add"><DiskID>0</DiskID><WillWipeDisk>true</WillWipeDisk><CreatePartitions><CreatePartition wcm:action="add"><Order>1</Order><Type>EFI</Type><Size>260</Size></CreatePartition><CreatePartition wcm:action="add"><Order>2</Order><Type>MSR</Type><Size>16</Size></CreatePartition><CreatePartition wcm:action="add"><Order>3</Order><Type>Primary</Type><Extend>true</Extend></CreatePartition></CreatePartitions><ModifyPartitions><ModifyPartition wcm:action="add"><Order>1</Order><PartitionID>1</PartitionID><Format>FAT32</Format><Label>System</Label></ModifyPartition><ModifyPartition wcm:action="add"><Order>2</Order><PartitionID>3</PartitionID><Format>NTFS</Format><Label>Windows</Label><Letter>C</Letter></ModifyPartition></ModifyPartitions></Disk><WillShowUI>OnError</WillShowUI></DiskConfiguration>
<ImageInstall><OSImage><InstallFrom><MetaData wcm:action="add"><Key>/IMAGE/NAME</Key><Value>%s</Value></MetaData></InstallFrom><InstallTo><DiskID>0</DiskID><PartitionID>3</PartitionID></InstallTo><WillShowUI>OnError</WillShowUI></OSImage></ImageInstall><UserData><AcceptEula>true</AcceptEula><FullName>pvecli</FullName><Organization>pvecli</Organization></UserData></component></settings>
<settings pass="specialize"><component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS"><ComputerName>pvecli-build</ComputerName><TimeZone>UTC</TimeZone></component></settings>
<settings pass="oobeSystem"><component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS"><OOBE><HideEULAPage>true</HideEULAPage><HideOnlineAccountScreens>true</HideOnlineAccountScreens><HideWirelessSetupInOOBE>true</HideWirelessSetupInOOBE><ProtectYourPC>3</ProtectYourPC></OOBE><UserAccounts><AdministratorPassword><Value>%s</Value><PlainText>true</PlainText></AdministratorPassword></UserAccounts><AutoLogon><Username>Administrator</Username><Password><Value>%s</Value><PlainText>true</PlainText></Password><Enabled>true</Enabled><LogonCount>1</LogonCount></AutoLogon><FirstLogonCommands><SynchronousCommand wcm:action="add"><Order>1</Order><Description>Bootstrap pvecli access</Description><CommandLine>powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand %s</CommandLine></SynchronousCommand></FirstLogonCommands></component></settings></unattend>
`, drivers, escaped(s.WindowsImage), escaped(password), escaped(password), encodedPS(bootstrap))
}
func renderAsset(name string, data any) ([]byte, error) {
	raw, e := assets.ReadFile("assets/" + name)
	if e != nil {
		return nil, e
	}
	t, e := template.New(name).Parse(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	if e != nil {
		return nil, e
	}
	var b bytes.Buffer
	e = t.Execute(&b, data)
	return b.Bytes(), e
}
func Worker(ctx context.Context, path, configPath string, hook func(*pve.Client)) error {
	raw, er := os.ReadFile(path)
	if er != nil {
		return er
	}
	var w WorkerInput
	if er = json.Unmarshal(raw, &w); er != nil {
		return er
	}
	if w.APIVersion != spec.Version || w.Context != w.Build.Context || !strings.Contains(filepath.Base(path), "worker") {
		return errors.New("invalid worker input")
	}
	if er = w.Build.Validate(); er != nil {
		return er
	}
	if filepath.Base(filepath.Dir(path)) != w.OperationID || filepath.Base(filepath.Dir(filepath.Dir(path))) != "builds" {
		return errors.New("worker input is outside its operation directory")
	}
	store := operation.Store{Dir: filepath.Dir(filepath.Dir(filepath.Dir(path)))}
	journal, er := store.Load(w.OperationID)
	if er != nil {
		return er
	}
	if journal.Kind != "template-build" || journal.VMID != w.Build.VMID || journal.ContextIdentity != w.ContextIdentity || journal.InputDigest != spec.Digest(w.Build) {
		return errors.New("worker journal identity mismatch")
	}
	// Restrict the worker's output/key paths to the protected operation directory.
	for _, p := range []string{w.KeyFile, w.DoneFile} {
		rel, e := filepath.Rel(filepath.Dir(path), p)
		if e != nil || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
			return errors.New("worker path escapes operation directory")
		}
	}
	var cfg spec.Config
	if er = spec.Read(configPath, &cfg); er != nil {
		return er
	}
	conn, _, er := cfg.Select(w.Context)
	if er != nil {
		return er
	}
	if spec.Digest(conn) != w.ContextIdentity {
		return errors.New("worker context identity changed")
	}
	secret := os.Getenv(conn.TokenSecretEnv)
	if secret == "" {
		return errors.New("worker API secret missing")
	}
	c := pve.New(conn.Endpoint, conn.TokenID, secret)
	if hook != nil {
		hook(c)
	}
	if _, er = c.CheckVersion(ctx); er != nil {
		return er
	}
	s := w.Build
	cfgVM, er := c.VMConfig(ctx, s.Node, s.VMID)
	if er != nil {
		return er
	}
	if cfgVM.Bool("template") || cfgVM.String("name") != s.Name || !containsTag(cfgVM.String("tags"), "build-"+w.OperationID) {
		return errors.New("worker target is not the requested build VM")
	}
	g := spec.Guest{OS: s.OS, User: "pvebuild", KeyFile: w.KeyFile, Port: 22}
	if strings.HasPrefix(s.OS, "windows") {
		g.User = "Administrator"
	}
	n := spec.Network{Interface: "net0", Bridge: s.Bridge, IPv4: "dhcp"}
	client, _, er := (guest.SSH{}).Connect(ctx, c, s.Node, s.VMID, n, g)
	if er != nil {
		return er
	}
	defer client.Close()
	data := map[string]string{"Operation": w.OperationID, "CloudbaseURL": strings.ReplaceAll(s.Cloudbase.URL, "'", "''"), "CloudbaseHash": strings.ToLower(s.Cloudbase.SHA256)}
	sealed := "/var/lib/pvecli/sealed"
	cmd := "sudo -n bash /tmp/pvecli-prepare.sh"
	remote := "/tmp/pvecli-prepare.sh"
	asset := "ubuntu-prepare.sh"
	if strings.HasPrefix(s.OS, "windows") {
		sealed = `C:\ProgramData\pvecli\sealed`
		remote = "C:/ProgramData/pvecli/prepare.ps1"
		asset = "windows-prepare.ps1"
		if _, er = guest.Command(ctx, client, `powershell.exe -NoProfile -Command "New-Item -ItemType Directory -Path C:\ProgramData\pvecli -Force | Out-Null"`); er != nil {
			return er
		}
		first, ee := assets.ReadFile("assets/windows-firstboot.ps1")
		if ee != nil {
			return ee
		}
		if er = guest.Upload(ctx, client, "C:/ProgramData/pvecli/firstboot.ps1", first); er != nil {
			return er
		}
		cmd = `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File C:\ProgramData\pvecli\prepare.ps1`
	}
	script, er := renderAsset(asset, data)
	if er != nil {
		return er
	}
	if er = guest.Upload(ctx, client, remote, script); er != nil {
		return er
	}
	out, er := guest.Command(ctx, client, cmd)
	if er != nil {
		return fmt.Errorf("guest preparation failed: %w (output withheld; inspect guest provisioning logs)", er)
	}
	if !strings.Contains(out, "pvecli-prepared") {
		return errors.New("guest did not complete preparation")
	}
	proof, er := c.AgentFile(ctx, s.Node, s.VMID, sealed)
	if er != nil || strings.TrimSpace(proof) != w.OperationID {
		return errors.New("guest sealing attestation missing")
	}
	coordinator := engine.Engine{Client: c, Store: store}
	if er = coordinator.Task(ctx, journal, "guest-shutdown", "POST", pve.Path(s.Node, s.VMID)+"/status/shutdown", nil); er != nil {
		_, er = coordinator.Fail(journal, er, engine.IsUnknown(er))
		return er
	}
	for {
		rs, ok, err := c.Find(ctx, s.VMID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("build VM disappeared")
		}
		if rs.Status == "stopped" {
			break
		}
		if er = pve.Delay(ctx, time.Second); er != nil {
			return er
		}
	}
	return os.WriteFile(w.DoneFile, []byte(w.OperationID+"\n"), 0600)
}
func cleanupSeed(ctx context.Context, c *pve.Client, node string, id int, operationID, volume string) error {
	cfg, err := c.VMConfig(ctx, node, id)
	if err == nil {
		if !containsTag(cfg.String("tags"), "build-"+operationID) {
			return errors.New("cannot prove build VM ownership for answer-media cleanup")
		}
		if cfg.String("lock") != "" {
			return errors.New("build VM is locked; reconcile its tasks before answer-media cleanup")
		}
		devices := []string{}
		for k, v := range cfg {
			if (strings.HasPrefix(k, "sata") || strings.HasPrefix(k, "ide") || strings.HasPrefix(k, "scsi")) && strings.Split(fmt.Sprint(v), ",")[0] == volume {
				devices = append(devices, k)
			}
		}
		if len(devices) > 0 {
			if !cfg.Bool("template") {
				rs, ok, er := c.Find(ctx, id)
				if er != nil {
					return er
				}
				if ok && rs.Status != "stopped" {
					task, er := c.Submit(ctx, "POST", pve.Path(node, id)+"/status/shutdown", nil)
					if er != nil {
						return er
					}
					if er = c.Wait(ctx, node, task); er != nil {
						return er
					}
				}
			}
			var task string
			er := c.Do(ctx, "POST", pve.Path(node, id)+"/config", url.Values{"delete": {strings.Join(devices, ",")}, "digest": {cfg.String("digest")}}, &task)
			if er != nil {
				return er
			}
			if strings.HasPrefix(task, "UPID:") {
				if er = c.Wait(ctx, node, task); er != nil {
					return er
				}
			}
		}
	} else {
		var api *pve.APIError
		if !errors.As(err, &api) || api.Status != 404 {
			return err
		}
	}
	store, _, _ := strings.Cut(volume, ":")
	var result any
	er := c.Do(ctx, "DELETE", "/nodes/"+url.PathEscape(node)+"/storage/"+url.PathEscape(store)+"/content/"+url.PathEscape(volume), nil, &result)
	var api *pve.APIError
	if errors.As(er, &api) && (api.Status == 404 || api.Status == 500 && strings.Contains(api.Message, "does not exist")) {
		return nil
	}
	if er != nil {
		return er
	}
	if task, ok := result.(string); ok && strings.HasPrefix(task, "UPID:") {
		return c.Wait(ctx, node, task)
	}
	return nil
}
func containsTag(tags, want string) bool {
	for _, tag := range strings.Split(tags, ";") {
		if tag == want {
			return true
		}
	}
	return false
}

type redactedWriter struct {
	mu      sync.Mutex
	out     io.Writer
	secrets []string
	buffer  []byte
}

func (w *redactedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer = append(w.buffer, p...)
	for {
		i := bytes.IndexByte(w.buffer, '\n')
		if i < 0 {
			break
		}
		w.emit(w.buffer[:i+1])
		w.buffer = w.buffer[i+1:]
	}
	// Discard an excessive unterminated line rather than risk splitting a secret.
	if len(w.buffer) > 1<<20 {
		w.buffer = nil
		_, _ = io.WriteString(w.out, "[oversized build output withheld]\n")
	}
	return len(p), nil
}
func (w *redactedWriter) emit(p []byte) {
	s := string(p)
	for _, secret := range w.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[redacted]")
		}
	}
	_, _ = io.WriteString(w.out, s)
}
func (w *redactedWriter) Flush() { w.mu.Lock(); defer w.mu.Unlock(); w.emit(w.buffer); w.buffer = nil }
