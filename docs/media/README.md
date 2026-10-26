# Sealjar images

The SVG wordmarks use Figtree at weight 650, converted to path outlines. Its
Open Font License is in [FONT-LICENSE.txt](FONT-LICENSE.txt). The mark is a
sealed jar on blue `#155BD9`. The lid uses `#9DBDFF`; the jar is white.
Lettering is `#1D1D21` on light backgrounds and white on dark ones.

The website's `src/lib/brand.ts` holds the shared paths. After changing them,
run `npm run brand:generate` there and copy the exports here. The site's
`wordmark.svg` becomes `wordmark-light.svg` here, and the site's
`wordmark-light.svg` becomes `wordmark-dark.svg`. Both mark files here come from
the site's `mark.svg`. The website's tests compare these files when the product
checkout is available.

`demo-report-v0.1.0.png` and `demo-seal-v0.1.0.png` show real output of the
bundled demo with its trusted DigiCert timestamp. The report opening and the
bilingual seal were captured from the reader folder in a 1440×1500 window; the
seal is cropped to its content. Both are losslessly compressed PNGs from public
release `v0.1.0` (`0711fe2`), captured with an `en-US` locale. Headless Chrome
used the same locale and device scale 1. Fonts and linked images were loaded
before each screenshot was taken from the top of the page. Compression kept the
decoded pixels unchanged. The report points to the seal and to verify for the
timestamp result.

Without a public timestamp authority, the demo signs with a local key and
labels the token “Demo timestamp, not trusted”. Verify then exits 11. The key
is public, so the token says nothing about time, even if an explicit root
trusts it.

The generated command documentation lives separately in
[reference/](../reference/index.md).
