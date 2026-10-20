# Commands

[Install Signetleaf](quickstart.md#install) before running these commands.

The [command reference](reference/index.md) comes from the CLI's command tree.
Run `signetleaf <command> --help` for the same flags and examples in your terminal.

When capturing a repository from its root, write packages outside it:

```sh
signetleaf capture . --dry-run --out ../signetleaf-captures
signetleaf capture . --confirm --no-prompt --out ../signetleaf-captures
```

This keeps generated packages out of later snapshots, including captures with
`--include-untracked`.

Use `--confirm --no-prompt` for an authorised unattended capture, after inspecting
the full dry run. Show the resolved output directory separately; it is not in
the CLI plan.

| Command | Daily use |
| --- | --- |
| `doctor` | Check Chrome, Git, output access and direct TSA connections. |
| `demo` | Capture the bundled bakery and example code, with a tampered copy to check. |
| `capture` | Capture listed pages and one repository; no crawling. |
| `verify` | Read capture quality, manifest-file consistency and timestamp trust separately. |
| `stamp` | Add timestamp tokens in a new ZIP without changing the manifest. |
| `report` | Copy stored reports and regenerate the seal from checked data. |
| `restore` | Restore checked source files into a new or empty folder. |

See [capture](capture.md) for scope and exclusions, [verify](verify.md) for exits
10 and 11, [timestamps](timestamps.md) for custom roots, and
[package format](package-format.md) for export and restore examples.
`--json` prints one result object. `--dry-run` on writing commands writes nothing,
starts no Chrome and makes no network requests. `--login` requires an interactive
person. Generated reference files stay inside [reference/](reference/index.md).
