"""Verify this module works outside its parent repository (no live host access)."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument("--go", default="go")
args = parser.parse_args()
source = Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix="pvecli-standalone-") as temp:
    target = Path(temp) / "proxmox"
    shutil.copytree(source, target, ignore=shutil.ignore_patterns(
        "dist", ".tools", ".state", ".git", "config.yaml", "*.exe", "*.test", "__pycache__"))
    environment = dict(os.environ, GOWORK="off")
    subprocess.run([args.go, "test", "./..."], cwd=target, env=environment, check=True)
    subprocess.run([args.go, "build", "-buildvcs=false", "-o", str(Path(temp) / "bin") + os.sep,
                    "./cmd/pvecli"], cwd=target, env=environment, check=True)
print("Standalone copy: tests and build passed with GOWORK=off")
