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

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- sealjar verify
- sealjar capture
