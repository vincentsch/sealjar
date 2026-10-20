# Example site

A small static website used by Signetleaf's tests and demo. Serve it with the
test server in `internal/testsite`, which adds the behaviours a real site has:

- `/old-page` redirects (301) to `/`.
- `/private/` needs HTTP basic auth (user `client`, password `handover`).
- `/assets/photo4.jpg` is served with a 1.5 s delay (slow asset).
- `/missing.png` returns 404 (a failed asset is referenced nowhere by
  default; tests add it to check capture-quality reporting).
- Responses carry a `Set-Cookie` for the notice and normal cache headers.
