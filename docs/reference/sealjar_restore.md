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

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- sealjar capture
- sealjar verify
