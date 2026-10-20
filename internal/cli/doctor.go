package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/signetleaf/internal/browser"
	"github.com/vincentsch/signetleaf/internal/tsa"
)

// This diagnostic checks connections only; it does not request a timestamp.
func doctorCommand() *rungrad.Command {
	var authorities []string
	var chrome, out string
	c := command("doctor", "Check Chrome, Git, output access and timestamp authority connections")
	c.Args = cobra.NoArgs
	c.Examples = []string{"signetleaf doctor", "signetleaf doctor --tsa https://freetsa.org/tsr"}
	c.Related = []string{"signetleaf capture", "signetleaf stamp"}
	c.Configure = func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&chrome, "chrome", "", "Check this Chrome or Chromium executable")
		cmd.Flags().StringVar(&out, "out", ".", "Check this output folder for write access")
		cmd.Flags().StringArrayVar(&authorities, "tsa", nil, "Check this timestamp authority instead of defaults (repeatable)")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		list, e := tsa.Authorities(authorities)
		if e != nil {
			return failure(1, e)
		}
		type check struct {
			URL       string `json:"url"`
			Reachable bool   `json:"reachable"`
			Detail    string `json:"detail"`
			Fix       string `json:"fix,omitempty"`
		}
		result := struct {
			Prerequisites []check `json:"prerequisites"`
			Authorities   []check `json:"timestamp_authorities"`
			Limitation    string  `json:"limitation"`
		}{Authorities: []check{}, Limitation: "A connection check sends no request and does not prove a timestamp would be issued."}
		path, err := browser.Find(chrome)
		cr := check{URL: "Chrome", Reachable: err == nil}
		if err == nil {
			v, e := browser.Version(cmd.Context(), path)
			cr.Detail = displayPath(path) + ": " + strings.TrimSpace(v)
			cr.Reachable = e == nil
		}
		if !cr.Reachable {
			cr.Detail = "Chrome is unavailable"
			cr.Fix = "Install Google Chrome or Chromium, or pass --chrome <path>."
		}
		result.Prerequisites = append(result.Prerequisites, cr)
		gc, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		version, err := exec.CommandContext(gc, "git", "--version").Output()
		cancel()
		gr := check{URL: "Git", Reachable: err == nil, Detail: strings.TrimSpace(string(version))}
		if err != nil {
			gr.Detail = "Git is unavailable"
			gr.Fix = "Install Git to capture repositories."
		}
		result.Prerequisites = append(result.Prerequisites, gr)
		err = nil
		folder := out
		for {
			info, e := os.Stat(folder)
			if e == nil {
				if !info.IsDir() {
					err = fmt.Errorf("not a directory")
				}
				break
			}
			if !os.IsNotExist(e) {
				err = e
				break
			}
			parent := filepath.Dir(folder)
			if parent == folder {
				err = e
				break
			}
			folder = parent
			err = nil
		}
		var probe *os.File
		if err == nil {
			probe, err = os.CreateTemp(folder, ".signetleaf-doctor-")
			if err == nil {
				name := probe.Name()
				err = probe.Close()
				_ = os.Remove(name)
			}
		}
		wr := check{URL: "Output", Reachable: err == nil, Detail: "Output folder is writable"}
		if err != nil {
			wr.Detail = "Output folder is not writable"
			wr.Fix = "Choose a writable directory with --out <dir>."
		}
		result.Prerequisites = append(result.Prerequisites, wr)
		for _, endpoint := range list {
			u, _ := url.Parse(endpoint)
			port := u.Port()
			if port == "" {
				port = "80"
				if u.Scheme == "https" {
					port = "443"
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			var conn net.Conn
			var err error
			address := net.JoinHostPort(u.Hostname(), port)
			if u.Scheme == "https" {
				conn, err = (&tls.Dialer{Config: &tls.Config{ServerName: u.Hostname()}}).DialContext(ctx, "tcp", address)
			} else {
				conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
			}
			cancel()
			r := check{URL: endpoint, Reachable: err == nil, Detail: "Connection available"}
			if err != nil {
				r.Detail = "Connection failed."
				r.Fix = "Check the endpoint, network, TLS certificates and firewall."
			} else {
				conn.Close()
			}
			result.Authorities = append(result.Authorities, r)
		}
		return f.WriteResult(result, func(w io.Writer) {
			for _, r := range append(result.Prerequisites, result.Authorities...) {
				fmt.Fprintf(w, "%s: %s\n", r.URL, r.Detail)
				if r.Fix != "" {
					fmt.Fprintln(w, "  Fix: "+r.Fix)
				}
			}
			fmt.Fprintln(w, result.Limitation)
		})
	}
	return c
}
