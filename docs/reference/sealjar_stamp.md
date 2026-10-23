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

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- sealjar verify
- sealjar capture
