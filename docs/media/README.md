# Signetleaf images

The light and dark SVG wordmarks use path outlines from DejaVu Serif.
Their font license is in [FONT-LICENSE.txt](FONT-LICENSE.txt). The simple mark
combines a page and a leaf. Its colours are ink `#1D1D1B` and accent `#2C4A7A`;
the dark variant uses paper `#F4F3EF` for the lettering and outline.

`demo-report.png` and `demo-seal.png` show actual bundled demo output with its
trusted DigiCert timestamp. The report opening and bilingual seal were captured
from the paired reader folder at 1440×1500 pixels, then losslessly compressed
as PNGs. The report points to the seal and verify for timestamp results.

An offline demo can instead use a local token labelled “Demo timestamp, not
trusted”, giving verify exit 11. Its signing key is public. Even if an explicit
root grants it trust, it supplies no independent time evidence.

Generated command documentation lives separately
in [reference/](../reference/index.md).
