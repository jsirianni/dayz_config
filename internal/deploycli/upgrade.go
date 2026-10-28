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

func runInternalUpgrade(args []string) error {
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	request := flags.String("request", "", "protected upgrade request")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *request == "" || flags.NArg() != 0 {
		return errors.New("internal agent upgrade requires --request")
	}
	if args[0] == "agent-upgrade-coordinator" {
		return deploy.RunUpgradeCoordinator(*request)
	}
	if args[0] == "agent-upgrade-retire" {
		return deploy.RunUpgradeRetirement(*request)
	}
	return deploy.SubmitAgentUpgrade(context.Background(), *request, os.Stdout)
}

func runAgentUpgrade(args []string) error {
	if len(args) == 3 && args[2] == "--help" {
		usage()
		return nil
	}
	if len(args) < 3 {
		return errors.New("agent upgrade requires one server alias")
	}
	alias := args[2]
	if _, err := deploy.TargetFor(alias); err != nil {
		return err
	}
	flags := flag.NewFlagSet("agent upgrade", flag.ContinueOnError)
	repo := flags.String("repo", ".", "repository root")
	config := flags.String("config", "", "client configuration")
	dry := flags.Bool("dry-run", false, "read-only upgrade preview")
	noWait := flags.Bool("no-wait", false, "return after durable submission")
	asJSON := flags.Bool("json", false, "JSON output")
	operation := flags.String("operation", "", "reattach to an existing upgrade operation")
	timeout := flags.Duration("timeout", 10*time.Minute, "command timeout")
	if err := flags.Parse(args[3:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || *timeout <= 0 {
		return errors.New("invalid timeout or unexpected agent upgrade arguments")
	}
	if *operation != "" && !deploy.ValidID(*operation) {
		return errors.New("invalid agent upgrade operation ID")
	}
	if *dry && (*noWait || *operation != "") {
		return errors.New("--dry-run cannot accompany --no-wait or --operation")
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	if *config == "" {
		*config = filepath.Join(root, "deploy.yaml")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	var revision string
	if !*dry && *operation == "" {
		revision, err = deploy.Eligible(ctx, root)
		if err != nil {
			return declined{err}
		}
		client := deploy.CurrentAgentIdentity()
		if client.Dirty || client.Revision != revision {
			return declined{errors.New("upgrade client must be a clean build of freshly fetched origin/main; rebuild dzdeploy or use its matching package")}
		}
	}
	cfg, err := deploy.LoadClient(*config)
	if err != nil {
		return err
	}
	endpoint, ok := cfg.Targets[alias]
	if !ok {
		return fmt.Errorf("endpoint %s is not enrolled", alias)
	}
	var binary []byte
	var candidate deploy.AgentIdentity
	if *operation == "" {
		binary, candidate, err = deploy.LoadUpgradeCandidate(endpoint)
		if err != nil {
			return err
		}
		if !*dry {
			if err = deploy.ValidateUpgradeCandidate(candidate, revision); err != nil {
				return declined{err}
			}
		}
	}
	remote, err := deploy.Connect(ctx, endpoint)
	if err != nil {
		return err
	}
	defer remote.Close()
	if *operation != "" {
		if err = remote.ResumeAgentUpgradeSubmission(ctx, alias, *operation); err != nil {
			return err
		}
		if *noWait {
			result, err := remote.AgentUpgradeStatus(ctx, alias, *operation)
			if err != nil {
				return err
			}
			if err = emitUpgrade(result, *asJSON); err != nil {
				return err
			}
			if result.Status == "failed" {
				return reported{errors.New(result.Error)}
			}
			return nil
		}
		return waitUpgrade(ctx, remote, alias, *operation, *asJSON)
	}
	preview, err := remote.PreviewAgentUpgrade(ctx, alias, candidate)
	if err != nil {
		return err
	}
	if candidate.Dirty || len(candidate.Revision) != 40 {
		preview.Blocked = append(preview.Blocked, "candidate has no clean recorded Git revision")
	}
	if *dry {
		if err = emit(preview, *asJSON); err != nil {
			return err
		}
		if len(preview.Blocked) > 0 {
			return reported{errors.New("agent upgrade preview contains blockers")}
		}
		return nil
	}
	if len(preview.Blocked) > 0 {
		if err = emit(preview, *asJSON); err != nil {
			return err
		}
		return reported{errors.New("agent upgrade is blocked by preflight")}
	}
	if preview.Unchanged {
		return emit(map[string]any{"target": alias, "status": "unchanged", "agent": candidate, "game": preview.Game, "continuity": true}, *asJSON)
	}
	host, err := remote.UpgradeHost(alias)
	if err != nil {
		return err
	}
	latest, err := deploy.Eligible(ctx, root)
	if err != nil {
		return declined{err}
	}
	if latest != revision {
		return declined{errors.New("origin/main changed during agent upgrade preparation; retry")}
	}
	req := deploy.UpgradeRequest{Version: deploy.Protocol, ID: deploy.NewID(), Target: alias, CandidateSHA256: candidate.SHA256, Revision: revision, Host: host}
	if !*asJSON {
		fmt.Fprintf(os.Stderr, "Agent upgrade operation %s; reconnect with dzdeploy agent upgrade %s --operation %s\n", req.ID, alias, req.ID)
	}
	if err = remote.SubmitAgentUpgrade(ctx, req, binary); err != nil {
		return fmt.Errorf("%s (operation %s)", err, req.ID)
	}
	if *noWait {
		return emit(map[string]string{"target": alias, "operation": req.ID, "status": "submitted"}, *asJSON)
	}
	return waitUpgrade(ctx, remote, alias, req.ID, *asJSON)
}

func waitUpgrade(ctx context.Context, remote *deploy.Remote, alias, id string, asJSON bool) error {
	result, waitErr := remote.WaitAgentUpgrade(ctx, alias, id, func(result deploy.UpgradeResult) {
		if !asJSON {
			fmt.Fprintln(os.Stderr, result.Phase)
		}
	})
	if result.ID != "" {
		if err := emitUpgrade(result, asJSON); err != nil {
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

func emitUpgrade(result deploy.UpgradeResult, asJSON bool) error {
	if asJSON {
		return emit(result, true)
	}
	fmt.Printf("%s agent upgrade: %s (%s), operation %s\n", result.Target, result.Status, result.Phase, result.ID)
	if result.Candidate != nil {
		fmt.Printf("Agent revision: %s\nAgent SHA256: %s\n", result.Candidate.Revision, result.Candidate.SHA256)
	}
	fmt.Printf("Game PID: %d; continuity verified: %t; agent startup verification pending: %t\n", result.Game.PID, result.Continuity, result.StartupPending)
	if result.Recovery != "" {
		fmt.Println("Recovery:", result.Recovery)
	}
	if result.Error != "" {
		fmt.Println("Error:", result.Error)
	}
	return nil
}

// Operation evidence remains readable while the managed agent is offline.
func statusOperation(ctx context.Context, remote *deploy.Remote, alias, id string, asJSON bool) error {
	if !deploy.ValidID(id) {
		return errors.New("invalid operation ID")
	}
	var normal deploy.Result
	err := remote.ReadJSON("results/"+id+".json", &normal)
	if err == nil {
		if normal.Version != deploy.Protocol {
			return deploy.VersionError(normal.Version)
		}
		if normal.ID != id || normal.Target != alias {
			return errors.New("operation result identity mismatch")
		}
		if err = emit(normal, asJSON); err != nil {
			return err
		}
		if normal.Status == "failed" {
			return reported{errors.New(normal.Error)}
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	upgrade, err := remote.AgentUpgradeStatus(ctx, alias, id)
	if err != nil {
		return err
	}
	if err = emitUpgrade(upgrade, asJSON); err != nil {
		return err
	}
	if upgrade.Status == "failed" {
		return reported{errors.New(upgrade.Error)}
	}
	return nil
}
