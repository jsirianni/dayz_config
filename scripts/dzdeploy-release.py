"""Build portable packages from GoReleaser outputs, verify them, and publish drafts.

Uses only Python's standard library. Run from the repository root.
"""

import argparse
import hashlib
import json
from pathlib import Path
import platform
import re
import struct
import subprocess
import tempfile
import zipfile


PLATFORMS = [(system, arch) for system in ("windows", "linux") for arch in ("amd64", "arm64")]
COMPANION = "agents/windows-amd64/dzdeploy.exe"
SUPPORT_FILES = {
    "bootstrap-windows.ps1", "deploy.example.yaml", "deployment.md", "operator-workflows.md"
}
STABLE_VERSION = r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"


def version_from_ref(ref, sha):
    if ref.startswith("refs/tags/"):
        match = re.fullmatch("refs/tags/dzdeploy/v(" + STABLE_VERSION + ")", ref)
        if not match:
            raise ValueError("release tag must be dzdeploy/vMAJOR.MINOR.PATCH")
        return match[1]
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("preview requires a full commit SHA")
    return "0.0.0-dev." + sha[:12]


def validate_version(version):
    if not re.fullmatch(STABLE_VERSION + r"|0\.0\.0-dev\.[0-9a-f]{12}", version):
        raise ValueError("invalid package version")


def digest(data):
    return hashlib.sha256(data).hexdigest()


def archive_name(version, system, arch):
    return f"dzdeploy_{version}_{system}_{arch}.zip"


def check_binary(data, system, arch):
    if system == "windows":
        if data[:2] != b"MZ" or len(data) < 64:
            raise ValueError("expected Windows PE executable")
        offset = struct.unpack_from("<I", data, 60)[0]
        if len(data) < offset + 26 or data[offset:offset + 4] != b"PE\0\0":
            raise ValueError("invalid PE header")
        machine = struct.unpack_from("<H", data, offset + 4)[0]
        magic = struct.unpack_from("<H", data, offset + 24)[0]
        if machine != {"amd64": 0x8664, "arm64": 0xAA64}[arch] or magic != 0x20B:
            raise ValueError("Windows executable architecture mismatch")
    else:
        if len(data) < 64 or data[:6] != b"\x7fELF\x02\x01":
            raise ValueError("expected little-endian ELF64 executable")
        kind, machine = struct.unpack_from("<HH", data, 16)
        if kind not in (2, 3) or machine != {"amd64": 62, "arm64": 183}[arch]:
            raise ValueError("Linux executable architecture mismatch")


def package(artifacts, output, version):
    validate_version(version)
    binaries = {}
    for artifact in json.loads(artifacts.read_text(encoding="utf-8")):
        if artifact["type"] != "Binary":
            continue
        target = artifact["goos"], artifact["goarch"]
        if target not in PLATFORMS or target in binaries:
            raise ValueError(f"unexpected or duplicate binary: {target}")
        binary = Path(artifact["path"])
        check_binary(binary.read_bytes(), *target)
        binaries[target] = binary
    if set(binaries) != set(PLATFORMS):
        raise ValueError("GoReleaser must produce all four client platforms")
    output.mkdir(parents=True, exist_ok=True)
    # Reject stale packages instead of accidentally publishing mixed versions.
    if list(output.iterdir()):
        raise ValueError("package output directory must be empty")
    for system, arch in PLATFORMS:
        subprocess.run([
            "go", "run", "./cmd/dzdeploy-package", "--os", system, "--arch", arch,
            "--version", version, "--output", str(output),
            "--client-binary", str(binaries[system, arch]),
            "--agent-binary", str(binaries["windows", "amd64"]),
        ], check=True)
    checksums = []
    for system, arch in PLATFORMS:
        name = archive_name(version, system, arch)
        checksums.append(f"{digest((output / name).read_bytes())}  {name}\n")
        # Packaging must preserve the exact GoReleaser bytes, with no rebuilds.
        with zipfile.ZipFile(output / name) as archive:
            client = "dzdeploy.exe" if system == "windows" else "dzdeploy"
            if archive.read(client) != binaries[system, arch].read_bytes():
                raise ValueError("packager changed the client executable")
            if archive.read(COMPANION) != binaries["windows", "amd64"].read_bytes():
                raise ValueError("packager changed the Windows companion")
    (output / "checksums.txt").write_text("".join(checksums), encoding="utf-8", newline="\n")
    verify(output, version)


def verify(directory, version, smoke=False):
    validate_version(version)
    expected = {archive_name(version, *target) for target in PLATFORMS}
    if {p.name for p in directory.iterdir()} != expected | {"checksums.txt"}:
        raise ValueError("release must contain exactly four archives and checksums.txt")
    checksums = {}
    for line in (directory / "checksums.txt").read_text(encoding="utf-8").splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([^/\\]+)", line)
        if not match or match[2] in checksums:
            raise ValueError("invalid or duplicate archive checksum")
        checksums[match[2]] = match[1]
    if set(checksums) != expected:
        raise ValueError("archive checksum inventory mismatch")
    companion_hash = None
    for system, arch in PLATFORMS:
        name = archive_name(version, system, arch)
        path = directory / name
        if digest(path.read_bytes()) != checksums[name]:
            raise ValueError(f"archive checksum mismatch: {name}")
        client = "dzdeploy.exe" if system == "windows" else "dzdeploy"
        with zipfile.ZipFile(path) as archive:
            inventory = SUPPORT_FILES | {client, COMPANION, "checksums.json"}
            if len(archive.namelist()) != len(inventory) or set(archive.namelist()) != inventory:
                raise ValueError(f"unexpected package contents: {name}")
            manifest = json.loads(archive.read("checksums.json"))
            if not isinstance(manifest.get("protocol"), int) or manifest["protocol"] < 1:
                raise ValueError("invalid deployment protocol in package manifest")
            if set(manifest["sha256"]) != inventory - {"checksums.json"}:
                raise ValueError("internal checksum inventory mismatch")
            for entry, checksum in manifest["sha256"].items():
                if digest(archive.read(entry)) != checksum:
                    raise ValueError(f"internal checksum mismatch: {entry}")
            check_binary(archive.read(client), system, arch)
            check_binary(archive.read(COMPANION), "windows", "amd64")
            agent_hash = digest(archive.read(COMPANION))
            if companion_hash is not None and companion_hash != agent_hash:
                raise ValueError("packages contain different Windows companions")
            companion_hash = agent_hash
            if system == "windows" and arch == "amd64" and archive.read(client) != archive.read(COMPANION):
                raise ValueError("Windows amd64 client and companion differ")
            if system == "linux" and not (archive.getinfo(client).external_attr >> 16) & 0o111:
                raise ValueError("Linux client is not executable")
            if smoke and arch == "amd64" and platform.system().lower() == system:
                with tempfile.TemporaryDirectory(prefix="dzdeploy-smoke-") as temporary:
                    executable = Path(temporary) / client
                    executable.write_bytes(archive.read(client))
                    executable.chmod(0o755)
                    for command in ("--help", "version"):
                        subprocess.run([str(executable), command], check=True, timeout=30)
    print(f"Verified all four dzdeploy {version} packages", flush=True)


def publish(directory, ref, repository):
    # Revalidate at the write boundary, independently of the workflow condition.
    if not ref.startswith("refs/tags/"):
        raise ValueError("only version tags can publish")
    version = version_from_ref(ref, "")
    tag = ref.removeprefix("refs/tags/")
    subprocess.run(["git", "fetch", "origin", "main"], check=True)
    commit = subprocess.check_output(["git", "rev-parse", tag + "^{commit}"], text=True).strip()
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    if commit != head:
        raise ValueError("release tag does not match the checked-out commit")
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], check=True)
    verify(directory, version)
    gh = ["gh", "--repo", repository, "release"]
    existing = subprocess.run(gh + ["view", tag, "--json", "isDraft"], capture_output=True, text=True)
    if existing.returncode == 0:
        if not json.loads(existing.stdout)["isDraft"]:
            raise ValueError("release is already published; existing assets will not be overwritten")
    else:
        subprocess.run(gh + [
            "create", tag, "--verify-tag", "--draft", "--latest=false",
            "--title", "dzdeploy " + version,
            "--notes", "Portable Windows and Linux deployment clients. Verify checksums.txt before extracting. "
            "Keep the bundled Windows amd64 companion beside the client. Publishing does not deploy or restart servers. "
            "Source revision: " + commit,
        ], check=True)
    files = sorted(directory.iterdir())
    subprocess.run(gh + ["upload", tag, "--clobber", *map(str, files)], check=True)
    # Verify downloaded bytes before making the draft public. A failed run leaves
    # a draft that can be retried; a published release is never overwritten.
    with tempfile.TemporaryDirectory(prefix="dzdeploy-release-") as temporary:
        downloaded = Path(temporary)
        subprocess.run(gh + ["download", tag, "--dir", str(downloaded)], check=True)
        verify(downloaded, version)
        for path in files:
            if digest(path.read_bytes()) != digest((downloaded / path.name).read_bytes()):
                raise ValueError("uploaded release asset differs: " + path.name)
    subprocess.run(gh + ["edit", tag, "--draft=false", "--latest=false"], check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    version_parser = commands.add_parser("version")
    version_parser.add_argument("--ref", required=True)
    version_parser.add_argument("--sha", required=True)
    package_parser = commands.add_parser("package")
    package_parser.add_argument("--artifacts", type=Path, default=Path(".tools/release/artifacts.json"))
    for name in ("package", "verify", "publish"):
        command = package_parser if name == "package" else commands.add_parser(name)
        command.add_argument("--directory", type=Path, default=Path(".tools/packages"))
        if name == "publish":
            command.add_argument("--ref", required=True)
            command.add_argument("--repository", required=True)
        else:
            command.add_argument("--version", required=True)
        if name == "verify":
            command.add_argument("--smoke", action="store_true")
    args = parser.parse_args()
    if args.command == "version":
        print(version_from_ref(args.ref, args.sha))
    elif args.command == "package":
        package(args.artifacts, args.directory, args.version)
    elif args.command == "verify":
        verify(args.directory, args.version, args.smoke)
    else:
        publish(args.directory, args.ref, args.repository)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.SubprocessError, zipfile.BadZipFile) as error:
        raise SystemExit(str(error)) from error
