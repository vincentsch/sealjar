#!/usr/bin/env python3
"""Print the changelog section for a release tag, refusing missing sections."""
import re
import sys
from pathlib import Path


def extract(text, tag):
    sections = re.split(r"(?m)^## (v\d+\.\d+\.\d+[^\n]*)\n", text)
    for heading, body in zip(sections[1::2], sections[2::2]):
        if re.fullmatch(re.escape(tag) + r"(?: - unreleased)?\s*", heading):
            return body.strip() + "\n"
    raise ValueError(f"No changelog section for {tag}. Add it to CHANGELOG.md before releasing.")


if __name__ == "__main__":
    if len(sys.argv) != 2 or not re.fullmatch(r"v\d+\.\d+\.\d+", sys.argv[1]):
        sys.exit("Usage: python3 scripts/release-notes.py vX.Y.Z")
    try:
        print(extract(Path("CHANGELOG.md").read_text(), sys.argv[1]), end="")
    except (ValueError, OSError) as error:
        sys.exit(str(error))
