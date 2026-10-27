package deploycli

import (
	"encoding/json"
	"fmt"
	"github.com/jsirianni/dayz_config/internal/deploy"
	"os"
)

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
		printDZSA(r.DZSA)
	case deploy.State:
		installed := "none"
		if r.Installed != nil {
			installed = r.Installed.Commit
		}
		fmt.Printf("%s: %s, paused=%t, PID=%d\nInstalled: %s\nLast managed startup: %s\nRestart required: %t\n", r.Target, r.Phase, r.Paused, r.PID, installed, r.RunningCommit, r.RestartRequired)
		if r.Error != "" {
			fmt.Println("Attention:", r.Error)
		}
		printDZSA(r.DZSA)
	case *deploy.DZSAReport:
		printDZSA(r)
	case deploy.Enrollment:
		fmt.Printf("%s enrollment: %s, operation %s\nRerun deploy to resume provisioning.\n", r.Target, r.Phase, r.ID)
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
func printDZSA(report *deploy.DZSAReport) {
	if report == nil {
		return
	}
	fmt.Printf("DZSA: detected %s, server %q, map %q, players %d/%d\n", report.Address, report.Server.Name, report.Server.Map, report.Server.Players, report.Server.MaxPlayers)
	fmt.Println("DZSA check:", report.CheckURL)
	for _, mod := range report.Server.Mods {
		fmt.Printf("  Mod: %q (Workshop %d)\n", mod.Name, mod.SteamWorkshopID)
	}
}
