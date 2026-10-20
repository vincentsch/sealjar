# Commands

The [command reference](reference/index.md) comes from the CLI's command tree.
Run `signetleaf <command> --help` for the same flags and examples in your terminal.

When capturing a repository from its root, write packages outside it:

```sh
signetleaf capture . --dry-run --out ../signetleaf-captures
signetleaf capture . --confirm --out ../signetleaf-captures
```

This keeps generated packages out of later snapshots, including captures with
`--include-untracked`.
