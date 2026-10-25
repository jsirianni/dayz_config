package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func DzPath(repo string) string {
	name := "dzcli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(repo, ".tools", "dzcli", name)
}
func EnsureDz(ctx context.Context, repo string, offline bool) (string, error) {
	return ensureDz(ctx, repo, offline, &http.Client{Timeout: 2 * time.Minute})
}
func ensureDz(ctx context.Context, repo string, offline bool, client *http.Client) (string, error) {
	dest := DzPath(repo)
	if offline {
		if _, err := os.Stat(dest); err != nil {
			return "", fmt.Errorf("cached dzcli missing; run dzdeploy tools install: %w", err)
		}
		return dest, nil
	}
	get := func(url string, limit int64) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "dzdeploy")
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if int64(len(b)) > limit {
			return nil, errors.New("download too large")
		}
		return b, err
	}
	raw, err := get("https://api.github.com/repos/jsirianni/dzcli/releases/latest", 4<<20)
	if err != nil {
		return "", fmt.Errorf("dzcli update check failed (use validate --offline only for local cached checks): %w", err)
	}
	var release struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err = json.Unmarshal(raw, &release); err != nil {
		return "", err
	}
	name := "dzcli_" + strings.TrimPrefix(release.Tag, "v") + "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, a := range release.Assets {
		if a.Name != name {
			continue
		}
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
			installed, err := exec.CommandContext(ctx, dest, "version").Output()
			if err == nil && strings.TrimSpace(strings.TrimPrefix(string(installed), "v")) == strings.TrimPrefix(release.Tag, "v") {
				return dest, nil
			}
		}
		if !strings.HasPrefix(a.URL, "https://github.com/jsirianni/dzcli/releases/download/") {
			return "", errors.New("unexpected release download URL")
		}
		binary, err := get(a.URL, 128<<20)
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(a.Digest, "sha256:") || Digest(binary) != strings.TrimPrefix(a.Digest, "sha256:") {
			return "", errors.New("release SHA-256 digest missing or mismatched")
		}
		if err = AtomicWrite(dest, binary, 0700); err != nil {
			return "", err
		}
		return dest, nil
	}
	return "", fmt.Errorf("dzcli has no release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
}
