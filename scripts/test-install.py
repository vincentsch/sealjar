#!/usr/bin/env python3
"""Run with scripts/fake-release.sh python3 scripts/test-install.py."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.work = tempfile.TemporaryDirectory(prefix="sl-check.", dir="/tmp")
        self.addCleanup(self.work.cleanup)
        self.root = Path(self.work.name)
        self.fixture = Path(os.environ["SIGNETLEAF_FIXTURE_DIR"])
        self.env = dict(os.environ, INSTALL_DIR=str(self.root / "installed"))
        self.env.pop("SIGNETLEAF_VERSION", None)
        self.shims = self.root / "shims"
        self.shims.mkdir()
        curl = shutil.which("curl")
        self.shim("curl", f'''#!/usr/bin/env bash
for arg do
 case "$arg" in http*) [[ "$arg" == "$SIGNETLEAF_DOWNLOAD_BASE/"* ]] || exit 90;; esac
done
exec '{curl}' "$@"
''')
        self.env["PATH"] = str(self.shims) + os.pathsep + self.env["PATH"]

    def shim(self, name, content):
        path = self.shims / name
        path.write_text(content)
        path.chmod(0o755)

    def install(self, success=True):
        result = subprocess.run(["bash", "install.sh"], env=self.env, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        self.assertEqual(result.returncode == 0, success, result.stdout)
        return result.stdout

    def test_latest_and_pinned(self):
        latest = self.install()
        self.assertIn("v0.1.0", latest)
        self.assertIn(str(self.root / "installed/signetleaf"), latest)
        self.assertIn("PATH", latest)
        self.assertIn("Chrome", latest)
        self.assertIn("Git", latest)
        self.env["SIGNETLEAF_VERSION"] = "v0.0.1"
        self.assertIn("v0.0.1", self.install())
        requests = (self.fixture / "requests.log").read_text()
        for prefix in ("/latest/download/", "/download/v0.0.1/"):
            for asset in ("signetleaf-linux-amd64", "checksums.txt"):
                self.assertIn(prefix + asset, requests)

    def test_default_install_directory(self):
        self.env.pop("INSTALL_DIR")
        self.env["HOME"] = str(self.root / "home directory")
        output = self.install()
        installed = Path(self.env["HOME"]) / ".local/bin/signetleaf"
        self.assertTrue(installed.is_file())
        self.assertIn(str(installed), output)

    def test_checksum_failures_preserve_install(self):
        self.install()
        installed = self.root / "installed/signetleaf"
        before = hashlib.sha256(installed.read_bytes()).digest()
        checksums = self.fixture / "latest/download/checksums.txt"
        original = checksums.read_bytes()
        self.addCleanup(checksums.write_bytes, original)
        for content, message in (("0" * 64 + "  signetleaf-linux-amd64\n", "Checksum verification failed"),
                                 ("", "was not found")):
            with self.subTest(content=bool(content)):
                checksums.write_text(content)
                self.assertIn(message, self.install(False))
                self.assertEqual(hashlib.sha256(installed.read_bytes()).digest(), before)
                installed.unlink()
                self.install(False)
                self.assertFalse(installed.exists())
                installed.write_bytes((self.fixture / "latest/download/signetleaf-linux-amd64").read_bytes())
                installed.chmod(0o755)

    def test_unsupported_architecture(self):
        self.shim("uname", '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo riscv64;; esac\n')
        self.assertIn("Unsupported architecture", self.install(False))
        self.assertFalse((self.root / "installed").exists())

    def test_platform_mappings(self):
        # Tiny executable fixtures exercise installer naming without executing
        # cross-compiled binaries on this Linux host.
        for system, machine, platform, suffix in (("Darwin", "aarch64", "darwin-arm64", ""),
                                                 ("MINGW64_NT", "AMD64", "windows-amd64", ".exe"),
                                                 ("MSYS_NT", "arm64", "windows-arm64", ".exe")):
            with self.subTest(platform=platform):
                self.shim("uname", f'#!/bin/sh\ncase "$1" in -s) echo {system};; -m) echo {machine};; esac\n')
                asset = "signetleaf-" + platform + suffix
                folder = self.fixture / "download/mapping"
                folder.mkdir(exist_ok=True)
                binary = b'#!/bin/sh\necho "signetleaf fixture"\n'
                (folder / asset).write_bytes(binary)
                (folder / "checksums.txt").write_text(hashlib.sha256(binary).hexdigest() + "  " + asset + "\n")
                self.env["SIGNETLEAF_VERSION"] = "mapping"
                self.assertIn("fixture", self.install())
                self.assertTrue((self.root / ("installed/signetleaf" + suffix)).exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
