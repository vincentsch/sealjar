// Package demo prepares bundled inputs and a changed-file example.
package demo

import (
	"archive/zip"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vincentsch/sealjar/internal/testsite"
	"github.com/vincentsch/sealjar/internal/tsa/demotsa"
)

//go:generate go run ./generate
//go:embed all:data
var Files embed.FS

type Inputs struct {
	Repository, SiteURL, TimestampURL string
	cleanup                           []func()
}

func (in *Inputs) Close() {
	for i := len(in.cleanup) - 1; i >= 0; i-- {
		in.cleanup[i]()
	}
	in.cleanup = nil
}
func (in *Inputs) serve(handler http.Handler) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server := &http.Server{Handler: handler}
	in.cleanup = append(in.cleanup, func() { server.Close() })
	go server.Serve(listener)
	return "http://" + listener.Addr().String(), nil
}
func Start(ctx context.Context) (_ *Inputs, err error) {
	in := &Inputs{}
	defer func() {
		if err != nil {
			in.Close()
		}
	}()
	temp, err := os.MkdirTemp("", "sealjar-demo-")
	if err != nil {
		return nil, err
	}
	in.cleanup = append(in.cleanup, func() { os.RemoveAll(temp) })
	in.Repository = filepath.Join(temp, "bakery")
	err = fs.WalkDir(Files, "data/example-repo", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel := strings.TrimPrefix(name, "data/example-repo")
		dest := in.Repository + filepath.FromSlash(rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		b, e := Files.ReadFile(name)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	})
	if err != nil {
		return nil, err
	}
	git := func(args ...string) error {
		prefix := []string{"-C", in.Repository, "-c", "user.name=Sealjar Demo", "-c", "user.email=demo@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + filepath.Join(temp, "no-hooks")}
		cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
		// Suppress inherited identity overrides, without changing any Git config.
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GIT_AUTHOR_") && !strings.HasPrefix(v, "GIT_COMMITTER_") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		if _, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("demo Git %s failed: %w", args[0], e)
		}
		return nil
	}
	for _, args := range [][]string{{"init", "--quiet", "--template="}, {"add", "."}, {"commit", "--quiet", "--no-verify", "-m", "Add bakery website"}} {
		if err = git(args...); err != nil {
			return nil, err
		}
	}
	if err = os.WriteFile(filepath.Join(in.Repository, "README.md"), []byte("# Bergblick Bakery\n\nOpening hours updated before handover.\n"), 0600); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(in.Repository, ".env"), []byte("DEMO_ONLY=fake-secret-for-exclusion-check\n"), 0600); err != nil {
		return nil, err
	}
	if err = git("add", "-f", ".env"); err != nil {
		return nil, err
	}
	site, err := fs.Sub(Files, "data/site")
	if err != nil {
		return nil, err
	}
	in.SiteURL, err = in.serve(testsite.WithFiles(site))
	if err != nil {
		return nil, err
	}
	authority, err := demotsa.New()
	if err != nil {
		return nil, err
	}
	in.TimestampURL, err = in.serve(authority)
	if err != nil {
		return nil, err
	}
	return in, nil
}

// Tamper rewrites one screenshot while preserving a readable ZIP structure.
func Tamper(source, out string) (output, screenshot string, err error) {
	r, err := zip.OpenReader(source)
	if err != nil {
		return "", "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "web/") && strings.HasSuffix(f.Name, "/desktop.png") {
			screenshot = f.Name
			break
		}
	}
	if screenshot == "" {
		return "", "", fmt.Errorf("demo package has no desktop screenshot")
	}
	var dest *os.File
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = fmt.Sprintf("-%d", n)
		}
		output = filepath.Join(out, "demo-tampered"+suffix+".sealjar.zip")
		dest, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		break
	}
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			os.Remove(output)
		}
	}()
	z := zip.NewWriter(dest)
	for _, f := range r.File {
		h := f.FileHeader
		w, e := z.CreateHeader(&h)
		if e != nil {
			err = e
			break
		}
		src, e := f.Open()
		if e != nil {
			err = e
			break
		}
		if f.Name == screenshot {
			var first [1]byte
			_, e = io.ReadFull(src, first[:])
			if e == nil {
				first[0] ^= 1
				_, e = w.Write(first[:])
			}
		}
		if e == nil {
			_, e = io.Copy(w, src)
		}
		ce := src.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			err = e
			break
		}
	}
	ze := z.Close()
	de := dest.Close()
	if err == nil {
		err = ze
	}
	if err == nil {
		err = de
	}
	return output, screenshot, err
}
