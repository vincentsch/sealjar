// Command generate-command-docs updates or checks the command reference.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vincentsch/rungrad/docsgen"
	"github.com/vincentsch/signetleaf/internal/cli"
)

func generate(check bool, dir string) error {
	verdict := 0
	app := cli.NewApp(&verdict)
	if !check {
		return docsgen.Write(app, dir)
	}
	result, err := docsgen.Check(app, dir)
	if err != nil {
		return err
	}
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
