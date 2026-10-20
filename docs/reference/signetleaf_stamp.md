# signetleaf stamp

Add timestamps without changing captured evidence

## Usage

```
signetleaf stamp <package>
```

## Examples

```
signetleaf stamp capture.signetleaf.zip
signetleaf stamp capture.signetleaf.zip --dry-run
```

## Flags

- `--tsa` Request this timestamp authority instead of defaults (repeatable)
- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- signetleaf verify
- signetleaf capture
