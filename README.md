# kit-fff-enforcer

A [kit](https://go-kit.dev/) extension that **blocks** the built-in `grep`
and `find` core tools — and shell invocations of the `grep`/`find`
binaries — whenever the current directory is a git work tree, steering the
model to the [fff](https://github.com/dmtrkovalenko/fff) MCP tools
(`fff__grep`, `fff__multi_grep`, `fff__find_files`) instead.

Outside a git repo (where fff has nothing indexed) `grep`/`find`/`shell`
are left alone — there's no fallback to fail into.

## Why

kit ships fast built-in `grep`/`find` core tools, but
[fff](https://github.com/dmtrkovalenko/fff) is a purpose-built file finder
that keeps a live, git-aware index of the repo and answers both file-name
and content searches far more precisely once it's registered as an MCP
server. A reminder in `AGENTS.md` is easy for a model to skip under
pressure; this extension makes the preference load-bearing instead of
advisory.

## Install

Register [fff](https://github.com/dmtrkovalenko/fff-mcp) as an MCP server
in `~/.kit.yml` first (see kit's
[MCP server configuration](https://go-kit.dev/configuration#mcp-servers)):

```yaml
mcpServers:
  fff:
    type: "local"
    command: ["/path/to/fff-mcp"]
```

Then install this extension:

```bash
kit install github.com/mgoodness/kit-fff-enforcer
```

Or, pinned to a specific released version (recommended, since the
blocking rules below have changed between releases and may again):

```bash
kit install github.com/mgoodness/kit-fff-enforcer@v0.1.0
```

Or, for a project-local install:

```bash
kit install -l github.com/mgoodness/kit-fff-enforcer
```

Or skip `kit install` entirely and drop `enforce-fff.go` directly into
`~/.config/kit/extensions/` (user-level, auto-discovered) or
`.kit/extensions/` (project-local).

## What it blocks

| Call | In a git work tree | Otherwise |
|---|---|---|
| built-in `grep` tool | blocked → use `fff__grep` / `fff__multi_grep` | allowed |
| built-in `find` tool | blocked → use `fff__find_files` | allowed |
| `shell` running a bare `grep`/`find` (as its own command, not a substring like `sgrep` or `findutils`) | blocked | allowed |

Blocked calls return a `Reason` that tells the model exactly which fff
tool to use instead — models reliably self-correct on the next tool call
rather than getting stuck.

### Exception: `grep` fed by a network fetch

`fff` only has repo files indexed — it has nothing indexed for data
pulled from the network, so blocking `grep` in that case leaves the
model with no working alternative. Shell `grep` is **not** blocked when
it's filtering the output of `curl`, `wget`, or an `http(s)` client:

| Example | Blocked? |
|---|---|
| `cat file \| grep foo` | blocked (real repo file) |
| `curl ... \| grep foo` | **allowed** (network response, not indexed) |
| `curl ... \| jq ... \| grep foo` | **allowed** (chain still originates at curl) |
| `grep foo <(curl ...)` / `grep foo "$(curl ...)"` | **allowed** (process/command substitution of a fetch) |
| `grep foo <(cat file)` | blocked (still a real repo file underneath) |

This exception is **`grep`-only**. `find` never consumes piped or
substituted stdin — it walks whatever path it's given as an argument,
which is still a real, fff-indexed filesystem search — so
`curl ... \| find . -type f` and `find <(curl ...) -type f` both stay
blocked.

The built-in `grep`/`find` core tools (the first two table rows above)
have no such exception either: they're blocked outright inside a git
work tree, since kit's core tools always operate on repo files, never
on piped or substituted process output.

## Repository layout

- **`enforce-fff.go`** — the actual kit extension, at the repo root. kit's
  [Yaegi](https://github.com/traefik/yaegi)-based loader evaluates this
  file's raw source text in an interpreter that only exposes the Go
  standard library and kit's own `kit/ext` API, so it can't import
  anything else in this module — the decision logic is inlined here. It
  must stay at the root: kit's `kit install` scanner only recognizes
  root-level `*.go` files (or `main.go` under an `ext/`,
  `*-ext/`/`*-extensions/` subdirectory) as extensions.
- **`internal/fffenforcer/policy.go`** — the same decision logic, factored
  out as an ordinary, dependency-free Go package so it can be unit tested
  with `go test`. It lives under `internal/` specifically because kit's
  extension scanner skips that directory — otherwise it would try (and
  fail) to load `policy.go` as a second extension on every `kit` startup,
  since it has no `Init(api ext.API)` function. Kept in sync with
  `enforce-fff.go` by hand; if you change one, change the other.
- **`internal/fffenforcer/policy_test.go`** — table-driven unit tests of
  `policy.go` (pure, no git or kit binary required).
- **`smoke_test.go`** — a black-box test, at the repo root next to
  `enforce-fff.go`, that shells out to the real `kit` binary (`kit
  extensions validate -e ./enforce-fff.go`) to confirm the actual
  yaegi-loadable file parses and registers a handler. Skipped
  automatically if `kit` isn't on `PATH`.
- **`.github/workflows/ci.yml`** — builds, vets, and tests on every push
  to `main` and every PR (job id `test`). `main` requires that job's
  check to pass before merging (see [CI](#ci) below).
- **`.github/workflows/release-please.yml`** and **`release-please-config.json`**
  / **`.release-please-manifest.json`** — the release automation described
  under [Versioning](#versioning).
- **`.github/workflows/dependabot-auto-merge.yml`** and **`.github/dependabot.yml`**
  — the dependency-update automation described under
  [Versioning](#versioning).

## Testing

```bash
go test ./...
```

`TestExtensionLoads` (in `smoke_test.go`) additionally validates the real
extension file if `kit` is installed locally; everything else is a pure
unit test with no external dependencies.

## CI

`.github/workflows/ci.yml` runs `go build`, `go vet`, and `go test` on
every push to `main` and every pull request, as its `test` job. `main`
has branch protection requiring that `test` check to pass before a PR
can merge (non-strict: merging doesn't require the branch to already be
up to date with `main` first). Direct pushes to `main` are still
allowed; only merges are gated.

## Versioning

Releases are tagged `vX.Y.Z` and managed by
[release-please](https://github.com/googleapis/release-please), which
opens a release PR from [Conventional
Commits](https://www.conventionalcommits.org/) on `main` and tags a new
version (with a generated `CHANGELOG.md`) once that PR is merged. This
repo is pre-1.0: the blocking rules (what counts as a network fetch,
which shell shapes are exempted, etc.) are still settling and may change
in a minor bump rather than a major one.

Pin `kit install` to a released tag (see [Install](#install)) rather
than tracking `main`, so a change to the blocking rules doesn't show up
unannounced.

release-please authenticates as a dedicated GitHub App (installed only
on this repo), rather than the default `GITHUB_TOKEN`, minted via
[`actions/create-github-app-token`](https://github.com/actions/create-github-app-token).
This is required, not cosmetic: GitHub suppresses new workflow runs
triggered by the default token (an anti-recursion rule), so the `test`
job in `ci.yml` would never run on the release PR itself, and that PR
could never satisfy the required `test` check described under
[CI](#ci). The App's
Client ID and private key are stored as the `RELEASE_PLEASE_APP_CLIENT_ID`
repo variable and `RELEASE_PLEASE_APP_PRIVATE_KEY` repo secret.

Dependency and GitHub Actions updates are automated via
[Dependabot](.github/dependabot.yml) (weekly, for both the Go module and
the workflows in `.github/workflows/`).
[`dependabot-auto-merge.yml`](.github/workflows/dependabot-auto-merge.yml)
enables GitHub's native auto-merge on Dependabot PRs that are
minor/patch bumps, once the required `test` check passes; major bumps
are left open for manual review.

## License

[GPL-3.0](LICENSE).
