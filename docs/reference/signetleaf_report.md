# signetleaf report

Copy stored reports after checking package integrity and timestamp trust

## Usage

```
signetleaf report <package>
```

## Examples

```
signetleaf report capture.signetleaf.zip --out reader
signetleaf report capture.signetleaf.zip --lang de --pdf --dry-run
```

## Flags

- `--lang` Report language: en, de or both
- `--out` Write copies to this new reader folder
- `--pdf` Include stored PDFs (no browser required)
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- signetleaf verify
- signetleaf restore
