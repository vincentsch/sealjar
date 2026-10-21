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

## Output modes

human, json

## Related commands

- sealjar capture
- sealjar stamp
