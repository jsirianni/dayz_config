package deploycli

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

func Main(args []string) int {
	err := run(args)
	if err == nil {
		return 0
	}
	var shown reported
	if !errors.As(err, &shown) {
		if jsonRequested(args) {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintln(os.Stderr, "dzdeploy:", err)
		}
	}
	var reject declined
	if errors.As(err, &reject) {
		return 2
	}
	return 1
}
func jsonRequested(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "-json" {
			return true
		}
	}
	return false
}
func usage() {
	fmt.Println(`dzdeploy: DayZ configuration deployment over SSH/SFTP

  dzdeploy health <server|all> [--since 24h]
  dzdeploy history <server|all> [--since 24h]
  dzdeploy logs <server> [--kind console|updates|agent|rpt|script|crash] [--tail 200] [--follow]
  dzdeploy diagnose <server|all> [--since 24h] [--output directory]
  dzdeploy backup create <server> [--stop] [--dry-run] [--no-wait]
  dzdeploy backup list <server>
  dzdeploy backup verify <server> --backup ID
  dzdeploy backup download <server> --backup ID --output file
  dzdeploy backup restore <server> --backup ID [--stop] [--dry-run] [--no-wait]
  dzdeploy doctor <server>
  dzdeploy check <server>
  dzdeploy deploy <server> [--dry-run] [--restart] [--no-wait]
  dzdeploy status <server> [--operation <id>]
  dzdeploy restart <server> [--hard] [--dry-run] [--no-wait]
  dzdeploy rollback <server> --deployment <id> [--restart]
  dzdeploy validate <server|all> [--offline]
  dzdeploy tools install
  dzdeploy agent upgrade <server> [--dry-run] [--no-wait] [--operation ID]
  dzdeploy bootstrap <server> [--account <account>] [--setup-ssh]
  dzdeploy agent --host-config <file> [--console]

Servers: nasdara, nasdara-staging, deer-isle
Client flags: --repo <directory> --config <file> --timeout 50m --json
First enrollment: deploy <server> (automatically trusts and saves the first SSH host key)
Client config defaults to <repo>/deploy.yaml. No deployment runs on merge.`)
}
func run(args []string) error {
	if len(args) > 0 && (args[0] == "agent-upgrade-submit" || args[0] == "agent-upgrade-coordinator" || args[0] == "agent-upgrade-retire") {
		return runInternalUpgrade(args)
	}
	if len(args) > 1 && args[0] == "agent" && args[1] == "upgrade" {
		return runAgentUpgrade(args)
	}
	if len(args) > 1 && args[1] == "--help" {
		usage()
		return nil
	}
	if len(args) > 0 && args[0] == "backup" {
		return runBackup(args)
	}
	if len(args) > 0 && (args[0] == "logs" || args[0] == "diagnose") {
		return runLogs(args)
	}
	if len(args) > 0 && (args[0] == "health" || args[0] == "history") {
		return runInspection(args)
	}
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
		identity := deploy.CurrentAgentIdentity()
		if jsonRequested(args) {
			return emit(identity, true)
		}
		fmt.Printf("dzdeploy protocol %d revision %s dirty=%t %s/%s\n", deploy.Protocol, identity.Revision, identity.Dirty, identity.GOOS, identity.GOARCH)
		return nil
	}
	alias := ""
	if action != "agent" && action != "enroll" && action != "steam-auth" {
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
	hard := flags.Bool("hard", false, "explicitly terminate only the identified managed game for a requested restart")
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
	hostKey := flags.String("host-key", "", "optional SHA256 SSH host fingerprint to require on first enrollment")
	enrollRequest := flags.String("request", "", "internal host enrollment request")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *hostFile != "" && action != "agent" && action != "steam-auth" {
		return errors.New("--host-config is internal to the installed agent; configure bootstrap using deploy.yaml")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if action == "status" && *operation != "" && !deploy.ValidID(*operation) {
		return errors.New("invalid operation ID")
	}
	if *offline && action != "validate" {
		return errors.New("--offline is only permitted for local validation")
	}
	if *hard && action != "restart" && !((action == "deploy" || action == "rollback") && *restart) {
		return errors.New("--hard requires restart or deploy/rollback --restart")
	}
	if *dry && action != "deploy" && !(*hard && (action == "restart" || action == "rollback")) {
		return errors.New("--dry-run is only valid for deploy")
	}
	if action == "check" && (*restart || *noWait || *operation != "" || *deployment != "") {
		return errors.New("check is read-only; restart, submission, and operation flags are not supported")
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
	case "enroll":
		if *enrollRequest == "" {
			return errors.New("internal enrollment requires --request")
		}
		return deploy.RunEnrollment(ctx, *enrollRequest, os.Stdout)
	case "steam-auth":
		if *hostFile == "" {
			return errors.New("authentication relay requires --host-config")
		}
		c, err := deploy.LoadHost(*hostFile)
		if err != nil {
			return err
		}
		return deploy.RelaySteamAuth(ctx, c, os.Stdin, os.Stdout)
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
	case "doctor", "check", "deploy", "status", "restart", "rollback":
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
	connect := deploy.ConnectPinned
	if action == "deploy" {
		connect = deploy.ConnectFirstUse
	}
	remote, err := connect(ctx, endpoint, *hostKey, action == "deploy" && !*dry)
	if err != nil {
		return err
	}
	defer remote.Close()
	if action == "status" && *operation != "" {
		return statusOperation(ctx, remote, alias, *operation, *asJSON)
	}
	if action == "check" {
		report, err := remote.CheckDZSA(ctx, alias)
		if err != nil {
			return err
		}
		return emit(report, *asJSON)
	}
	if action == "deploy" {
		needed, err := remote.EnrollmentNeeded(alias)
		if err != nil {
			return err
		}
		if needed {
			if err = enrollmentAccount(endpoint.Host, *dry, deploy.ReadInput); err != nil {
				return err
			}
			if err = deploy.CheckEnrollment(endpoint, alias); err != nil {
				return err
			}
			if *dry {
				preview := deploy.NewHostPreview(bundle, *restart)
				preview.Warnings = validation.Warnings
				return emit(preview, *asJSON)
			}
			fmt.Fprintln(os.Stderr, "First deployment: preparing the Windows service and prerequisites...")
			if err = remote.Enroll(ctx, bundle, os.Stderr); err != nil {
				return err
			}
			readyCtx, cancel := context.WithTimeout(ctx, time.Minute)
			err = remote.WaitReady(readyCtx, alias)
			cancel()
			if err != nil {
				return err
			}
		}
		if !*dry && (needed || *restart) {
			if err = remote.Authenticate(ctx, promptSteam, os.Stderr); err != nil {
				return err
			}
		}
	}
	state, err := remote.State(alias)
	if err != nil {
		if action == "status" && *operation == "" && os.IsNotExist(err) {
			var enrollment deploy.Enrollment
			if readErr := remote.ReadJSON("enrollment.json", &enrollment); readErr == nil && enrollment.Version == deploy.Protocol && enrollment.Target == alias && deploy.ValidID(enrollment.ID) {
				return emit(enrollment, *asJSON)
			}
		}
		return err
	}
	if *hard && !(*dry && action == "deploy") {
		if *dry {
			preview, e := remote.PreviewMaintenance(ctx, alias)
			if e != nil {
				return e
			}
			return emit(preview, *asJSON)
		}
		if err = remote.RequireFeature(alias, deploy.HardStopCapability); err != nil {
			return err
		}
	}
	if !*dry && (action == "restart" || (action == "rollback" && *restart)) {
		if err = remote.Authenticate(ctx, promptSteam, os.Stderr); err != nil {
			return err
		}
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
		report := deploy.AgentStatus{State: state}
		var heartbeat deploy.Heartbeat
		if remote.ReadJSON("heartbeat.json", &heartbeat) == nil && heartbeat.Version == deploy.Protocol {
			report.Agent = heartbeat.Agent
			report.AgentHeartbeat = heartbeat.Updated
			report.AgentFresh = !heartbeat.Updated.IsZero() && time.Since(heartbeat.Updated) <= 15*time.Second && time.Until(heartbeat.Updated) <= 15*time.Second
		}
		return emit(report, *asJSON)
	case "deploy":
		if *dry {
			preview, err := remote.Preview(bundle, *restart)
			if err != nil {
				return err
			}
			preview.Warnings = validation.Warnings
			if *hard {
				maintenance, e := remote.PreviewMaintenance(ctx, alias)
				if e != nil {
					return e
				}
				return emit(map[string]any{"changes": preview, "maintenance": maintenance}, *asJSON)
			}
			return emit(preview, *asJSON)
		}
	}
	req := deploy.Request{Version: deploy.Protocol, ID: deploy.NewID(), Target: alias, Action: action, Deployment: *deployment, Restart: *restart, Hard: *hard}
	if *hard {
		req.ExpectedPID = state.PID
		req.ExpectedToken = state.ProcessToken
		if state.Installed != nil {
			req.ExpectedRelease = state.Installed.ID
		}
	}
	if bundle != nil {
		latest, err := deploy.Eligible(ctx, root)
		if err != nil {
			return declined{err}
		}
		if latest != bundle.Manifest.Commit {
			return declined{errors.New("origin/main changed during enrollment/authentication; retry deployment")}
		}
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

func enrollmentAccount(host *deploy.HostConfig, dry bool, prompt func(string) (string, error)) error {
	if host == nil || (host.SteamUser != "" && !strings.HasPrefix(host.SteamUser, "REPLACE_")) {
		return nil
	}
	if dry {
		host.SteamUser = "PROMPT_DURING_DEPLOY"
		return nil
	}
	account, err := prompt("Steam account name for DayZ downloads: ")
	if err != nil {
		return err
	}
	account = strings.TrimSpace(account)
	if account == "" || strings.HasPrefix(account, "REPLACE_") || strings.ContainsAny(account, " \t\r\n\x00") {
		return errors.New("a Steam account name is required")
	}
	host.SteamUser = account
	return nil
}

func promptSteam(f deploy.AuthFrame) (string, error) {
	switch f.Type {
	case "password":
		return deploy.ReadSecret("Steam password: ")
	case "guard":
		return deploy.ReadSecret("Steam Guard / two-factor code: ")
	case "approval":
		fmt.Fprintln(os.Stderr, "Approve the Steam sign-in request in your Steam mobile app.")
	case "status":
		fmt.Fprintln(os.Stderr, "Checking Steam authentication under the service identity...")
	}
	return "", nil
}
