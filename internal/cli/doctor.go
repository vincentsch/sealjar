package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/signetleaf/internal/tsa"
)

// This diagnostic checks connections only; it does not request a timestamp.
func doctorCommand() *rungrad.Command {
	var authorities []string
	c := command("doctor", "Check timestamp authority connections")
	c.Args = cobra.NoArgs
	c.Examples = []string{"signetleaf doctor", "signetleaf doctor --tsa https://freetsa.org/tsr"}
	c.Related = []string{"signetleaf capture", "signetleaf stamp"}
	c.Configure = func(cmd *cobra.Command) {
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
		}
		result := struct {
			Authorities []check `json:"timestamp_authorities"`
			Limitation  string  `json:"limitation"`
		}{Authorities: []check{}, Limitation: "A connection check sends no request and does not prove a timestamp would be issued."}
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
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
			cancel()
			r := check{URL: endpoint, Reachable: err == nil, Detail: "Connection available"}
			if err != nil {
				r.Detail = "Connection failed. Check the endpoint, network and firewall."
			} else {
				conn.Close()
			}
			result.Authorities = append(result.Authorities, r)
		}
		return f.WriteResult(result, func(w io.Writer) {
			for _, r := range result.Authorities {
				fmt.Fprintf(w, "%s: %s\n", r.URL, r.Detail)
			}
			fmt.Fprintln(w, result.Limitation)
		})
	}
	return c
}
