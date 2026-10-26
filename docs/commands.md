# Commands

Sealjar is a free, open-source CLI with seven commands. The
[command reference](reference/index.md) is generated from the CLI itself, and
`sealjar <command> --help` prints the same flags and examples.

When you capture a repository from its root, write the package outside it:

```sh
sealjar capture . --dry-run --out ../sealjar-captures
```

Check the plan, including the exclusions and the absolute output directory. If
it is right, run:

```sh
sealjar capture . --confirm --no-prompt --out ../sealjar-captures
```

That keeps packages out of later captures, also with `--include-untracked`.
`--confirm --no-prompt` runs a capture unattended. Use it only after reading
the full dry run.

| Command | What it does |
| --- | --- |
| `doctor` | Checks for Chrome and Git, output access and the connections to the timestamp authorities. |
| `demo` | Captures the bundled bakery site and example code, with a tampered copy to check. |
| `capture` | Captures the listed pages and one repository. No crawling. |
| `verify` | Checks the files and the timestamp. Reports capture problems separately. |
| `stamp` | Adds timestamp tokens in a new ZIP without changing the manifest. |
| `report` | Copies the stored reports and writes a fresh seal page from checked data. |
| `restore` | Writes the checked source files into a new or empty folder. |

The [capture guide](capture.md) explains scope and exclusions.
[Verification](verify.md) explains exit codes 10 and 11.
[Timestamps](timestamps.md) covers your own authorities and roots. The
[package format](package-format.md) has the report and restore examples.
`--json` prints one result object. `--dry-run` on a writing command writes
nothing, starts no Chrome and makes no network request. `--login` needs a
person at the keyboard.
