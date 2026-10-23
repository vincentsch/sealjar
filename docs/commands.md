# Commands

[Install Sealjar](quickstart.md#install) before running these commands.

The [command reference](reference/index.md) comes from the CLI's command tree. Run
`sealjar <command> --help` for the same flags and examples in your terminal.

When capturing a repository from its root, write packages outside it:

```sh
sealjar capture . --dry-run --out ../sealjar-captures
sealjar capture . --confirm --no-prompt --out ../sealjar-captures
```

This keeps generated packages out of later snapshots, including captures with
`--include-untracked`.

Use `--confirm --no-prompt` for an authorised unattended capture, after inspecting
the full dry run, including its resolved absolute output directory.

| Command | Daily use |
| --- | --- |
| `doctor` | Find missing prerequisites and check output access or TSA connections. |
| `demo` | Capture the bundled bakery and example code, with a tampered copy to check. |
| `capture` | Capture listed pages and one repository; no crawling. |
| `verify` | Check the files and timestamp; report capture problems separately. |
| `stamp` | Add timestamp tokens in a new ZIP without changing the manifest. |
| `report` | Copy stored reports and regenerate the seal from checked data. |
| `restore` | Restore checked source files into a new or empty folder. |

The [capture guide](capture.md) explains scope and exclusions.
[Verification](verify.md) covers exit codes 10 and 11. For custom roots, see
[timestamps](timestamps.md). The [package format](package-format.md) includes export
and restore examples. `--json` prints one result object. `--dry-run` on writing
commands writes nothing, starts no Chrome and makes no network requests. `--login`
requires an interactive person.