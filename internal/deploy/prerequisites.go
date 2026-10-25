package deploy

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const nasdaraMissionCommit = "f220b2918b1d79a5055b1ee9be70c0ec1516e876"
const steamInstallerURL = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"

// The official Nasdara areaflags.map is about 80 MiB. This separate stock-asset
// limit must not weaken the smaller limit on managed configuration files.
const maxStockFileBytes = 128 << 20

func downloadOfficial(ctx context.Context, url string, max int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errors.New("prerequisite downloads require HTTPS")
	}
	client := &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) > 5 {
			return errors.New("unsafe prerequisite redirect")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dzdeploy")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prerequisite download returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if int64(len(raw)) > max {
		return nil, errors.New("prerequisite download too large")
	}
	return raw, err
}

func steamExecutable(raw []byte) ([]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	var exe []byte
	for _, f := range z.File {
		if f.Name != "steamcmd.exe" {
			continue
		}
		if exe != nil || !f.Mode().IsRegular() || f.UncompressedSize64 > 128<<20 {
			return nil, errors.New("invalid SteamCMD installer archive")
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		exe, err = io.ReadAll(io.LimitReader(r, 128<<20+1))
		r.Close()
		if err != nil {
			return nil, err
		}
	}
	if len(exe) == 0 {
		return nil, errors.New("SteamCMD executable missing from official archive")
	}
	return exe, nil
}

func (g *NativeGame) ensureMission(ctx context.Context, t Target) error {
	if t.Mission != "dayzOffline.nasdara" {
		return nil
	}
	root, err := SafeJoin(g.Config.InstallDir, "mpmissions/"+t.Mission)
	if err != nil {
		return err
	}
	complete := true
	for _, file := range []string{"db/types.xml", "cfgeconomycore.xml", "init.c"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); os.IsNotExist(err) {
			complete = false
		} else if err != nil {
			return err
		}
	}
	if complete {
		return nil
	}
	url := "https://codeload.github.com/BohemiaInteractive/DayZ-Central-Economy/zip/" + nasdaraMissionCommit
	raw, err := downloadOfficial(ctx, url, 256<<20)
	if err != nil {
		return fmt.Errorf("download pinned stock Nasdara mission: %w", err)
	}
	return installNasdaraMission(ctx, root, raw)
}

func installNasdaraMission(ctx context.Context, root string, raw []byte) error {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	prefix := "DayZ-Central-Economy-" + nasdaraMissionCommit + "/dayzOffline.nasdara/"
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		if !strings.HasPrefix(f.Name, prefix) || f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(f.Name, prefix)
		dest, err := SafeJoin(root, rel)
		if err != nil {
			return err
		}
		if !f.Mode().IsRegular() || f.UncompressedSize64 > maxStockFileBytes || seen[strings.ToLower(rel)] {
			return fmt.Errorf("unsafe, duplicate, or oversized stock mission entry %s (mode %s, bytes %d)", rel, f.Mode(), f.UncompressedSize64)
		}
		total += f.UncompressedSize64
		if total > MaxPackageBytes {
			return errors.New("stock mission exceeds size limit")
		}
		seen[strings.ToLower(rel)] = true
		if err = ctx.Err(); err != nil {
			return err
		}
		if _, err = os.Lstat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(r, maxStockFileBytes+1))
		r.Close()
		if err != nil {
			return err
		}
		if len(data) > maxStockFileBytes {
			return errors.New("stock mission file too large")
		}
		if err = AtomicWrite(dest, data, 0600); err != nil {
			return err
		}
	}
	for _, required := range []string{"db/types.xml", "cfgeconomycore.xml", "init.c"} {
		if !seen[required] {
			return fmt.Errorf("pinned stock mission missing %s", required)
		}
	}
	return nil
}

func acquireSteam(ctx context.Context, c HostConfig) (func(), error) {
	dir := filepath.Join(filepath.Dir(c.SteamCMD), ".dzdeploy-lock")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	for {
		unlock, err := lockHost(dir)
		if err == nil {
			return unlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("SteamCMD is busy or inaccessible: %w", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
