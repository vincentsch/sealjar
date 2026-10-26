# sealjar stamp

Add timestamps without changing captured evidence

## Usage

```
sealjar stamp <package>
```

## Examples

```
sealjar stamp capture.sealjar.zip
sealjar stamp capture.sealjar.zip --dry-run
```

## Flags

- `--tsa` Request this timestamp authority instead of defaults (repeatable)
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output

Defaults to human-readable output. Use `--json` for JSON.

## Preview changes

`--dry-run` previews changes without writing files.

## Related commands

- sealjar verify
- sealjar capture
