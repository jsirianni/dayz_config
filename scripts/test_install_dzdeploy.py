import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
import zipfile

spec = importlib.util.spec_from_file_location("installer", Path(__file__).with_name("install-dzdeploy.py"))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallerTests(unittest.TestCase):
    def archive(self, extra=None):
        data = io.BytesIO()
        with zipfile.ZipFile(data, "w") as archive:
            for name in ("dzdeploy", "agents/windows-amd64/dzdeploy.exe", "bootstrap-windows.ps1", "deploy.example.yaml", "deployment.md", "operator-workflows.md"):
                archive.writestr(name, b"fixture")
            if extra:
                archive.writestr(extra, b"unexpected")
        return data.getvalue()

    def test_checksum_mismatch_fails(self):
        with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
            installer.verify(b"corrupt", "0" * 64)

    def test_rejects_extra_and_unsafe_paths_before_writing(self):
        for name in ("../outside", "/absolute", "private-key", "dzdeploy"):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                with self.assertRaisesRegex(ValueError, "archive contents"):
                    installer.unpack(self.archive(name), "linux", Path(temporary))
                self.assertEqual(list(Path(temporary).iterdir()), [])

    def test_preserves_companion_layout(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary)
            installer.unpack(self.archive(), "linux", destination)
            self.assertEqual((destination / "agents/windows-amd64/dzdeploy.exe").read_bytes(), b"fixture")
            self.assertEqual(len(list(p for p in destination.rglob("*") if p.is_file())), 6)


if __name__ == "__main__":
    unittest.main()
