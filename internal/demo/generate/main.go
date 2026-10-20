// Command generate copies tracked fixtures into the demo's embedded data.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate() error {
	root, err := filepath.Abs("../..")
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--", "testdata/site", "testdata/example-repo")
	files, err := cmd.Output()
	if err != nil {
		return err
	}
	if err = os.RemoveAll("data"); err != nil {
		return err
	}
	for _, entry := range bytes.Split(files, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		name := string(entry)
		dest := filepath.Join("data", name[len("testdata/"):])
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(dest, b, 0644); e != nil {
			return e
		}
	}
	return nil
}
