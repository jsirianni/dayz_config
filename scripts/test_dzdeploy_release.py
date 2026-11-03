import importlib.util
import json
from pathlib import Path
import shutil
import struct
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile


spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("dzdeploy-release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


def binary(system, arch):
    data = bytearray(128)
    if system == "windows":
        data[:2] = b"MZ"
        struct.pack_into("<I", data, 60, 64)
        data[64:68] = b"PE\0\0"
        struct.pack_into("<H", data, 68, {"amd64": 0x8664, "arm64": 0xAA64}[arch])
        struct.pack_into("<H", data, 88, 0x20B)
    else:
        data[:6] = b"\x7fELF\x02\x01"
        struct.pack_into("<HH", data, 16, 2, {"amd64": 62, "arm64": 183}[arch])
    return bytes(data)


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.contents = {}
        for system, arch in release.PLATFORMS:
            client = "dzdeploy.exe" if system == "windows" else "dzdeploy"
            files = {name: b"public documentation" for name in release.SUPPORT_FILES}
            files[client] = binary(system, arch)
            files[release.COMPANION] = binary("windows", "amd64")
            self.contents[system, arch] = files
        self.write_packages()

    def write_packages(self):
        lines = []
        for target, files in self.contents.items():
            path = self.directory / release.archive_name("1.2.3", *target)
            with zipfile.ZipFile(path, "w") as archive:
                for name, data in files.items():
                    entry = zipfile.ZipInfo(name)
                    entry.external_attr = 0o100755 << 16
                    archive.writestr(entry, data)
                archive.writestr("checksums.json", json.dumps({
                    "protocol": 1, "sha256": {name: release.digest(data) for name, data in files.items()}
                }))
            lines.append(f"{release.digest(path.read_bytes())}  {path.name}\n")
        (self.directory / "checksums.txt").write_text("".join(lines), encoding="utf-8")

    def test_release_and_preview_versions(self):
        self.assertEqual(release.version_from_ref("refs/tags/dzdeploy/v1.2.3", ""), "1.2.3")
        self.assertEqual(release.version_from_ref("refs/pull/5/merge", "a" * 40), "0.0.0-dev." + "a" * 12)
        for ref in ("refs/tags/v1.2.3", "refs/tags/dzdeploy/v01.2.3", "refs/tags/dzdeploy/v1.2.3-rc.1",
                    "refs/tags/dzdeploy/v1.2", "refs/tags/dzdeploy/v1.2.3/../../oops"):
            with self.subTest(ref=ref), self.assertRaises(ValueError):
                release.version_from_ref(ref, "a" * 40)

    def test_verify_all_platforms(self):
        release.verify(self.directory, "1.2.3")

    def test_wrong_architecture_rejected_even_with_valid_checksums(self):
        self.contents["windows", "arm64"]["dzdeploy.exe"] = binary("windows", "amd64")
        self.write_packages()
        with self.assertRaisesRegex(ValueError, "architecture"):
            release.verify(self.directory, "1.2.3")

    def test_missing_companion_rejected(self):
        del self.contents["linux", "arm64"][release.COMPANION]
        self.write_packages()
        with self.assertRaisesRegex(ValueError, "contents"):
            release.verify(self.directory, "1.2.3")

    def test_secret_file_rejected(self):
        self.contents["windows", "amd64"]["deploy.yaml"] = b"private config"
        self.write_packages()
        with self.assertRaisesRegex(ValueError, "contents"):
            release.verify(self.directory, "1.2.3")

    def test_companions_must_match(self):
        self.contents["linux", "arm64"][release.COMPANION] += b"different revision"
        self.write_packages()
        with self.assertRaisesRegex(ValueError, "different Windows companions"):
            release.verify(self.directory, "1.2.3")

    def test_archive_tampering_rejected(self):
        path = self.directory / release.archive_name("1.2.3", "linux", "amd64")
        path.write_bytes(path.read_bytes() + b"tampering")
        with self.assertRaisesRegex(ValueError, "archive checksum mismatch"):
            release.verify(self.directory, "1.2.3")

    def test_internal_tampering_rejected(self):
        path = self.directory / release.archive_name("1.2.3", "linux", "amd64")
        with zipfile.ZipFile(path) as archive:
            files = {entry.filename: archive.read(entry) for entry in archive.infolist()}
        files["deployment.md"] = b"tampering"
        with zipfile.ZipFile(path, "w") as archive:
            for name, data in files.items():
                archive.writestr(name, data)
        checksums = self.directory / "checksums.txt"
        lines = checksums.read_text().splitlines()
        lines = [f"{release.digest(path.read_bytes())}  {path.name}" if line.endswith(path.name) else line for line in lines]
        checksums.write_text("\n".join(lines) + "\n")
        with self.assertRaisesRegex(ValueError, "internal checksum mismatch"):
            release.verify(self.directory, "1.2.3")

    def simulate_publish(self, fail_download=False, published=False, new=False, corrupt_download=False):
        commands = []

        def run(command, **kwargs):
            commands.append(command)
            if "view" in command:
                if new:
                    return subprocess.CompletedProcess(command, 1, "")
                return subprocess.CompletedProcess(command, 0, json.dumps({"isDraft": not published}))
            if "download" in command:
                if fail_download:
                    raise subprocess.CalledProcessError(1, command)
                destination = Path(command[command.index("--dir") + 1])
                for path in self.directory.iterdir():
                    shutil.copyfile(path, destination / path.name)
                if corrupt_download:
                    (destination / "checksums.txt").write_text("corrupted upload")
            return subprocess.CompletedProcess(command, 0)

        with patch.object(release.subprocess, "run", side_effect=run), \
                patch.object(release.subprocess, "check_output", return_value="a" * 40):
            if published:
                with self.assertRaisesRegex(ValueError, "already published"):
                    release.publish(self.directory, "refs/tags/dzdeploy/v1.2.3", "owner/repo")
            elif fail_download or corrupt_download:
                with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                    release.publish(self.directory, "refs/tags/dzdeploy/v1.2.3", "owner/repo")
            else:
                release.publish(self.directory, "refs/tags/dzdeploy/v1.2.3", "owner/repo")
        return commands

    def test_publish_verifies_before_publishing_and_preserves_latest(self):
        commands = self.simulate_publish()
        self.assertIn("download", commands[-2])
        self.assertIn("--draft=false", commands[-1])
        self.assertIn("--latest=false", commands[-1])

    def test_new_release_is_created_as_draft(self):
        commands = self.simulate_publish(new=True)
        creation = next(command for command in commands if "create" in command)
        self.assertIn("--draft", creation)
        self.assertIn("--verify-tag", creation)
        self.assertIn("--latest=false", creation)

    def test_failed_asset_verification_leaves_draft(self):
        commands = self.simulate_publish(fail_download=True)
        self.assertFalse(any("edit" in command for command in commands))

    def test_corrupted_download_leaves_draft(self):
        commands = self.simulate_publish(corrupt_download=True)
        self.assertFalse(any("edit" in command for command in commands))

    def test_published_assets_not_overwritten(self):
        commands = self.simulate_publish(published=True)
        self.assertFalse(any("upload" in command for command in commands))

    def test_branch_cannot_publish(self):
        with patch.object(release.subprocess, "run") as run, self.assertRaises(ValueError):
            release.publish(self.directory, "refs/heads/main", "owner/repo")
        run.assert_not_called()

    def test_tag_must_match_checkout(self):
        with patch.object(release.subprocess, "run") as run, \
                patch.object(release.subprocess, "check_output", side_effect=["a" * 40, "b" * 40]):
            with self.assertRaisesRegex(ValueError, "checked-out commit"):
                release.publish(self.directory, "refs/tags/dzdeploy/v1.2.3", "owner/repo")
        self.assertFalse(any(call.args[0][0] == "gh" for call in run.call_args_list))

    def test_commit_outside_main_cannot_publish(self):
        def run(command, **kwargs):
            if "merge-base" in command:
                raise subprocess.CalledProcessError(1, command)

        with patch.object(release.subprocess, "run", side_effect=run) as mocked, \
                patch.object(release.subprocess, "check_output", return_value="a" * 40):
            with self.assertRaises(subprocess.CalledProcessError):
                release.publish(self.directory, "refs/tags/dzdeploy/v1.2.3", "owner/repo")
        self.assertFalse(any(call.args[0][0] == "gh" for call in mocked.call_args_list))


if __name__ == "__main__":
    unittest.main()
