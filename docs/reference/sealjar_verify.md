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

## Output modes

human, json

## Related commands

- sealjar capture
- sealjar restore
