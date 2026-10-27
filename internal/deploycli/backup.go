package deploycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func runBackup(args []string) error {
	if len(args) == 3 && args[2] == "--help" {
		usage()
		return nil
	}
	if len(args) < 3 {
		return errors.New("backup requires create/list/verify/download/restore and one server alias")
	}
	action, alias := args[1], args[2]
	switch action {
	case "create", "list", "verify", "download", "restore":
	default:
		return errors.New("unknown backup subcommand")
	}
	if _, err := deploy.TargetFor(alias); err != nil {
		return err
	}
	f := flag.NewFlagSet("backup "+action, flag.ContinueOnError)
	config := f.String("config", "", "client configuration")
	repo := f.String("repo", ".", "repository root")
	id := f.String("backup", "", "backup ID")
	output := f.String("output", "", "local download file")
	stopGame := f.Bool("stop", false, "authorize guarded hard stop for consistent maintenance")
	dry := f.Bool("dry-run", false, "read-only maintenance preview")
	noWait := f.Bool("no-wait", false, "return after durable submission")
	asJSON := f.Bool("json", false, "JSON output")
	timeout := f.Duration("timeout", 50*time.Minute, "command timeout")
	if err := f.Parse(args[3:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	mutate := action == "create" || action == "restore"
	if f.NArg() != 0 || *timeout <= 0 {
		return errors.New("invalid timeout or unexpected backup arguments")
	}
	if !mutate && (*stopGame || *dry || *noWait) {
		return errors.New("maintenance flags require backup create/restore")
	}
	if (action == "verify" || action == "download" || action == "restore") && !deploy.ValidID(*id) {
		return errors.New("a valid --backup ID is required")
	}
	if (action == "create" || action == "list") && *id != "" {
		return errors.New("--backup is not valid for create/list")
	}
	if action == "download" && *output == "" {
		return errors.New("download requires --output")
	}
	if action != "download" && *output != "" {
		return errors.New("--output is only valid for download")
	}
	if *dry && *noWait {
		return errors.New("--no-wait cannot accompany --dry-run")
	}
	if *config == "" {
		*config = filepath.Join(*repo, "deploy.yaml")
	}
	cfg, err := deploy.LoadClient(*config)
	if err != nil {
		return err
	}
	ctx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt)
	defer shutdown()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	r, err := deploy.Connect(ctx, cfg.Targets[alias])
	if err != nil {
		return err
	}
	defer r.Close()
	switch action {
	case "list":
		backups, e := r.ListBackups(alias)
		if e != nil {
			return e
		}
		if *asJSON {
			return emit(backups, true)
		}
		for _, b := range backups {
			fmt.Printf("%s %s %d bytes revision=%s safety=%t\n", b.ID, b.Created.Format(time.RFC3339), b.Bytes, b.Manifest.Revision, b.Safety)
		}
		return nil
	case "verify":
		receipt, e := r.VerifyBackup(ctx, alias, *id)
		if e != nil {
			return e
		}
		if *asJSON {
			return emit(receipt, true)
		}
		fmt.Println("Verified backup:", receipt.ID)
		return nil
	case "download":
		receipt, e := r.DownloadBackup(ctx, alias, *id, *output)
		if e != nil {
			return e
		}
		return emit(map[string]any{"backup": receipt.ID, "file": *output, "sha256": receipt.SHA256, "verified": true}, *asJSON)
	}
	state, err := r.State(alias)
	if err != nil {
		return err
	}
	var source deploy.BackupReceipt
	if action == "restore" {
		source, err = r.VerifyBackup(ctx, alias, *id)
		if err != nil {
			return err
		}
	}
	if *dry {
		preview, e := r.PreviewMaintenance(ctx, alias)
		if e != nil {
			return e
		}
		preview["action"] = "backup-" + action
		preview["stopAuthorized"] = *stopGame
		preview["sourceBackup"] = *id
		preview["preserveManagedConfiguration"] = true
		preview["updateSteam"] = false
		if e = r.RequireFeature(alias, deploy.BackupCapability); errors.Is(e, deploy.ErrUnsupportedFeature) {
			preview["upgradeRequired"] = true
		} else if e != nil {
			return e
		}
		return emit(preview, *asJSON)
	}
	req := deploy.Request{Version: deploy.Protocol, ID: deploy.NewID(), Target: alias, Action: "backup-" + action, Backup: *id, BackupSafety: source.Safety, Stop: *stopGame, ExpectedPID: state.PID, ExpectedToken: state.ProcessToken}
	if state.Installed != nil {
		req.ExpectedRelease = state.Installed.ID
	}
	if err = r.Submit(req, nil); err != nil {
		return err
	}
	if *noWait {
		return emit(map[string]string{"target": alias, "operation": req.ID, "status": "submitted"}, *asJSON)
	}
	if !*asJSON {
		fmt.Fprintf(os.Stderr, "Submitted %s; reconnect with dzdeploy status %s --operation %s\n", req.ID, alias, req.ID)
	}
	result, waitErr := r.Wait(ctx, req.ID, func(result deploy.Result) {
		if !*asJSON {
			fmt.Fprintln(os.Stderr, result.Phase)
		}
	})
	if result.ID != "" {
		if err = emit(result, *asJSON); err != nil {
			return err
		}
	}
	if waitErr != nil {
		if result.ID != "" {
			return reported{waitErr}
		}
		return waitErr
	}
	return nil
}
