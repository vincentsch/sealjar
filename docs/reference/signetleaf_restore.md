# signetleaf restore

Restore checked repository files into an empty folder

## Usage

```
signetleaf restore <package>
```

## Examples

```
signetleaf restore capture.signetleaf.zip --to recovered
signetleaf restore capture.signetleaf.zip --to recovered --dry-run
```

## Flags

- `--to` Restore into this new or empty folder
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- signetleaf capture
- signetleaf verify
