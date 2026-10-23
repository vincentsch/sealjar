#!/usr/bin/env python3
"""Give a cold native runner's browser its first start before timed Go tests."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

candidates = [shutil.which(name) for name in ("google-chrome", "chromium", "chrome")]
candidates += [
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    str(Path(os.environ.get("PROGRAMFILES", "C:/Program Files")) / "Google/Chrome/Application/chrome.exe"),
]
chrome = next((path for path in candidates if path and Path(path).is_file()), None)
if chrome is None:
    raise SystemExit("Chrome is required for native browser tests")
with tempfile.TemporaryDirectory(prefix="sealjar-ci-chrome-") as profile:
    result = subprocess.run(
        [chrome, "--headless=new", "--no-first-run", "--no-default-browser-check",
         "--disable-gpu", "--user-data-dir=" + profile, "--dump-dom",
         "data:text/html,<title>sealjar-ci</title><p>ready</p>"],
        capture_output=True, text=True, timeout=60,
    )
    if result.returncode != 0 or "<p>ready</p>" not in result.stdout:
        raise SystemExit("Chrome warmup failed: " + result.stderr[-3000:])
print("Chrome cold start completed before native tests")
