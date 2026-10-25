package deploy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func Git(ctx context.Context, repo string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	b, err := cmd.Output()
	if err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(string(x.Stderr)))
		}
		return nil, err
	}
	return b, nil
}
func Eligible(ctx context.Context, repo string) (string, error) {
	b, err := Git(ctx, repo, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	if len(b) != 0 {
		return "", fmt.Errorf("deployment declined: working tree is dirty (including untracked files); commit and merge changes to main first")
	}
	branch, err := Git(ctx, repo, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(branch)) != "main" {
		return "", fmt.Errorf("deployment declined: checkout must be on main")
	}
	if _, err = Git(ctx, repo, "fetch", "--no-tags", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
		return "", fmt.Errorf("cannot verify latest origin/main: %w", err)
	}
	head, err := Git(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	remote, err := Git(ctx, repo, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return "", err
	}
	if string(head) != string(remote) {
		return "", fmt.Errorf("deployment declined: HEAD must equal latest origin/main; merge/push or update the checkout explicitly")
	}
	return strings.TrimSpace(string(head)), nil
}
