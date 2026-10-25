package deploy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func gitFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	origin := t.TempDir()
	gitTest(t, repo, "init", "-b", "main")
	gitTest(t, origin, "init", "--bare")
	gitTest(t, repo, "config", "user.email", "test@example.invalid")
	gitTest(t, repo, "config", "user.name", "Test")
	if err := AtomicWrite(filepath.Join(repo, ".gitignore"), []byte(".tools/\n.deploy/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(filepath.Join(repo, "internal", "deploy", "targets.json"), defaultTargets, 0600); err != nil {
		t.Fatal(err)
	}
	target, _ := TargetFor("nasdara-staging")
	base := filepath.Join(repo, "servers", "current", target.Folder)
	for p, raw := range map[string]string{"serverDZ.cfg": "hostname = \"test\";\r\n", "mpmissions/dayzOffline.nasdara/init.c": "// exact bytes\n", "profiles/ExpansionMod/Loadouts/test.json": "{}\n", "profiles/ExpansionMod/ATM/account.json": "runtime", "profiles/VPPAdminTools/Permissions/credentials.txt": "not in package", "validation/result.json": "excluded"} {
		writeTest(t, base, p, raw)
	}
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "fixture")
	gitTest(t, repo, "remote", "add", "origin", origin)
	gitTest(t, repo, "push", "origin", "main")
	return repo, gitTest(t, repo, "rev-parse", "HEAD")
}
func TestGitEligibilityRejectsUnsafeSources(t *testing.T) {
	for _, scenario := range []string{"clean", "unstaged", "staged", "untracked", "branch", "unpublished", "behind", "ignored"} {
		t.Run(scenario, func(t *testing.T) {
			repo, commit := gitFixture(t)
			switch scenario {
			case "unstaged":
				writeTest(t, repo, ".gitignore", "changed")
			case "staged":
				writeTest(t, repo, "new", "x")
				gitTest(t, repo, "add", "new")
			case "untracked":
				writeTest(t, repo, "new", "x")
			case "branch":
				gitTest(t, repo, "checkout", "-b", "feature")
			case "unpublished":
				writeTest(t, repo, "new", "x")
				gitTest(t, repo, "add", "new")
				gitTest(t, repo, "commit", "-m", "not pushed")
			case "behind":
				gitTest(t, repo, "commit", "--allow-empty", "-m", "new remote main")
				gitTest(t, repo, "push", "origin", "main")
				gitTest(t, repo, "reset", "--hard", commit)
			case "ignored":
				writeTest(t, repo, ".tools/cache", "ignored")
				writeTest(t, repo, ".deploy/config.json", "private")
			}
			got, err := Eligible(context.Background(), repo)
			if scenario == "clean" || scenario == "ignored" {
				if err != nil || got != commit {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}
func TestBuildUsesCommittedBytesAndExcludesRuntimeData(t *testing.T) {
	repo, commit := gitFixture(t)
	target, _ := TargetFor("nasdara-staging")
	writeTest(t, filepath.Join(repo, "servers", "current", target.Folder), "serverDZ.cfg", "dirty edit")
	b, err := Build(context.Background(), repo, "nasdara-staging", commit)
	if err != nil {
		t.Fatal(err)
	}
	if string(b.Content["serverDZ.cfg"]) != "hostname = \"test\";\r\n" {
		t.Fatal("package did not use committed bytes")
	}
	if len(b.Manifest.Files) != 3 {
		t.Fatal("runtime files included", b.Manifest.Files)
	}
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw, "nasdara-staging")
	if err != nil {
		t.Fatal(err)
	}
	if err = decoded.Extract(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
func TestDiscoveryMatchesCurrentBaseline(t *testing.T) {
	targets, err := Targets(nil)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for alias, target := range targets {
		checks, _, err := Discover(filepath.Join("..", "..", "servers", "current", target.Folder))
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"nasdara": 2, "nasdara-staging": 4, "deer-isle": 7}[alias]
		if len(checks) != want {
			t.Errorf("%s: got %d checks want %d", alias, len(checks), want)
		}
		total += len(checks)
	}
	if total != 13 {
		t.Fatal(total)
	}
}
func TestSourceSymlinkIsNotPackaged(t *testing.T) {
	repo, commit := gitFixture(t)
	target, _ := TargetFor("nasdara-staging")
	p := filepath.Join(repo, "servers", "current", target.Folder, "mpmissions", "dayzOffline.nasdara", "linked.json")
	if err := os.Symlink(filepath.Join(repo, ".gitignore"), p); err != nil {
		t.Skip("symlink privilege unavailable")
	}
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "symlink")
	commit = gitTest(t, repo, "rev-parse", "HEAD")
	if _, err := Build(context.Background(), repo, "nasdara-staging", commit); err == nil {
		t.Fatal("managed symlink accepted")
	}
}
