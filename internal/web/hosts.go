package web

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
)

func Origin(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("Target needs an http:// or https:// URL with a host.")
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port), nil
}
func OriginID(origin string) string {
	h := sha256.Sum256([]byte(origin))
	return hex.EncodeToString(h[:])
}
func CollectHosts(ctx context.Context, work string, origins []string) ([]packagefmt.Host, error) {
	hosts := []packagefmt.Host{}
	dnsCache := map[string]map[string]any{}
	for _, origin := range origins {
		u, _ := url.Parse(origin)
		id := OriginID(origin)
		dir := filepath.Join(work, "hosts", id)
		if e := os.MkdirAll(dir, 0700); e != nil {
			return nil, e
		}
		hosts = append(hosts, packagefmt.Host{Origin: origin, OriginID: id})
		dns, ok := dnsCache[u.Hostname()]
		if !ok {
			dns = map[string]any{"resolver": "system"}
			dc, cancel := context.WithTimeout(ctx, 5*time.Second)
			ips, e := net.DefaultResolver.LookupIPAddr(dc, u.Hostname())
			a, aaaa := []string{}, []string{}
			for _, ip := range ips {
				if ip.IP.To4() != nil {
					a = append(a, ip.IP.String())
				} else {
					aaaa = append(aaaa, ip.IP.String())
				}
			}
			dns["a"] = a
			dns["aaaa"] = aaaa
			if e != nil {
				dns["address_error"] = "System address lookup failed"
			}
			cname, e := net.DefaultResolver.LookupCNAME(dc, u.Hostname())
			dns["cname"] = cname
			if e != nil {
				dns["cname_error"] = "System CNAME lookup failed"
			}
			cancel()
			dnsCache[u.Hostname()] = dns
		}
		record := map[string]any{"origin": origin}
		for k, v := range dns {
			record[k] = v
		}
		if e := writeJSON(filepath.Join(dir, "dns.json"), record); e != nil {
			return nil, e
		}
		facts := map[string]any{"origin": origin, "status": "Not applicable"}
		if u.Scheme == "https" {
			facts["status"] = "Unavailable"
			tc, cancel := context.WithTimeout(ctx, 5*time.Second)
			// This separate metadata connection must retain untrusted chains too.
			conn, e := (&tls.Dialer{Config: &tls.Config{ServerName: u.Hostname(), InsecureSkipVerify: true}}).DialContext(tc, "tcp", u.Host)
			cancel()
			if e != nil {
				facts["error"] = "TLS metadata connection failed"
			} else {
				state := conn.(*tls.Conn).ConnectionState()
				conn.Close()
				facts["status"] = "Collected"
				facts["version"] = tls.VersionName(state.Version)
				facts["cipher"] = tls.CipherSuiteName(state.CipherSuite)
				var chain []byte
				pool := x509.NewCertPool()
				for i, c := range state.PeerCertificates {
					chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
					if i > 0 {
						pool.AddCert(c)
					}
				}
				if len(state.PeerCertificates) > 0 {
					leaf := state.PeerCertificates[0]
					facts["not_before"] = leaf.NotBefore
					facts["not_after"] = leaf.NotAfter
					_, e = leaf.Verify(x509.VerifyOptions{DNSName: u.Hostname(), Intermediates: pool})
					facts["system_roots_trusted"] = e == nil
					if e != nil {
						facts["verification_error"] = "Certificate chain is not trusted by system roots or does not match this host"
					}
				}
				if e := os.WriteFile(filepath.Join(dir, "tls.pem"), chain, 0600); e != nil {
					return nil, e
				}
			}
		}
		if e := writeJSON(filepath.Join(dir, "tls.json"), facts); e != nil {
			return nil, e
		}
	}
	return hosts, nil
}
