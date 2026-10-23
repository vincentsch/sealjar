# sealjar restore

Restore checked repository files into an empty folder

## Usage

```
sealjar restore <package>
```

## Examples

```
sealjar restore capture.sealjar.zip --to recovered
sealjar restore capture.sealjar.zip --to recovered --dry-run
```

## Flags

- `--to` Restore into this new or empty folder
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- sealjar capture
- sealjar verify
