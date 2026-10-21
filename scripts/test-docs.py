#!/usr/bin/env python3
"""Check public guide links, media and the copied manual verification script."""
from pathlib import Path
import re
import struct
import unittest
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]


class DocumentationTests(unittest.TestCase):
    def test_local_links_and_media_exist(self):
        pages = [ROOT / 'README.md', ROOT / 'AGENTS.md', ROOT / 'CONTRIBUTING.md', *ROOT.glob('docs/**/*.md')]
        for page in pages:
            text = page.read_text(encoding="utf-8")
            links = re.findall(r'!?\[[^\]]*\]\(([^)]+)\)', text)
            links += re.findall(r'(?:src|srcset|href)="([^"]+)"', text)
            for link in links:
                if re.match(r'\w+://', link) or link.startswith('#'):
                    continue
                target = link.split('#')[0]
                with self.subTest(page=page.relative_to(ROOT), link=link):
                    self.assertTrue((page.parent / target).is_file(), link)

    def test_readme_screenshots_are_sized_for_reading(self):
        for name in ('demo-report.png', 'demo-seal.png'):
            with self.subTest(image=name):
                data = (ROOT / 'docs/media' / name).read_bytes()
                self.assertEqual(data[:8], b'\x89PNG\r\n\x1a\n')
                self.assertEqual(data[12:16], b'IHDR')
                self.assertEqual(struct.unpack('>II', data[16:24]), (1440, 1500))
                self.assertLess(len(data), 400_000)

    def test_manual_hash_check_matches_packaged_instructions(self):
        def python_block(path):
            return path.read_text(encoding="utf-8").split('```python\n')[1].split('```')[0]
        self.assertEqual(python_block(ROOT / 'docs/verify.md'),
                         python_block(ROOT / 'internal/pkg/VERIFY.txt'))

    def test_public_guides_avoid_forbidden_claims(self):
        forbidden = re.compile(r'court-proof|gerichtsfest|rechtssicher|beweissicher|legally binding', re.I)
        for page in [ROOT / 'AGENTS.md', ROOT / 'README.md', *ROOT.glob('docs/**/*.md')]:
            with self.subTest(page=page.relative_to(ROOT)):
                self.assertIsNone(forbidden.search(page.read_text(encoding="utf-8")))

    def test_brand_assets_are_self_contained_paths(self):
        for kind in ('mark', 'wordmark'):
            for mode in ('light', 'dark'):
                root = ET.parse(ROOT / f'docs/media/{kind}-{mode}.svg').getroot()
                self.assertTrue(root.findall('.//{http://www.w3.org/2000/svg}path'))
                self.assertFalse(root.findall('.//{http://www.w3.org/2000/svg}text'))
                self.assertFalse(root.findall('.//{http://www.w3.org/2000/svg}image'))


if __name__ == '__main__':
    unittest.main()
