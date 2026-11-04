"""Exercise release packaging in a Go-free Git fixture, without host access."""

import io
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
CLIENT = ROOT / ".tools/bin" / ("dzdeploy.exe" if os.name == "nt" else "dzdeploy")


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], stderr=subprocess.STDOUT)


def main():
    # Archive only tracked configuration. Never copy operator YAML or SSH state.
    archive = git(ROOT, "archive", "--format=zip", "HEAD", "--", "internal/deploy/targets.json", "servers/current")
    with tempfile.TemporaryDirectory(prefix="dzdeploy-config-test-") as temporary:
        base = Path(temporary)
        repo = base / "config"
        repo.mkdir()
        with zipfile.ZipFile(io.BytesIO(archive)) as source:
            source.extractall(repo)
        assert not list(repo.rglob("*.go")), "Fixture must contain no Go source"
        git(repo, "init", "--initial-branch=main")
        git(repo, "config", "user.name", "Configuration integration test")
        git(repo, "config", "user.email", "config-test@example.invalid")
        git(repo, "config", "core.autocrlf", "false")
        (repo / ".gitignore").write_text(".tools/\n")
        git(repo, "add", ".")
        git(repo, "commit", "-m", "Configuration fixture")
        git(base, "clone", "--bare", str(repo), str(base / "origin.git"))
        git(repo, "remote", "add", "origin", str(base / "origin.git"))
        shutil.copytree(ROOT / ".tools/dzcli", repo / ".tools/dzcli")
        for alias in ("nasdara", "nasdara-staging", "deer-isle"):
            missing = repo / "intentionally-missing-client.yaml"
            result = subprocess.run(
                [str(CLIENT), "deploy", alias, "--repo", str(repo), "--dry-run", "--config", str(missing)],
                capture_output=True, text=True, timeout=180,
            )
            # Config loading occurs after committed packaging and validation,
            # and before SSH. A missing path makes host access impossible.
            assert result.returncode == 1, result.stdout + result.stderr
            assert "load local endpoint config" in result.stderr and missing.name in result.stderr, result.stderr
            assert list((repo / ".tools/dzdeploy/build").rglob("serverDZ.cfg")), "No configuration bundle extracted"
            print(f"{alias}: committed packaging and validation passed without Go or host access")


if __name__ == "__main__":
    main()
