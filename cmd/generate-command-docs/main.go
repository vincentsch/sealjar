// Command generate-command-docs updates or checks the command reference.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vincentsch/rungrad/docsgen"
	"github.com/vincentsch/sealjar/internal/cli"
)

func generate(check bool, dir string) error {
	verdict := 0
	app := cli.NewApp(&verdict)
	// Tighten rungrad's generic prose without changing command help or examples.
	prose := strings.NewReplacer(
		"Shows text you can read in your terminal.\n\nUse --json to get JSON for scripts and other programs.",
		"Defaults to human-readable output. Use `--json` for JSON.",
		"This command makes changes. Use `--dry-run` to see what would change without changing anything.",
		"`--dry-run` previews changes without writing files.",
	)
	pages := docsgen.Generate(app)
	for name, content := range pages {
		pages[name] = prose.Replace(content)
	}
	if !check {
		if err := docsgen.Write(app, dir); err != nil {
			return err
		}
		for name, content := range pages {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
				return err
			}
		}
		return nil
	}
	result, err := docsgen.Check(app, dir)
	if err != nil {
		return err
	}
	stale := result.Stale[:0]
	for name := range pages {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if string(content) != pages[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	result.Stale = stale
	if !result.OK() {
		return fmt.Errorf("command reference differs: missing %v; stale %v; orphaned %v. Run make docs", result.Missing, result.Stale, result.Orphaned)
	}
	return nil
}

func main() {
	check := flag.Bool("check", false, "check generated command docs without changing files")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Use --check or run without arguments to write docs/reference.")
		os.Exit(1)
	}
	if err := generate(*check, "docs/reference"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
