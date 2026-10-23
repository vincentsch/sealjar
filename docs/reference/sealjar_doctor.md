# sealjar doctor

Check Chrome, Git, output access and timestamp authority connections

## Usage

```
sealjar doctor
```

## Examples

```
sealjar doctor
sealjar doctor --tsa https://freetsa.org/tsr
```

## Flags

- `--chrome` Check this Chrome or Chromium executable
- `--out` Check this output folder for write access
- `--tsa` Check this timestamp authority instead of defaults (repeatable)

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Related commands

- sealjar capture
- sealjar stamp
