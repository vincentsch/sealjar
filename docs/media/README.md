# Sealjar images

The SVG wordmarks use Figtree at weight 650, converted to path outlines.
Its Open Font License is in [FONT-LICENSE.txt](FONT-LICENSE.txt). The mark is a
sealed jar on blue `#155BD9`. The lid uses `#9DBDFF`; the jar is white.
Light-background lettering is `#1D1D21`; dark-background lettering is white.

The website's `src/lib/brand.ts` holds the shared paths. After changing them,
run `npm run brand:generate` there and copy the exports here. Site
`wordmark.svg` becomes `wordmark-light.svg` here; site `wordmark-light.svg`
becomes `wordmark-dark.svg`. Both product mark files use site `mark.svg`.
The website's tests compare these files when the product checkout is available.

`demo-report-v0.1.0.png` and `demo-seal-v0.1.0.png` show actual bundled demo output with its
trusted DigiCert timestamp. The report opening and bilingual seal were captured
from the paired reader folder in a 1440×1500 window; the seal is cropped to its
content. Both are losslessly compressed PNGs from public release `v0.1.0` (`0711fe2`)
and an `en-US` capture locale.
Headless Chrome used the same locale and device scale 1, with fonts and linked
images loaded before taking each screenshot from the page top. Compression
preserved the decoded pixels. The report points to the seal and verify for
timestamp results.

An offline demo can instead use a local token labelled “Demo timestamp, not
trusted”, giving verify exit 11. Its signing key is public. Even if an explicit
root grants it trust, it supplies no independent time evidence.

Generated command documentation lives separately
in [reference/](../reference/index.md).
