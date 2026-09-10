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

## Testing

```bash
go test ./...
```

`TestExtensionLoads` (in `smoke_test.go`) additionally validates the real
extension file if `kit` is installed locally; everything else is a pure
unit test with no external dependencies.

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

Dependency and GitHub Actions updates are automated via
[Dependabot](.github/dependabot.yml) (weekly, for both the Go module and
the workflows in `.github/workflows/`).

## License

[GPL-3.0](LICENSE).
