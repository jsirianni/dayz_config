package deploy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Check struct {
	Kind     string   `json:"kind"`
	Target   string   `json:"target"`
	ArgList  []string `json:"argList"`
	ExitCode int      `json:"exitCode"`
	Output   string   `json:"output"`
}
type Validation struct {
	Checks   []Check  `json:"checks"`
	Failures int      `json:"failures"`
	Warnings int      `json:"warnings"`
	Skipped  []string `json:"skipped,omitempty"`
}

func Discover(root string) ([]Check, []string, error) {
	var checks []Check
	var skipped []string
	exists := func(p string) bool { s, err := os.Stat(p); return err == nil && !s.IsDir() }
	add := func(kind, p string, args ...string) {
		checks = append(checks, Check{Kind: kind, Target: p, ArgList: args})
	}
	if exists(filepath.Join(root, "serverDZ.cfg")) {
		p := filepath.Join(root, "serverDZ.cfg")
		add("server", p, "validate", "server", p)
	}
	dirs := map[string]bool{root: true}
	hasXML := false
	hasAI := false
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("validation tree contains symlink: %s", p)
		}
		if d.IsDir() {
			return nil
		}
		base := strings.ToLower(d.Name())
		if strings.HasSuffix(base, ".xml") {
			hasXML = true
		}
		switch base {
		case "cfgeconomycore.xml", "cfggameplay.json", "cfgweather.xml", "init.c":
			dirs[filepath.Dir(p)] = true
		}
		if base == "aipatrolsettings.json" || base == "aisettings.json" {
			hasAI = true
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	var ordered []string
	for dir := range dirs {
		ordered = append(ordered, dir)
	}
	sort.Strings(ordered)
	for _, dir := range ordered {
		if exists(filepath.Join(dir, "cfgeconomycore.xml")) {
			add("economy", dir, "validate", "economy", dir, "--warnings", "compact")
		} else if exists(filepath.Join(dir, "db", "types.xml")) {
			skipped = append(skipped, dir+": partial mission; no cfgeconomycore.xml")
		}
		for _, entry := range []struct{ name, kind string }{{"cfggameplay.json", "gameplay"}, {"cfgweather.xml", "weather"}, {"init.c", "init"}} {
			p := filepath.Join(dir, entry.name)
			if exists(p) {
				add(entry.kind, p, "validate", entry.kind, p)
			}
		}
	}
	if hasXML {
		add("xml", root, "validate", "xml", root)
	}
	if hasAI {
		add("expansion-ai", root, "validate", "expansion", "ai", root)
	}
	return checks, skipped, nil
}
func Validate(ctx context.Context, dz, root string) (Validation, error) {
	checks, skipped, err := Discover(root)
	r := Validation{Checks: checks, Skipped: skipped}
	if err != nil {
		return r, err
	}
	if len(checks) == 0 {
		return r, fmt.Errorf("no configuration validations discovered in %s", root)
	}
	for i := range r.Checks {
		c := &r.Checks[i]
		cmd := exec.CommandContext(ctx, dz, c.ArgList...)
		out, err := cmd.CombinedOutput()
		c.Output = string(out)
		if err != nil {
			c.ExitCode = 1
			if x, ok := err.(*exec.ExitError); ok {
				c.ExitCode = x.ExitCode()
			}
			r.Failures++
		}
		r.Warnings += strings.Count(strings.ToLower(c.Output), "warning:")
		if strings.Contains(c.Output, "Usage:\n  dzcli [command]") || strings.Contains(c.Output, "Usage:\r\n  dzcli [command]") {
			c.ExitCode = 1
			r.Failures++
			c.Output += "\nUnexpected top-level help; validator was not executed."
		}
	}
	if r.Failures > 0 {
		return r, fmt.Errorf("configuration validation failed: %d/%d checks", r.Failures, len(r.Checks))
	}
	return r, nil
}
