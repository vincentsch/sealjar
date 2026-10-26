# sealjar report

Copy stored reports after checking package integrity and timestamp trust

## Usage

```
sealjar report <package>
```

## Examples

```
sealjar report capture.sealjar.zip --out reader
sealjar report capture.sealjar.zip --lang de --pdf --out reader-de --dry-run
```

## Flags

- `--lang` Report language: en, de or both
- `--out` Write copies to this new reader folder
- `--pdf` Include stored PDFs (no browser required)
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output

Defaults to human-readable output. Use `--json` for JSON.

## Preview changes

`--dry-run` previews changes without writing files.

## Related commands

- sealjar verify
- sealjar restore
