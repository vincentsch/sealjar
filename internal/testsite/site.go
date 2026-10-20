// Package testsite serves the bakery fixture with redirects and login behavior.
package testsite

import (
	site "github.com/vincentsch/signetleaf/testdata/site"
	"io/fs"
	"net/http"
	"time"
)

func Handler() http.Handler {
	return WithFiles(site.Files)
}

// WithFiles serves the same fixture behaviours from bundled static files.
func WithFiles(source fs.FS) http.Handler {
	files := http.FileServer(http.FS(source))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		http.SetCookie(w, &http.Cookie{Name: "bakery_notice", Value: "seen", Path: "/", SameSite: http.SameSiteLaxMode})
		switch r.URL.Path {
		case "/old-page":
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		case "/private/":
			u, p, ok := r.BasicAuth()
			if !ok || u != "client" || p != "handover" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Bakery"`)
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("<!doctype html><title>Private page</title><h1>Login required</h1>"))
				return
			}
		case "/assets/photo4.jpg":
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		case "/missing.png":
			http.NotFound(w, r)
			return
		case "/login/":
			if r.Method == http.MethodPost {
				if e := r.ParseForm(); e != nil {
					http.Error(w, "Invalid form", 400)
					return
				}
				if r.Form.Get("user") != "client" || r.Form.Get("password") != "handover" {
					http.Error(w, "Login failed", 401)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "bakery_session", Value: "yes", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
				http.Redirect(w, r, "/account/", http.StatusSeeOther)
				return
			}
		case "/account/":
			c, e := r.Cookie("bakery_session")
			if e != nil || c.Value != "yes" {
				http.Redirect(w, r, "/login/", http.StatusFound)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}
