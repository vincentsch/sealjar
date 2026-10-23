# Capture pages and code

[Install Sealjar](quickstart.md#install) first. A capture accepts listed HTTP(S)
URLs and paths in one Git repository. It does not crawl or prove that the captured
code produced the captured website. The repository needs a commit.

From the repository root, resolve an output directory outside it and show it
alongside the complete plan:

```sh
OUT=$(cd .. && pwd -P)
sealjar capture https://staging.example.com https://staging.example.com/prices/ . --out "$OUT" --dry-run
sealjar capture https://staging.example.com https://staging.example.com/prices/ . --out "$OUT" --confirm --no-prompt
```

Check that `OUT` is outside the repository after resolving links. Relative paths
start at the actual working directory. This keeps packages and reader folders out of
ordinary commits and later captures. The plan shows the absolute destination.
`--dry-run` makes no network request, starts no Chrome and writes nothing. Without
`--confirm`, Sealjar asks for confirmation in an interactive terminal. It refuses
unattended capture without that flag.

## Pages and hosts

Each URL gets desktop 1440×900 and mobile 390×844 views. Mobile screenshots have
twice the CSS resolution. `--tablet` adds 768×1024 at scale 2. Capture waits for
loading and ordinary network activity, scrolls for lazy images, then returns to the
top. WebSocket and EventSource connections do not prevent completion. `--wait 2s`
adds a wait after loading, up to 30 seconds. Full-page screenshots stop at 20,000
CSS pixels and record truncation. Cookie banners remain untouched.

For each page, the package contains:

- PNG screenshots at the selected screen sizes.
- The rendered desktop HTML (DOM) and an A4 page PDF.
- Network requests and response headers.
- Console messages and redirects.

It also records DNS details for each visited origin and certificate details for
HTTPS connections. Multiple hosts and ports remain separate. Captured DOM is the
rendered document, rather than an archive that can replay the site. HTTP pages have
no TLS certificate. These facts do not establish server authorship.

Capture records failed resources and non-2xx pages as problems. It also flags login
forms, timeouts and truncated screenshots. A failed target does not stop other
targets. If any target succeeds, a package is written. Ordinary capture problems do
not change exit 0; `--strict` returns 10 after writing. If every target fails, no
package is written and exit is 2. Read quality details even when verify exits 0.

## Authentication

Use an environment variable for real basic-auth credentials. Obtain it through your
normal secret input method, without saving or printing it. With that variable
already set:

```sh
sealjar capture https://staging.example.com/private/ --out .. --confirm --no-prompt
```

`SEALJAR_AUTH` and `--auth user:password` are sent only to listed target origins,
never another origin after a redirect or a third-party resource. Application
Authorization headers, such as bearer tokens, are preserved for requests the browser
permits, including API requests to another origin. Sealjar adds Basic credentials
only when a request has no application Authorization header. Do not put credentials
inside URLs. The [local fixture](contributor-testing.md) uses `--auth
client:handover` for testing.

A form login requires a person and an interactive terminal:

```sh
sealjar capture https://staging.example.com/account/ --login --out .. --confirm
```

Sign in in the isolated Chrome window, then press Enter in the terminal. Capture
reuses that session, including session cookies and session storage. It closes its
browser and deletes the temporary profile afterwards.

## Repository selection

Tracked files use their current working-tree bytes, including uncommitted edits.
Deleted tracked files are recorded as deleted. This example adds untracked files
that Git does not ignore. It also includes history and excludes selected files:

```sh
sealjar capture . --include-untracked --history --exclude 'build/' --exclude '*.key' --name handover --label client=Example --out .. --no-timestamp --confirm --no-prompt
```

History includes all refs in a separate Git bundle. It may contain old secrets and
unrelated work. Submodule contents are not included. Git LFS is never fetched;
materialised files are copied and pointers are marked as pointers. Symlinks are
never followed, and links pointing outside the repository are excluded. Untracked
nested repositories and linked worktrees are excluded too.

Built-in secret patterns cover `.env`, `.env.*`, key files and common credential
files. `.env.example` and `.env.dist` are allowed unless a user rule excludes them.
Only candidate files appear in exclusion lists; ordinary gitignored files were never
candidates. A pattern without `/` matches a basename at any depth; one with `/`
matches from the repository root using Go `path.Match`. A trailing `/` excludes the
matching folder tree. `**` is rejected. The plan lists exclusions with reasons.
Repository-only capture can write HTML without Chrome, but no PDFs.

## Privacy and network access

Capture contacts target pages and their resources, DNS and timestamp authorities. A
TSA receives a manifest hash and normal network metadata. Requests honour
`HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`; doctor's connection probes do not honour
proxies. `--no-timestamp` skips TSA requests, not website or DNS traffic. Dry runs
make no network requests. Verification works offline.

Network metadata masks Cookie, Set-Cookie, Authorization and Proxy-Authorization
values and known secret query values. No response bodies are stored as network
records. Masking and exclusions cannot remove private content from screenshots, DOM,
source or history. Captured files are preserved as captured. Inspect the package
before sharing it; Sealjar does not upload it for you.