// testsite serves the local website fixture for documentation and onboarding checks.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vincentsch/signetleaf/internal/testsite"
)

func main() {
	out := flag.String("out", "", "Existing directory for the site endpoint")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(1)
	}
	if e := run(*out); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(out string) error {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	server := &http.Server{Handler: testsite.Handler(), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	if e = os.WriteFile(filepath.Join(out, "endpoint"), []byte("http://"+listener.Addr().String()), 0600); e != nil {
		server.Close()
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		return server.Close()
	case e := <-done:
		return e
	}
}
