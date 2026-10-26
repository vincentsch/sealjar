# Capture pages and code

Sealjar is a free, open-source CLI. A capture takes the URLs you list and the
working tree of one Git repository and seals them in a timestamped ZIP. Sealjar
does not follow links, and a package cannot show that the captured code
produced the captured website. The repository needs at least one commit.

From the repository root, pick an output directory outside it and look at the
plan first:

```sh
OUT=$(cd .. && pwd -P)
sealjar capture https://staging.example.com https://staging.example.com/prices/ . --out "$OUT" --dry-run
```

Check the plan, including the exclusions and the absolute output directory. If
it is right, run the capture:

```sh
sealjar capture https://staging.example.com https://staging.example.com/prices/ . --out "$OUT" --confirm --no-prompt
```

Keep `OUT` outside the repository, also after resolving symlinks. Relative paths
start at your current directory. Otherwise the package and the reader folder
would end up in your next commit or your next capture. The plan prints the
absolute destination. `--dry-run` starts no Chrome, makes no network request and
writes nothing. Without `--confirm`, Sealjar asks for confirmation in an
interactive terminal and refuses to run unattended.

## Pages and hosts

Each URL is loaded at 1440×900 for desktop and at 390×844 for mobile. Mobile
screenshots use twice the CSS resolution. `--tablet` adds 768×1024 at scale 2.
Capture waits for the page to load and the network to go quiet. It scrolls down
so lazy images load, then scrolls back to the top. Open WebSocket and
EventSource connections do not block it. `--wait 2s` adds a pause after loading,
up to 30 seconds. Full-page screenshots stop at 20,000 CSS pixels, and the
package records when that happened. Cookie banners are left as they are.

For each page, the package holds:

- PNG screenshots at the selected screen sizes.
- The rendered desktop HTML (DOM) and an A4 page PDF.
- Network requests and response headers.
- Console messages and redirects.

It also records DNS details for each origin and the certificate details of each
HTTPS connection. Each host and port is kept separately. The saved HTML is the
rendered document; it cannot replay the site like a web archive. HTTP pages have
no certificate. None of this shows who ran the server.

Capture records failed resources and non-2xx pages as problems. It also flags
login forms, timeouts and truncated screenshots. One failed target does not stop
the others, and a package is written if any target succeeds. Ordinary capture
problems keep exit 0; with `--strict` the exit code is 10 after the package is
written. If every target fails, nothing is written and the exit code is 2. Read
the quality details even when verify exits 0.

## Authentication

For HTTP basic auth, put the credentials in an environment variable. Get them
from your normal secret store without saving or printing them. With the
variable set:

```sh
sealjar capture https://staging.example.com/private/ --out .. --confirm --no-prompt
```

`SEALJAR_AUTH` and `--auth user:password` go only to the origins you listed,
never to another origin after a redirect and never to a third-party resource.
Authorization headers that the application sets itself, such as bearer tokens
for an API, are left alone, also for requests to another origin. Sealjar adds
the basic credentials only when a request has no Authorization header. Do not
put credentials in URLs. The [local fixture](contributor-testing.md) uses
`--auth client:handover` for tests.

A form login needs a person at the keyboard and an interactive terminal:

```sh
sealjar capture https://staging.example.com/account/ --login --out .. --confirm
```

Sign in in the Chrome window that opens, then press Enter in the terminal.
Capture reuses that session, including its cookies and session storage.
Afterwards it closes the browser and deletes the temporary profile.

## Repository selection

Tracked files are copied as they are in your working tree, including
uncommitted edits. Deleted tracked files are recorded as deleted. This example
adds untracked files that Git does not ignore, includes the history and
excludes some files:

```sh
sealjar capture . --include-untracked --history --exclude 'build/' --exclude '*.key' --name handover --label client=Example --out .. --no-timestamp --confirm --no-prompt
```

History goes into a separate Git bundle with all refs. It can contain old
secrets and unrelated work. Submodules are listed without their contents. Git LFS is never fetched. Files that are already present are copied, and pointers
stay pointers. Symlinks are never followed, and links that point outside the
repository are left out. Untracked nested repositories and linked worktrees are
left out too.

The built-in secret patterns cover `.env`, `.env.*`, key files and common
credential files. `.env.example` and `.env.dist` are allowed unless you exclude
them. The exclusion list shows only files that were candidates; files Git
ignores never were. A pattern without `/` matches a file name at any depth. A
pattern with `/` matches from the repository root with Go's `path.Match`. A
trailing `/` excludes the whole folder. `**` is rejected. The plan lists every
exclusion with its reason. A code-only capture without Chrome writes HTML
reports but no PDFs.

## Privacy and network access

Capture loads the listed pages with everything they load. It asks DNS servers
about the hosts and sends the manifest hash to the timestamp authority, which
receives that hash and normal network metadata. Requests use `HTTP_PROXY`,
`HTTPS_PROXY` and `NO_PROXY`; doctor's connection probes do not.
`--no-timestamp` skips the timestamp request, not the website or DNS traffic. A
dry run makes no network request at all. Verification works offline.

In the saved network records, Sealjar masks the values of the Cookie,
Set-Cookie, Authorization and Proxy-Authorization headers and of known secret
query parameters. Response bodies are not stored as network records. Masking and
exclusions cannot remove all private content. Screenshots and HTML can show it,
and source files and Git history can contain it. Captured files are stored as captured. Look through the package before you share it. Sealjar never
uploads it.
