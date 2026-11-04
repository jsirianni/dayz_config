"""Install the complete, checksum-pinned dzdeploy v0.1.0 release (Python 3)."""

import hashlib
import io
import os
from pathlib import Path
import platform
import tempfile
import urllib.request
import zipfile

VERSION = "0.1.0"
BASE_URL = f"https://github.com/jsirianni/dzdeploy/releases/download/v{VERSION}"
CHECKSUMS_SHA256 = "ecf38581f7839f7dfa5bfcd28ecdf50b6cc65adbbaf55e70a53f600516bfdbcd"
ROOT = Path(__file__).resolve().parents[1]


def target():
    system = platform.system().lower()
    arch = {"amd64": "amd64", "x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    if system not in ("windows", "linux") or arch is None:
        raise ValueError("dzdeploy supports Windows/Linux on amd64/arm64 only")
    return system, arch


def download(name, limit):
    request = urllib.request.Request(f"{BASE_URL}/{name}", headers={"User-Agent": "dayz-config-dzdeploy-installer"})
    with urllib.request.urlopen(request, timeout=120) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError(f"Download exceeds size limit: {name}")
    return data


def verify(data, expected):
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError("Release SHA-256 mismatch; refusing to install")


def unpack(data, system, destination):
    client = "dzdeploy.exe" if system == "windows" else "dzdeploy"
    expected = {client, "agents/windows-amd64/dzdeploy.exe", "bootstrap-windows.ps1", "deploy.example.yaml", "deployment.md", "operator-workflows.md"}
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        members = [entry for entry in archive.infolist() if not entry.is_dir()]
        if len(members) != len(expected) or {entry.filename for entry in members} != expected:
            raise ValueError("Unexpected release archive contents")
        if sum(entry.file_size for entry in members) > 128 * 1024 * 1024:
            raise ValueError("Release archive expands beyond size limit")
        for entry in members:
            path = destination / entry.filename
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(archive.read(entry))
            if entry.filename in (client, "agents/windows-amd64/dzdeploy.exe"):
                path.chmod(0o755)


def install():
    system, arch = target()
    name = f"dzdeploy_{VERSION}_{system}_{arch}.zip"
    checksums = download("checksums.txt", 64 * 1024)
    verify(checksums, CHECKSUMS_SHA256)
    entries = dict(line.split()[::-1] for line in checksums.decode("ascii").splitlines())
    data = download(name, 64 * 1024 * 1024)
    verify(data, entries[name])
    tools = ROOT / ".tools"
    tools.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="dzdeploy-install-", dir=tools) as temporary:
        staged = Path(temporary)
        unpack(data, system, staged)
        destination = tools / "bin"
        for source in staged.rglob("*"):
            if source.is_file():
                final = destination / source.relative_to(staged)
                final.parent.mkdir(parents=True, exist_ok=True)
                os.replace(source, final)
    client = destination / ("dzdeploy.exe" if system == "windows" else "dzdeploy")
    print(f"Installed dzdeploy v{VERSION}: {client}")


if __name__ == "__main__":
    install()
