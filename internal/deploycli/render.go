package deploycli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func renderHealth(destination io.Writer, reports []deploy.HealthReport) error {
	var buffer bytes.Buffer
	w := tabwriter.NewWriter(&buffer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SERVER\tHEALTH\tUPTIME\tPLAYERS\tCLEAN RELAUNCHES\tRESTART PENDING")
	for _, r := range reports {
		uptime, players, cycles := "unknown", "unknown", fmt.Sprint(len(r.History.Cycles))
		if r.Snapshot != nil && len(r.Snapshot.Processes) == 1 {
			duration := r.Snapshot.At.Sub(r.Snapshot.Processes[0].Started)
			if !r.Snapshot.Processes[0].Started.IsZero() && duration >= 0 {
				uptime = duration.Truncate(time.Second).String()
			}
		}
		if r.DZSA != nil {
			players = fmt.Sprintf("%d/%d", r.DZSA.Server.Players, r.DZSA.Server.MaxPlayers)
		}
		if r.History.Incomplete {
			if len(r.History.Cycles) == 0 {
				cycles = "unknown"
			} else {
				cycles = ">=" + cycles
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%t\n", r.Target, r.Status, uptime, players, cycles, r.RestartPending)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	for _, r := range reports {
		fmt.Fprintf(&buffer, "%s Installed: %s\n%s Last managed startup: %s\n", r.Target, r.Installed, r.Target, r.Startup)
		for _, e := range r.Errors {
			fmt.Fprintf(&buffer, "%s ERROR: %s\n", r.Target, e)
		}
		for _, v := range r.Warnings {
			fmt.Fprintf(&buffer, "%s WARNING: %s\n", r.Target, v)
		}
	}
	_, err := io.Copy(destination, &buffer)
	return err
}

func emit(v any, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch r := v.(type) {
	case deploy.Result:
		if r.Backup != nil {
			fmt.Printf("Backup: %s (%d bytes, revision %s)\n", r.Backup.ID, r.Backup.Bytes, r.Backup.Manifest.Revision)
		}
		if r.StopMethod != "" {
			fmt.Println("Stop method:", r.StopMethod)
		}
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
