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

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- sealjar verify
- sealjar restore
