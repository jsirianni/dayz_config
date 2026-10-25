package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

type declined struct{ error }
type reported struct{ error }

func main() {
	err := run(os.Args[1:])
	if err != nil {
		var shown reported
		if errors.As(err, &shown) {
			// A structured result has already been emitted; keep JSON output singular.
		} else if jsonRequested(os.Args[1:]) {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintln(os.Stderr, "dzdeploy:", err)
		}
		var reject declined
		if errors.As(err, &reject) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
func jsonRequested(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "-json" {
			return true
		}
	}
	return false
}
func emit(v any, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch r := v.(type) {
	case deploy.Result:
		fmt.Printf("%s: %s (%s), operation %s, commit %s\n", r.Target, r.Status, r.Phase, r.ID, r.Commit)
		if r.RestartRequired {
			fmt.Println("Configuration installed; restart required to activate startup settings.")
		}
		if r.Recovery != "" {
			fmt.Println(r.Recovery)
		}
		if r.Error != "" {
			fmt.Println("Error:", r.Error)
		}
	case deploy.State:
		installed := "none"
		if r.Installed != nil {
			installed = r.Installed.Commit
		}
		fmt.Printf("%s: %s, paused=%t, PID=%d\nInstalled: %s\nLast managed startup: %s\nRestart required: %t\n", r.Target, r.Phase, r.Paused, r.PID, installed, r.RunningCommit, r.RestartRequired)
		if r.Error != "" {
			fmt.Println("Attention:", r.Error)
		}
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
func usage() {
	fmt.Println(`dzdeploy: DayZ configuration deployment over SSH/SFTP

  dzdeploy doctor <server>
  dzdeploy deploy <server> [--dry-run] [--restart] [--no-wait]
  dzdeploy status <server> [--operation <id>]
  dzdeploy restart <server> [--no-wait]
  dzdeploy rollback <server> --deployment <id> [--restart]
  dzdeploy validate <server|all> [--offline]
  dzdeploy tools install
  dzdeploy bootstrap <server> [--account <account>] [--setup-ssh]
  dzdeploy agent --host-config <file> [--console]

Servers: nasdara, nasdara-staging, deer-isle
Client flags: --repo <directory> --config <file> --timeout 50m --json
Client config defaults to <repo>/deploy.yaml. No deployment runs on merge.`)
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage()
		return nil
	}
	action := args[0]
	args = args[1:]
	if len(args) > 0 && args[0] == "--help" {
		usage()
		return nil
	}
	if action == "version" {
		fmt.Printf("dzdeploy protocol %d\n", deploy.Protocol)
		return nil
	}
	alias := ""
	if action != "agent" {
		if len(args) == 0 {
			return errors.New("server or tools subcommand required")
		}
		alias = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository root")
	configFile := flags.String("config", "", "client config")
	asJSON := flags.Bool("json", false, "JSON results")
	dry := flags.Bool("dry-run", false, "read-only preview")
	restart := flags.Bool("restart", false, "restart immediately after deployment")
	noWait := flags.Bool("no-wait", false, "return after durable submission")
	offline := flags.Bool("offline", false, "local validation using cached dzcli")
	operation := flags.String("operation", "", "operation ID for status")
	deployment := flags.String("deployment", "", "retained deployment ID for rollback")
	timeout := flags.Duration("timeout", 50*time.Minute, "operation timeout")
	hostFile := flags.String("host-config", "", "host configuration file")
	console := flags.Bool("console", false, "run agent in foreground for diagnostics")
	account := flags.String("account", "", "existing service/SSH account")
	passwordEnv := flags.String("password-env", "", "environment variable holding service account password")
	setupSSH := flags.Bool("setup-ssh", false, "install/configure Windows OpenSSH")
	publicKey := flags.String("public-key", "", "deployment public key file")
	sshSources := flags.String("ssh-sources", "", "comma-separated SSH source IPs/CIDRs")
	sshPort := flags.Int("ssh-port", 22, "host SSH port")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *hostFile != "" && action != "agent" {
		return errors.New("--host-config is internal to the installed agent; configure bootstrap using deploy.yaml")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if *offline && action != "validate" {
		return errors.New("--offline is only permitted for local validation")
	}
	if *dry && action != "deploy" {
		return errors.New("--dry-run is only valid for deploy")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, *timeout)
	defer cancelTimeout()
	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	if *configFile == "" {
		*configFile = filepath.Join(root, "deploy.yaml")
	}
	switch action {
	case "agent":
		if *hostFile == "" {
			return errors.New("--host-config is required")
		}
		c, err := deploy.LoadHost(*hostFile)
		if err != nil {
			return err
		}
		return deploy.RunService(c, *console)
	case "bootstrap":
		cfg, err := deploy.LoadClient(*configFile)
		if err != nil {
			return err
		}
		endpoint, ok := cfg.Targets[alias]
		if !ok || endpoint.Host == nil {
			return errors.New("deploy.yaml requires this server's host settings")
		}
		c := *endpoint.Host
		if *account == "" {
			*account = endpoint.User
			if !strings.ContainsAny(*account, `\@`) {
				*account = `.\` + *account
			}
		}
		if *publicKey == "" {
			*publicKey = endpoint.PublicKeyFile
		}
		if *sshSources == "" {
			*sshSources = strings.Join(endpoint.SSHSources, ",")
		}
		portExplicit := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "ssh-port" {
				portExplicit = true
			}
		})
		if !portExplicit {
			_, port, err := net.SplitHostPort(endpoint.Address)
			if err != nil {
				return fmt.Errorf("configure address before bootstrap: %w", err)
			}
			*sshPort, err = strconv.Atoi(port)
			if err != nil {
				return err
			}
		}
		info, err := deploy.Bootstrap(ctx, deploy.BootstrapOptions{Config: c, Account: *account, PasswordEnv: *passwordEnv, InstallSSH: *setupSSH, PublicKeyFile: *publicKey, SSHSources: *sshSources, SSHPort: *sshPort})
		if err != nil {
			return err
		}
		return emit(info, *asJSON)
	case "tools":
		if alias != "install" {
			return errors.New("tools supports install")
		}
		p, err := deploy.EnsureDz(ctx, root, false)
		if err != nil {
			return err
		}
		return emit(map[string]string{"dzcli": p}, *asJSON)
	case "validate":
		targets, err := deploy.Targets(nil)
		if err != nil {
			return err
		}
		if alias != "all" {
			t, err := deploy.TargetFor(alias)
			if err != nil {
				return err
			}
			targets = map[string]deploy.Target{alias: t}
		}
		dz, err := deploy.EnsureDz(ctx, root, *offline)
		if err != nil {
			return err
		}
		results := map[string]deploy.Validation{}
		var failures []error
		for name, t := range targets {
			r, err := deploy.Validate(ctx, dz, filepath.Join(root, "servers", "current", t.Folder))
			results[name] = r
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", name, err))
			}
		}
		if err = emit(results, *asJSON); err != nil {
			return err
		}
		if err = errors.Join(failures...); err != nil {
			return reported{err}
		}
		return nil
	case "doctor", "deploy", "status", "restart", "rollback":
	default:
		return fmt.Errorf("unknown command %q", action)
	}
	if _, err = deploy.TargetFor(alias); err != nil {
		return err
	}
	var bundle *deploy.Bundle
	var raw []byte
	var validation deploy.Validation
	if action == "deploy" {
		commit, err := deploy.Eligible(ctx, root)
		if err != nil {
			return declined{err}
		}
		bundle, err = deploy.Build(ctx, root, alias, commit)
		if err != nil {
			return err
		}
		dz, err := deploy.EnsureDz(ctx, root, false)
		if err != nil {
			return err
		}
		snapshot := filepath.Join(root, ".tools", "dzdeploy", "build", bundle.Manifest.ID)
		if err = bundle.Extract(snapshot); err != nil {
			return err
		}
		validation, err = deploy.Validate(ctx, dz, snapshot)
		if err != nil {
			_ = emit(validation, *asJSON)
			return reported{err}
		}
		raw, err = bundle.Encode()
		if err != nil {
			return err
		}
		// Catch edits or a moving remote main while validation was in progress.
		latest, err := deploy.Eligible(ctx, root)
		if err != nil {
			return declined{err}
		}
		if latest != commit {
			return declined{errors.New("origin/main changed during packaging; retry deployment")}
		}
	}
	if action == "rollback" && !deploy.ValidID(*deployment) {
		return errors.New("rollback requires --deployment <retained-id>")
	}
	cfg, err := deploy.LoadClient(*configFile)
	if err != nil {
		return fmt.Errorf("load local endpoint config (see docs/deployment.md): %w", err)
	}
	endpoint, ok := cfg.Targets[alias]
	if !ok {
		return fmt.Errorf("endpoint %s is not enrolled", alias)
	}
	remote, err := deploy.Connect(ctx, endpoint)
	if err != nil {
		return err
	}
	defer remote.Close()
	state, err := remote.State(alias)
	if err != nil {
		return err
	}
	switch action {
	case "doctor":
		var host deploy.HostConfig
		if err = remote.ReadJSON("host.json", &host); err != nil {
			return err
		}
		if err = host.Validate(); err != nil {
			return err
		}
		if host.Target != alias {
			return errors.New("host config target mismatch")
		}
		var heartbeat struct {
			Version int       `json:"version"`
			Updated time.Time `json:"updated"`
		}
		if err = remote.ReadJSON("heartbeat.json", &heartbeat); err != nil {
			return err
		}
		if heartbeat.Version != deploy.Protocol {
			return deploy.VersionError(heartbeat.Version)
		}
		if time.Since(heartbeat.Updated) > 15*time.Second || time.Until(heartbeat.Updated) > 15*time.Second {
			return errors.New("agent heartbeat is stale or clocks differ; check Windows service and time synchronization")
		}
		if _, err = deploy.Eligible(ctx, root); err != nil {
			return declined{err}
		}
		if err = emit(map[string]any{"target": alias, "ssh": "verified", "sftp": "available", "agent": "running", "protocol": deploy.Protocol, "git": "clean and current on main", "state": state}, *asJSON); err != nil {
			return err
		}
		if state.Error != "" {
			return errors.New("host reports an error; inspect status and protected host logs")
		}
		return nil
	case "status":
		if *operation != "" {
			if !deploy.ValidID(*operation) {
				return errors.New("invalid operation ID")
			}
			var result deploy.Result
			if err = remote.ReadJSON("results/"+*operation+".json", &result); err != nil {
				return err
			}
			if result.Version != deploy.Protocol {
				return deploy.VersionError(result.Version)
			}
			if err = emit(result, *asJSON); err != nil {
				return err
			}
			if result.Status == "failed" {
				return reported{errors.New(result.Error)}
			}
			return nil
		}
		return emit(state, *asJSON)
	case "deploy":
		if *dry {
			preview, err := remote.Preview(bundle, *restart)
			if err != nil {
				return err
			}
			preview.Warnings = validation.Warnings
			return emit(preview, *asJSON)
		}
	}
	req := deploy.Request{Version: deploy.Protocol, ID: deploy.NewID(), Target: alias, Action: action, Deployment: *deployment, Restart: *restart}
	if bundle != nil {
		req.ID = bundle.Manifest.ID
		req.PackageSHA256 = deploy.Digest(raw)
	}
	if err = remote.Submit(req, raw); err != nil {
		return err
	}
	if *noWait {
		return emit(map[string]string{"operation": req.ID, "status": "submitted", "target": alias}, *asJSON)
	}
	if !*asJSON {
		fmt.Printf("Submitted %s. Reconnect with: dzdeploy status %s --operation %s\n", req.ID, alias, req.ID)
		if validation.Warnings > 0 {
			fmt.Printf("Validation warnings: %d\n", validation.Warnings)
		}
	}
	result, waitErr := remote.Wait(ctx, req.ID, func(r deploy.Result) {
		if !*asJSON {
			fmt.Fprintln(os.Stderr, r.Phase)
		}
	})
	if waitErr != nil && result.ID != "" && result.Error == "" {
		result.Error = waitErr.Error()
	}
	if result.ID != "" {
		if err = emit(result, *asJSON); err != nil {
			return err
		}
	}
	if waitErr != nil {
		if result.ID != "" {
			return reported{fmt.Errorf("operation %s: %w", req.ID, waitErr)}
		}
		return fmt.Errorf("operation %s: %w", req.ID, waitErr)
	}
	return nil
}
