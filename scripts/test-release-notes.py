import importlib.util
from pathlib import Path
import unittest
import sys

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location("release_notes", Path(__file__).with_name("release-notes.py"))
notes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notes)


class ReleaseNotesTests(unittest.TestCase):
    def test_matching_section_stops_at_next_version(self):
        text = "# Changes\n\n## v0.2.0\n\nNew.\n\n## v0.1.0 - unreleased\n\nFirst.\n\n### Fixes\n\nFixed.\n\n## v0.0.1\n\nOld.\n"
        self.assertEqual(notes.extract(text, "v0.1.0"), "First.\n\n### Fixes\n\nFixed.\n")
        self.assertEqual(notes.extract(text, "v0.2.0"), "New.\n")
        with self.assertRaisesRegex(ValueError, "No changelog section"):
            notes.extract(text, "v9.9.9")


if __name__ == "__main__":
    unittest.main()
