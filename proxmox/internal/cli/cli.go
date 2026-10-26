package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/engine"
	"github.com/jsirianni/dayz_config/proxmox/internal/guest"
	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/template"
	"github.com/jsirianni/dayz_config/proxmox/internal/tools"
	"github.com/jsirianni/dayz_config/proxmox/internal/vm"
	"github.com/spf13/cobra"
)

var Version = "dev"

type App struct {
	Out, Err                                io.Writer
	ConfigPath, Context, StateDir, ToolsDir string
	JSON                                    bool
	Timeout                                 time.Duration
	result                                  *operation.Result
	secrets                                 []string
	// ClientHook is used only by in-process tests to supply an httptest transport.
	ClientHook func(*pve.Client)
	Guest      guest.Checker
	Runner     tools.Runner
}

func (a *App) connect(ctx context.Context) (*engine.Engine, error) {
	var cfg spec.Config
	if err := spec.Read(a.ConfigPath, &cfg); err != nil {
		return nil, engine.Input(err)
	}
	c, name, err := cfg.Select(a.Context)
	if err != nil {
		return nil, engine.Input(err)
	}
	secret := os.Getenv(c.TokenSecretEnv)
	if secret == "" {
		return nil, engine.Input(errors.New("API token secret environment variable is unset"))
	}
	a.secrets = append(a.secrets, secret)
	client := pve.New(c.Endpoint, c.TokenID, secret)
	if a.ClientHook != nil {
		a.ClientHook(client)
	}
	if _, err = client.CheckVersion(ctx); err != nil {
		return nil, engine.Input(err)
	}
	g := a.Guest
	if g == nil {
		g = guest.SSH{}
	}
	return &engine.Engine{Client: client, Context: name, Identity: spec.Digest(c), Store: operation.Store{Dir: a.StateDir}, Guest: g, Progress: a.Err}, nil
}
func (a *App) redact(s string) string {
	for _, secret := range a.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[redacted]")
		}
	}
	return s
}
func (a *App) Execute(ctx context.Context, args []string) int {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if a.ConfigPath == "" {
		a.ConfigPath, _ = spec.DefaultConfigPath()
	}
	if a.StateDir == "" {
		a.StateDir, _ = spec.DefaultStateDir()
	}
	if a.ToolsDir == "" {
		a.ToolsDir, _ = spec.DefaultToolsDir()
	}
	for _, p := range []*string{&a.ConfigPath, &a.StateDir, &a.ToolsDir} {
		absolute, err := filepath.Abs(*p)
		if err != nil {
			fmt.Fprintln(a.Err, err)
			return 2
		}
		*p = absolute
	}
	root := a.command()
	wrapArgs(root)
	root.SetArgs(args)
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	err := root.ExecuteContext(ctx)
	code := 0
	if err != nil {
		code = 1
		if strings.HasPrefix(err.Error(), "unknown command ") {
			err = engine.Input(err)
		}
		var typed *engine.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			code = 3
		} else if errors.As(err, &typed) {
			code = typed.Code
		}
		if a.result == nil {
			a.result = &operation.Result{APIVersion: spec.Version, Status: "failed"}
		}
		a.result.Status = "failed"
		if code == 3 {
			a.result.Status = "unknown"
		}
		a.result.Error = a.redact(err.Error())
	}
	if a.result != nil {
		b, e := json.MarshalIndent(a.result, "", "  ")
		if e != nil {
			fmt.Fprintln(a.Err, e)
			return 1
		}
		b = []byte(a.redact(string(b)))
		if a.JSON {
			fmt.Fprintln(a.Out, string(b))
		} else {
			if a.result.OperationID != "" {
				fmt.Fprintf(a.Out, "Operation: %s\n", a.result.OperationID)
			}
			fmt.Fprintf(a.Out, "%s: %s\n", a.result.Status, a.result.Phase)
			if a.result.SSHEndpoint != "" {
				fmt.Fprintln(a.Out, "SSH:", a.result.SSHEndpoint)
			}
			fmt.Fprintln(a.Out, string(b))
		}
	} else if err != nil {
		fmt.Fprintln(a.Err, a.redact(err.Error()))
	}
	return code
}
func (a *App) command() *cobra.Command {
	r := &cobra.Command{Use: "pvecli", Short: "Independent Proxmox VE 9.x VM management", SilenceUsage: true, SilenceErrors: true}
	r.CompletionOptions.DisableDefaultCmd = true
	r.PersistentFlags().StringVar(&a.ConfigPath, "config", a.ConfigPath, "connection configuration file")
	r.PersistentFlags().StringVar(&a.Context, "context", a.Context, "named Proxmox context")
	r.PersistentFlags().StringVar(&a.StateDir, "state-dir", a.StateDir, "private operation journal directory")
	r.PersistentFlags().StringVar(&a.ToolsDir, "tools-dir", a.ToolsDir, "pinned build-tool cache")
	r.PersistentFlags().BoolVar(&a.JSON, "json", false, "emit one JSON result")
	r.PersistentFlags().DurationVar(&a.Timeout, "timeout", 50*time.Minute, "overall operation deadline")
	r.SetFlagErrorFunc(func(_ *cobra.Command, e error) error { return engine.Input(e) })
	r.AddCommand(&cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		a.result = &operation.Result{APIVersion: spec.Version, Status: "succeeded", Data: map[string]string{"pvecli": Version, "packer": tools.PackerVersion, "proxmox_plugin": tools.PluginVersion}}
		return nil
	}})
	for _, kind := range []string{"nodes", "storage", "networks"} {
		kind := kind
		group := &cobra.Command{Use: kind}
		var node string
		cmd := &cobra.Command{Use: "list", Args: cobra.NoArgs}
		cmd.Flags().StringVar(&node, "node", "", "node name (required for storage/networks)")
		cmd.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, _ []string) error {
			path := "/nodes"
			if kind != "nodes" {
				if !spec.Identifier(node) {
					return engine.Input(errors.New("--node is required"))
				}
				suffix := "storage"
				if kind == "networks" {
					suffix = "network"
				}
				path = "/nodes/" + url.PathEscape(node) + "/" + suffix
			}
			rows, err := e.Client.Rows(ctx, path)
			a.result = &operation.Result{APIVersion: spec.Version, Status: "succeeded", Data: rows}
			return err
		})
		group.AddCommand(cmd)
		r.AddCommand(group)
	}
	doctor := &cobra.Command{Use: "doctor", Args: cobra.NoArgs}
	var buildCheck bool
	doctor.Flags().BoolVar(&buildCheck, "template-build", false, "also check local Packer/plugin/ISO tools")
	doctor.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, _ []string) error {
		var perms any
		if err := e.Client.Do(ctx, "GET", "/access/permissions", nil, &perms); err != nil {
			return err
		}
		nodes, err := e.Client.Rows(ctx, "/nodes")
		if err != nil {
			return err
		}
		a.result = &operation.Result{APIVersion: spec.Version, Status: "succeeded", Context: e.Context, Data: map[string]any{"nodes": nodes, "permissions": perms, "tls": "verified OS trust", "template_build_checked": buildCheck}}
		if buildCheck {
			return engine.Input(a.toolManager().Check())
		}
		return nil
	})
	r.AddCommand(doctor)
	vms := &cobra.Command{Use: "vm"}
	templates := &cobra.Command{Use: "template"}
	r.AddCommand(vms, templates)
	for _, g := range []*cobra.Command{vms, templates} {
		isTemplate := g == templates
		list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, _ []string) error {
			rs, err := e.Client.Resources(ctx)
			if err != nil {
				return err
			}
			filtered := []pve.Resource{}
			for _, v := range rs {
				if v.Type == "qemu" && (v.Template == 1) == isTemplate {
					filtered = append(filtered, v)
				}
			}
			a.result = &operation.Result{APIVersion: spec.Version, Status: "succeeded", Data: filtered}
			return nil
		})}
		show := &cobra.Command{Use: "show <vmid>", Args: vmIDArgs, RunE: a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, args []string) error {
			id, _ := strconv.Atoi(args[0])
			var err error
			a.result, err = e.Show(ctx, id)
			if err == nil {
				data := a.result.Data.(map[string]any)
				if data["configuration"].(pve.Config).Bool("template") != isTemplate {
					return engine.Input(errors.New("VM/template command does not match resource type"))
				}
			}
			return err
		})}
		g.AddCommand(list, show)
	}
	var name, cpu string
	var cores, sockets, memory int
	var onboot, dry bool
	update := &cobra.Command{Use: "update <vmid>", Args: vmIDArgs}
	update.Flags().StringVar(&name, "name", "", "VM name")
	update.Flags().StringVar(&cpu, "cpu", "", "CPU model")
	update.Flags().IntVar(&cores, "cores", 0, "cores per socket")
	update.Flags().IntVar(&sockets, "sockets", 0, "socket count")
	update.Flags().IntVar(&memory, "memory-mib", 0, "memory in MiB")
	update.Flags().BoolVar(&onboot, "onboot", false, "start on host boot")
	update.Flags().BoolVar(&dry, "dry-run", false, "preview without writes")
	update.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, c *cobra.Command, args []string) error {
		id, _ := strconv.Atoi(args[0])
		_, cfg, err := e.Existing(ctx, id)
		if err != nil {
			return err
		}
		if (c.Flags().Changed("cores") && cores < 1) || (c.Flags().Changed("sockets") && sockets < 1) || (c.Flags().Changed("memory-mib") && memory < 128) {
			return engine.Input(errors.New("invalid CPU/memory amount"))
		}
		v, err := vm.HardwareChanges(cfg, spec.Hardware{CPU: cpu, Cores: cores, Sockets: sockets, MemoryMiB: memory})
		if err != nil {
			return engine.Input(err)
		}
		if c.Flags().Changed("name") {
			if !spec.Identifier(name) {
				return engine.Input(errors.New("invalid name"))
			}
			v.Set("name", name)
		}
		if c.Flags().Changed("onboot") {
			v.Set("onboot", boolString(onboot))
		}
		v = vm.Diff(cfg, v)
		a.result, err = e.Mutate(ctx, id, "update", v, "", "", "", dry)
		return err
	})
	vms.AddCommand(update)
	nic := &cobra.Command{Use: "nic"}
	var nicName, bridge, model, vlan string
	var enabled, nicDry bool
	nicUpdate := &cobra.Command{Use: "update <vmid>", Args: vmIDArgs}
	nicUpdate.Flags().StringVar(&nicName, "interface", "net0", "existing NIC")
	nicUpdate.Flags().StringVar(&bridge, "bridge", "", "bridge")
	nicUpdate.Flags().StringVar(&model, "model", "", "NIC model")
	nicUpdate.Flags().StringVar(&vlan, "vlan", "", "1-4094, or none")
	nicUpdate.Flags().BoolVar(&enabled, "enabled", true, "enable virtual link")
	nicUpdate.Flags().BoolVar(&nicDry, "dry-run", false, "preview without writes")
	nicUpdate.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, c *cobra.Command, args []string) error {
		id, _ := strconv.Atoi(args[0])
		_, cfg, err := e.Existing(ctx, id)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(nicName, "net") {
			return engine.Input(errors.New("invalid interface"))
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(nicName, "net")); err != nil {
			return engine.Input(errors.New("invalid interface"))
		}
		patch := map[string]string{}
		if c.Flags().Changed("bridge") {
			if !spec.Identifier(bridge) {
				return engine.Input(errors.New("invalid bridge"))
			}
			patch["bridge"] = bridge
		}
		if c.Flags().Changed("vlan") {
			if vlan == "none" {
				patch["tag"] = ""
			} else {
				n, err := strconv.Atoi(vlan)
				if err != nil || n < 1 || n > 4094 {
					return engine.Input(errors.New("VLAN must be 1-4094 or none"))
				}
				patch["tag"] = strconv.Itoa(n)
			}
		}
		if c.Flags().Changed("enabled") {
			patch["link_down"] = boolString(!enabled)
		}
		if model != "" && !strings.Contains("|virtio|e1000|rtl8139|vmxnet3|", "|"+model+"|") {
			return engine.Input(errors.New("unsupported NIC model"))
		}
		value, err := vm.MergeProperty(cfg.String(nicName), patch, model)
		if err != nil {
			return engine.Input(err)
		}
		a.result, err = e.Mutate(ctx, id, "nic-update", vm.Diff(cfg, url.Values{nicName: {value}}), "", "", "", nicDry)
		return err
	})
	nic.AddCommand(nicUpdate)
	vms.AddCommand(nic)
	disk := &cobra.Command{Use: "disk"}
	var diskName, size string
	var diskDry bool
	resize := &cobra.Command{Use: "resize <vmid>", Args: vmIDArgs}
	resize.Flags().StringVar(&diskName, "disk", "scsi0", "attached disk")
	resize.Flags().StringVar(&size, "size", "", "absolute target size, e.g. 100G")
	resize.Flags().BoolVar(&diskDry, "dry-run", false, "preview without writes")
	resize.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, args []string) error {
		id, _ := strconv.Atoi(args[0])
		var err error
		a.result, err = e.Mutate(ctx, id, "disk-resize", nil, "", diskName, size, diskDry)
		return err
	})
	disk.AddCommand(resize)
	vms.AddCommand(disk)
	for _, action := range []string{"start", "shutdown", "reboot", "stop"} {
		action := action
		var powerDry, force bool
		c := &cobra.Command{Use: action + " <vmid>", Args: vmIDArgs}
		c.Flags().BoolVar(&powerDry, "dry-run", false, "preview without writes")
		if action == "stop" {
			c.Flags().BoolVar(&force, "force", false, "acknowledge immediate hard power-off")
		}
		c.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, args []string) error {
			if action == "stop" && !force {
				return engine.Input(errors.New("hard stop requires --force"))
			}
			id, _ := strconv.Atoi(args[0])
			var err error
			a.result, err = e.Mutate(ctx, id, action, nil, action, "", "", powerDry)
			return err
		})
		vms.AddCommand(c)
	}
	var deployFile string
	var deployDry bool
	deploy := &cobra.Command{Use: "deploy", Args: cobra.NoArgs}
	deploy.Flags().StringVar(&deployFile, "file", "", "VMDeployment YAML")
	deploy.Flags().BoolVar(&deployDry, "dry-run", false, "preview without writes")
	deploy.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, _ []string) error {
		s, err := spec.LoadDeploy(deployFile)
		if err != nil {
			return engine.Input(err)
		}
		a.registerPassphrase(s.Guest)
		a.result, err = e.Deploy(ctx, s, deployDry)
		return err
	})
	vms.AddCommand(deploy)
	var readyFile string
	ready := &cobra.Command{Use: "ready <vmid>", Args: vmIDArgs}
	ready.Flags().StringVar(&readyFile, "file", "", "VMDeployment YAML describing guest access")
	ready.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, args []string) error {
		s, err := spec.LoadDeploy(readyFile)
		if err != nil {
			return engine.Input(err)
		}
		id, _ := strconv.Atoi(args[0])
		if s.Context != e.Context || s.VMID != id {
			return engine.Input(errors.New("readiness spec must match selected context/VMID"))
		}
		a.registerPassphrase(s.Guest)
		r, _, err := e.Existing(ctx, id)
		if err != nil {
			return err
		}
		access, err := e.Guest.Ready(ctx, e.Client, r.Node, id, s.Network, s.Guest)
		a.result = &operation.Result{APIVersion: spec.Version, Status: "succeeded", VMID: id, Context: e.Context, Node: r.Node, SSHEndpoint: access.Endpoint, HostKey: access.Fingerprint}
		return err
	})
	vms.AddCommand(ready)
	var buildFile string
	var buildDry bool
	build := &cobra.Command{Use: "build", Args: cobra.NoArgs}
	build.Flags().StringVar(&buildFile, "file", "", "TemplateBuild YAML")
	build.Flags().BoolVar(&buildDry, "dry-run", false, "preview without writes")
	build.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, _ []string) error {
		s, err := spec.LoadBuild(buildFile)
		if err != nil {
			return engine.Input(err)
		}
		builder := template.Builder{Engine: e, Tools: a.toolManager(), Runner: a.Runner, ConfigPath: a.ConfigPath, Progress: a.Err}
		a.result, err = builder.Build(ctx, s, buildDry)
		return err
	})
	templates.AddCommand(build)
	op := &cobra.Command{Use: "operation"}
	for _, action := range []string{"status", "resume"} {
		action := action
		c := &cobra.Command{Use: action + " <id>", Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return engine.Input(errors.New("one operation ID required"))
			}
			return nil
		}}
		c.RunE = a.withEngine(func(ctx context.Context, e *engine.Engine, _ *cobra.Command, args []string) error {
			var err error
			if action == "resume" {
				if r, er := e.Store.Load(args[0]); er == nil && r.Deploy != nil {
					a.registerPassphrase(r.Deploy.Guest)
				}
				a.result, err = e.Resume(ctx, args[0])
			} else {
				a.result, err = e.Status(ctx, args[0])
			}
			return err
		})
		op.AddCommand(c)
	}
	r.AddCommand(op)
	toolGroup := &cobra.Command{Use: "tools"}
	var toolsDry bool
	install := &cobra.Command{Use: "install", Args: cobra.NoArgs}
	install.Flags().BoolVar(&toolsDry, "dry-run", false, "show pinned tools without downloading")
	install.RunE = func(c *cobra.Command, _ []string) error {
		if a.Timeout <= 0 {
			return engine.Input(errors.New("timeout must be positive"))
		}
		ctx, cancel := context.WithTimeout(c.Context(), a.Timeout)
		defer cancel()
		a.result = &operation.Result{APIVersion: spec.Version, Status: "planned", Data: map[string]string{"packer": tools.PackerVersion, "proxmox_plugin": tools.PluginVersion, "directory": a.ToolsDir}}
		if toolsDry {
			return nil
		}
		if err := a.toolManager().Install(ctx, a.Err); err != nil {
			return err
		}
		a.result.Status = "succeeded"
		return nil
	}
	toolGroup.AddCommand(install)
	r.AddCommand(toolGroup)
	var workerFile string
	worker := &cobra.Command{Use: "_template-worker", Hidden: true, Args: cobra.NoArgs}
	worker.Flags().StringVar(&workerFile, "file", "", "protected worker input")
	worker.RunE = func(c *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(c.Context(), a.Timeout)
		defer cancel()
		return template.Worker(ctx, workerFile, a.ConfigPath, a.ClientHook)
	}
	r.AddCommand(worker)
	return r
}
func (a *App) toolManager() tools.Manager { return tools.Manager{Dir: a.ToolsDir, Runner: a.Runner} }
func (a *App) registerPassphrase(g spec.Guest) {
	if g.PassphraseEnv != "" {
		a.secrets = append(a.secrets, os.Getenv(g.PassphraseEnv))
	}
}
func (a *App) withEngine(f func(context.Context, *engine.Engine, *cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		if a.Timeout <= 0 {
			return engine.Input(errors.New("timeout must be positive"))
		}
		ctx, cancel := context.WithTimeout(c.Context(), a.Timeout)
		defer cancel()
		e, err := a.connect(ctx)
		if err != nil {
			return err
		}
		err = f(ctx, e, c, args)
		if err != nil && a.result != nil && a.result.Status == "succeeded" {
			a.result.Status = "failed"
		}
		return err
	}
}
func vmIDArgs(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return engine.Input(errors.New("one VMID required"))
	}
	id, e := strconv.Atoi(args[0])
	if e != nil || !spec.VMID(id) {
		return engine.Input(errors.New("VMID must be 100-999999999"))
	}
	return nil
}
func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
func wrapArgs(c *cobra.Command) {
	if c.Args != nil {
		old := c.Args
		c.Args = func(cmd *cobra.Command, args []string) error { return engine.Input(old(cmd, args)) }
	}
	for _, child := range c.Commands() {
		wrapArgs(child)
	}
}
