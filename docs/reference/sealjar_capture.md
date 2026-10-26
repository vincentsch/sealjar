# sealjar capture

Capture web pages and current files from one Git repository

## Usage

```
sealjar capture <target>...
```

## Examples

```
sealjar capture . --confirm --no-timestamp --out ../sealjar-captures
sealjar capture . --dry-run --no-timestamp --include-untracked --out ../sealjar-captures
```

## Flags

- `--auth` Send basic auth user:password only to listed target origins (or SEALJAR_AUTH)
- `--chrome` Use this Chrome or Chromium executable
- `--confirm` Capture without asking to continue
- `--exclude` Exclude a path.Match pattern; ** is not supported (repeatable)
- `--history` Include all Git history as a separate bundle; it can contain old secrets
- `--include-untracked` Include untracked files that Git does not ignore
- `--label` Record a label as key=value (repeatable)
- `--lang` Report language: en, de or both
- `--login` Open Chrome visibly for login, then press Enter in this terminal
- `--name` Use this package name (letters, numbers, hyphens and underscores)
- `--no-timestamp` Create a package without a timestamp
- `--out` Write the package in this folder
- `--strict` Exit 10 after writing if a target has problems or requested timestamping fails
- `--tablet` Also capture a 768×1024 tablet viewport at scale 2
- `--tsa` Request this timestamp authority instead of defaults (repeatable)
- `--tsa-root` Add trusted timestamp roots from a PEM file
- `--wait` Wait extra after load (maximum 30s)

## Output

Defaults to human-readable output. Use `--json` for JSON.

## Preview changes

`--dry-run` previews changes without writing files.

## Related commands

- sealjar verify
- sealjar restore
