# Example site

A small static website used by Sealjar's tests and demo. Serve it with the
test server in `internal/testsite`, which adds the behaviours a real site has:

- `/old-page` redirects (301) to `/`.
- `/private/` needs HTTP basic auth (user `client`, password `handover`).
- `/assets/photo4.jpg` is served with a 1.5 s delay (slow asset).
- `/missing.png` returns 404 (a failed asset is referenced nowhere by
  default; tests add it to check capture-quality reporting).
- Responses carry a `Set-Cookie` for the notice and normal cache headers.
- `/qa/late/` (test page, not linked from the site): text set 800 ms after
  the load event and an image (`photo3.jpg`, `loading="lazy"`) below a
  6,000 px spacer. A good capture shows "Fresh bread from 7 am." and the
  decoded image.
- `/login/` and `/account/` (test pages for `--login`): a `POST` to
  `/login/` with user `client` and password `handover` sets a session cookie
  `bakery_session` (no expiry) and redirects to `/account/`; the form also
  sets `sessionStorage`. `/account/` without the cookie redirects to
  `/login/`; with it, the page shows "Logged in. 2 open orders." only when
  the session storage survived too.
