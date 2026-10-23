# sealjar verify

Check capture results, file hashes and timestamp status

## Usage

```
sealjar verify <package>
```

## Examples

```
sealjar verify capture.sealjar.zip
sealjar verify capture.sealjar.zip --json
```

## Flags

- `--tsa-root` Add trusted timestamp roots from a PEM file

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Related commands

- sealjar capture
- sealjar restore
