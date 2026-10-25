package deploy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	battleye "github.com/multiplay/go-battleye"
	a2s "github.com/rumblefrog/go-a2s"
)

type Game interface {
	Alive(State) bool
	Stop(context.Context, State) error
	Update(context.Context, Target) error
	Launch(Target) (int, string, error)
	Healthy(context.Context, Target) error
}
type NativeGame struct{ Config HostConfig }

func (g *NativeGame) Alive(s State) bool {
	if s.PID == 0 {
		return false
	}
	token, err := processToken(s.PID, filepath.Join(g.Config.InstallDir, "DayZServer_x64.exe"))
	return err == nil && token == s.ProcessToken
}
func (g *NativeGame) rcon(command string) error {
	raw, err := os.ReadFile(g.Config.RConPasswordFile)
	if err != nil {
		return err
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return errors.New("empty host RCon password")
	}
	c, err := battleye.NewClient(g.Config.RConAddress, password, battleye.Timeout(3*time.Second))
	if err != nil {
		return fmt.Errorf("RCon authentication failed: %w", err)
	}
	defer c.Close()
	// Drain broadcasts: the library has a bounded message channel.
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case _, ok := <-c.Messages():
				if !ok {
					return
				}
			}
		}
	}()
	_, err = c.Exec(command)
	return err
}
func (g *NativeGame) Stop(ctx context.Context, s State) error {
	if !g.Alive(s) {
		return nil
	}
	if err := g.rcon("version"); err != nil {
		return err
	}
	// A shutdown may close its socket before acknowledging. Process exit is the authority.
	_ = g.rcon("#shutdown")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for g.Alive(s) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("clean shutdown timed out; process was not force-killed: %w", ctx.Err())
		case <-ticker.C:
		}
	}
	return nil
}
func (g *NativeGame) run(ctx context.Context, exe string, args ...string) error {
	log, err := os.OpenFile(filepath.Join(g.Config.StateDir, "logs", "updates.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = filepath.Dir(exe)
	prepareCommand(cmd)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Stdin = nil
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("%s failed; see protected host update log: %w", filepath.Base(exe), err)
	}
	return nil
}
func (g *NativeGame) Update(ctx context.Context, t Target) error {
	if err := ensureNoGame(g.Config.InstallDir); err != nil {
		return err
	}
	if err := t.Validate(); err != nil {
		return err
	}
	unlock, err := acquireSteam(ctx, g.Config)
	if err != nil {
		return err
	}
	defer unlock()
	args := []string{"+force_install_dir", g.Config.InstallDir, "+login", g.Config.SteamUser, "+app_update", "223350"}
	if t.ValidateServer {
		args = append(args, "validate")
	}
	args = append(args, "+quit")
	if err := g.run(ctx, g.Config.SteamCMD, args...); err != nil {
		return err
	}
	args = []string{"+login", g.Config.SteamUser}
	for _, m := range t.Mods {
		args = append(args, "+workshop_download_item", "221100", m.ID, "validate")
	}
	args = append(args, "+quit")
	if err := g.run(ctx, g.Config.SteamCMD, args...); err != nil {
		return err
	}
	keys, err := SafeJoin(g.Config.InstallDir, "keys")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(keys, 0700); err != nil {
		return err
	}
	for _, m := range t.Mods {
		source, err := SafeJoin(g.Config.WorkshopDir, m.ID)
		if err != nil {
			return err
		}
		dest, err := SafeJoin(g.Config.InstallDir, m.Folder)
		if err != nil {
			return err
		}
		if err = syncModContext(ctx, source, dest); err != nil {
			return fmt.Errorf("sync %s: %w", m.Folder, err)
		}
		for _, dir := range []string{dest, filepath.Join(dest, "keys")} {
			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.IsDir() || !strings.Contains(strings.ToLower(e.Name()), "key") {
					continue
				}
				src, err := SafeJoin(dir, e.Name())
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(src)
				if err != nil {
					return err
				}
				dst, err := SafeJoin(keys, e.Name())
				if err != nil {
					return err
				}
				if err = AtomicWrite(dst, raw, 0600); err != nil {
					return err
				}
			}
		}
	}
	if err = g.ensureMission(ctx, t); err != nil {
		return err
	}
	return setupRCon(g.Config)
}

// syncMod mirrors only a validated @mod destination, never mission or profiles data.
func syncMod(source, dest string) error {
	return syncModContext(context.Background(), source, dest)
}
func syncModContext(ctx context.Context, source, dest string) error {
	if _, err := os.Stat(source); err != nil {
		return err
	}
	if err := noLink(source); err != nil {
		return err
	}
	if err := noLink(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	keep := map[string]bool{}
	err := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if p == source {
			return nil
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err = noLink(p); err != nil {
			return err
		}
		out, err := SafeJoin(dest, rel)
		if err != nil {
			return err
		}
		keep[strings.ToLower(rel)] = true
		if d.IsDir() {
			return os.MkdirAll(out, 0700)
		}
		if !d.Type().IsRegular() {
			return errors.New("non-regular Workshop file")
		}
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		if current, e := os.Stat(out); e == nil && current.Size() == info.Size() && current.ModTime().Equal(info.ModTime()) {
			return nil
		}
		if err = atomicCopyContext(ctx, p, out, 0600); err != nil {
			return err
		}
		return os.Chtimes(out, info.ModTime(), info.ModTime())
	})
	if err != nil {
		return err
	}
	var remove []string
	err = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dest {
			return nil
		}
		if err = noLink(p); err != nil {
			return err
		}
		rel, err := filepath.Rel(dest, p)
		if err != nil {
			return err
		}
		if !keep[strings.ToLower(filepath.ToSlash(rel))] {
			remove = append(remove, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(remove, func(i, j int) bool { return len(remove[i]) > len(remove[j]) })
	for _, p := range remove {
		if err = ctx.Err(); err != nil {
			return err
		}
		if !within(dest, p) {
			return errors.New("mod cleanup escaped destination")
		}
		if err = os.Remove(p); err != nil {
			return err
		}
	}
	return nil
}
func (g *NativeGame) Launch(t Target) (int, string, error) {
	if err := ensureNoGame(g.Config.InstallDir); err != nil {
		return 0, "", err
	}
	exe, err := SafeJoin(g.Config.InstallDir, "DayZServer_x64.exe")
	if err != nil {
		return 0, "", err
	}
	mission, err := SafeJoin(g.Config.InstallDir, "mpmissions/"+t.Mission)
	if err != nil {
		return 0, "", err
	}
	if _, err = os.Stat(filepath.Join(mission, "db", "types.xml")); err != nil {
		return 0, "", fmt.Errorf("complete stock mission must be installed before launch: %w", err)
	}
	var mods []string
	for _, m := range t.Mods {
		mods = append(mods, m.Folder)
	}
	args := []string{"-config=" + filepath.Join(g.Config.InstallDir, "serverDZ.cfg"), fmt.Sprintf("-port=%d", t.GamePort), "-profiles=" + filepath.Join(g.Config.InstallDir, "profiles"), "-dologs", "-adminlog", "-netlog", "-freezecheck", "-BEpath=" + filepath.Join(g.Config.InstallDir, "battleye"), "-mod=" + strings.Join(mods, ";")}
	log, err := os.OpenFile(filepath.Join(g.Config.StateDir, "logs", "server-console.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return 0, "", err
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = g.Config.InstallDir
	cmd.Stdout = log
	cmd.Stderr = log
	prepareCommand(cmd)
	if err = cmd.Start(); err != nil {
		log.Close()
		return 0, "", err
	}
	pid := cmd.Process.Pid
	token, err := processToken(pid, exe)
	go func() { _ = cmd.Wait(); log.Close() }()
	return pid, token, err
}

var hostnamePattern = regexp.MustCompile(`(?m)^\s*hostname\s*=\s*"([^"\r\n]*)"\s*;`)

func (g *NativeGame) Healthy(ctx context.Context, t Target) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := g.rcon("version"); err != nil {
		return err
	}
	c, err := a2s.NewClient(g.Config.QueryAddress, a2s.TimeoutOption(3*time.Second))
	if err != nil {
		return err
	}
	defer c.Close()
	info, err := c.QueryInfo()
	if err != nil {
		return err
	}
	terrain := strings.Split(t.Mission, ".")
	if !strings.EqualFold(info.Map, terrain[len(terrain)-1]) {
		return fmt.Errorf("query reports unexpected map %q", info.Map)
	}
	raw, err := os.ReadFile(filepath.Join(g.Config.InstallDir, "serverDZ.cfg"))
	if err != nil {
		return err
	}
	match := hostnamePattern.FindSubmatch(raw)
	if len(match) != 2 || info.Name != string(match[1]) {
		return errors.New("query does not match deployed hostname")
	}
	return nil
}
