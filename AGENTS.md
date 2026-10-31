# Repository Guidelines

## Project Overview
`sss` is a minimalist, self-hosted feed reader written in Go (module `sss`, single static binary). It reproduces [Miniflux](https://github.com/miniflux/v2) behavior and its policy ("the purpose is to read feeds, nothing else"), with these deliberate differences:
- SQLite only (`modernc.org/sqlite`, pure Go, `CGO_ENABLED=0`). No Postgres.
- Classless semantic HTML UI. JavaScript is opt-in per user, and every feature works without it.
- Config comes from env vars and an optional `KEY=VALUE` file. English only (no i18n).
- Clients stay compatible: Miniflux REST API `/v1/`, Fever `/fever/`, and Google Reader `/reader/api/0/`.
- Trimmed integration set: Apprise, Discord, Gotify, Instapaper, Linkwarden, Matrix, Notion, Ntfy, Pushover, RSS-Bridge, Slack, Telegram, Webhook. Do not re-add the other upstream integrations.

When unsure how something should behave, check upstream Miniflux first (`git clone --depth 1 https://github.com/miniflux/v2 /tmp/mfx`) and match it, unless this file or the README says sss differs.

## Architecture & Data Flow
- **Startup (`main.go`):**
  1. Flags are parsed, then `config.Load(-c)` sets the global `config.Opts`.
  2. `-config-dump`, `-info` and `-version` exit here, before the logger and database exist.
  3. `setupLogger()` runs, then `-healthcheck`.
  4. The database is opened with `database.Open`, wrapped by `storage.New(db)`, and checked with `store.Ping()`.
  5. Maintenance flags run and exit: `-migrate`, `-reset-*`, `-export-user-feeds`, `-flush-sessions`, `-create-admin`, `-reset-password`.
  6. Migrations run if `RUN_MIGRATIONS` is set; otherwise sss exits if the schema is behind. Then `CREATE_ADMIN` is applied.
  7. `-refresh-feeds` and `-run-cleanup-tasks` run once and exit. Otherwise `runDaemon` starts the worker pool, scheduler, HTTP server, sd_notify and the DB-pinging watchdog.
- **HTTP (`internal/server/server.go`):**
  - The probes `/liveness`, `/healthz`, `/readiness` and `/readyz` sit at the server root.
  - Everything else is under `BasePath`: `/healthcheck`, `/fever/`, `/accounts/ClientLogin` and `/reader/api/0/` (Google Reader), `/v1/` (API), and `/` (UI).
  - Each `LISTEN_ADDR` entry can be a TCP address or a Unix socket path (`/…`). TLS comes from `CERT_FILE`/`KEY_FILE` or autocert (`CERT_DOMAIN`); the autocert cache is the `acme_cache` SQLite table.
- **Feed pipeline:**
  1. `worker.StartScheduler` runs `BatchBuilder.FetchJobs()` and pushes the jobs to the pool.
  2. Each job runs `reader/handler.RefreshFeed`: `fetcher` → `parser.ParseFeed` → `processor.ProcessFeedEntries` → `store.RefreshFeedEntries` (one transaction per entry) → `go integration.PushEntries` → icon update → `store.UpdateFeed`.
  3. The processor order is fixed: filter → urlcleaner → `RewriteEntryURL` → scraper (crawler feeds only) → content rewrite → re-filter → `sanitizer.SanitizeHTML` (always last) → reading time.
- **Integrations:** fire-and-forget goroutines. Saving an entry calls `integration.SendEntry` from the UI, API, Fever and Google Reader handlers. Feed refresh calls `integration.PushEntries`. RSS-Bridge is only used for feed discovery (`reader/subscription`).

## Key Directories
| Path | Purpose |
|---|---|
| `internal/config` | `Options` struct, `Parse` (pure, used by tests), `Load` (file + env), `Dump` |
| `internal/database` | `Open` (pragmas, Unicode `lower()`), `Migrate`, append-only `migrations.go` |
| `internal/storage` | All SQL: `Storage`, `EntryQueryBuilder`, `EntryPaginationBuilder`, `FeedQueryBuilder`, `BatchBuilder` |
| `internal/model` | Structs with Miniflux JSON tags; `*ModificationRequest` types use pointer fields with `Patch()` |
| `internal/api`, `internal/fever`, `internal/googlereader` | Client APIs, wire-compatible with Miniflux |
| `internal/ui` | HTML UI: routes (`ui.go`, `account_routes.go`), `handler.go`, `middleware.go`, `templates/`, `static/` |
| `internal/reader/*` | fetcher, parser (gofeed + `encoding/json`), processor, sanitizer, scraper, rewrite, filter, icon, opml, subscription |
| `internal/integration/*` | One subpackage per integration; dispatch in `integration.go` |
| `internal/worker` | Pool, scheduler, cleanup tasks |
| `internal/validator` | Request validation behind a small `validator.Store` interface (it does not import storage) |
| `packaging/` | `sss.conf`, systemd system and user units, `Dockerfile.goreleaser` |

## Development Commands
```sh
gofmt -l .                                  # must print nothing
go vet ./...
go test -race ./...                         # CI runs with -race
CGO_ENABLED=0 go build -o /tmp/sss .
/tmp/sss -config-dump                       # effective config, secrets redacted
docker build -t sss:ci . && docker run --rm sss:ci -version   # same as CI's docker job
```
To run locally: `RUN_MIGRATIONS=1 CREATE_ADMIN=1 ADMIN_USERNAME=admin ADMIN_PASSWORD=test123456 LISTEN_ADDR=127.0.0.1:8097 DATABASE_PATH=/tmp/sss-dev.db /tmp/sss`

To check API compatibility, start sss with a fresh database and `FETCHER_ALLOW_PRIVATE_NETWORKS=1`, then run upstream's suite against it from the Miniflux clone: `TEST_MINIFLUX_BASE_URL=http://127.0.0.1:8097 TEST_MINIFLUX_ADMIN_USERNAME=admin TEST_MINIFLUX_ADMIN_PASSWORD=test123456 go test -count=1 ./internal/api/`. `TestGetUsersEndpointAsAdmin` and `TestCreateUserEndpointAsAdmin` are expected to fail: they assert upstream defaults (theme `light_serif`, gesture nav `tap`), while sss defaults to `dark_serif` with opt-in JS off.

**Never run privileged containers, systemd-in-Docker, or live systemd unit tests on the dev workstation.** It has crashed the host session. Check units only with `systemd-analyze verify` and `systemd-analyze security --offline=true`.

## Code Conventions & Common Patterns
- **License headers:** a file ported or adapted from Miniflux starts with exactly:
  ```go
  // Copyright The Miniflux Authors. Modified by The sss Authors.
  // SPDX-License-Identifier: Apache-2.0
  ```
  This satisfies Apache-2.0 §4(b) (states the file was modified) and §4(c) (keeps Miniflux's copyright notice). Files written from scratch get no header. Never copy upstream's `SPDX-FileCopyrightText` header.
- **The name "Miniflux":** use it only where needed: license headers, `NOTICE`, the README's compatibility and differences sections, and the rules-documentation link in `partials.html`. Code identifiers, comments, test data and wire names use `sss`: session cookie `SssSessionID`, webhook headers `X-Sss-Signature` / `X-Sss-Event-Type`, OPML namespace `sss:` (`https://github.com/pentago/sss/opml`).
- **Marker comments:**
  - `// sss: <reason>` marks an intentional difference from upstream behavior, including in ported tests.
  - `// ponytail: <ceiling>` marks a known shortcut and how to upgrade it.
- **Errors:** wrap with the package name as prefix, e.g. `fmt.Errorf("storage: unable to …: %w", err)`. User-facing and API error strings copy upstream's wording exactly, because clients compare them.
- **Logging:** `log/slog` only, with structured attributes (`slog.Int64("user_id", …)`, `slog.Any("error", err)`).
- **Config:** read `config.Opts.Field` directly; there are no getters. To add an option:
  1. Add a field to `Options`, in alphabetical order by env name.
  2. Add one line to the `Parse` struct literal using the matching helper (`str` with optional choices, `boolean`, `integer`, `duration`, `list`, `url`, or `secret` for credentials, which also gives `_FILE` support and redacts the value in `Dump`; `fileValue(name, def, false)` gives `_FILE` support without redaction). Put cross-field checks in `validate()`.
  3. Add cases to `config_test.go`.
  4. Update the README table of differences from Miniflux, and `packaging/sss.conf` if it's a common setting.
- **Storage:**
  - No `context.Context` parameters.
  - Raw SQL with `$N` placeholders. ID lists are passed as one JSON argument: `id IN (SELECT value FROM json_each($N))` with `jsonArray(ids)`.
  - Timestamps are INTEGER unix seconds, bound as `time.Time` (the DSN sets `_time_integer_format=unix`) and scanned with `unixTime`/`nullUnixTime`. Use `unixepoch()` in SQL.
  - Booleans are stored as 0/1.
  - Transactions: `Begin` → `defer tx.Rollback()` or `return rollback(tx, err)` → `Commit`. `_txlock=immediate` means every `Begin` takes the write lock.
  - Case-insensitive lookups rely on the Unicode `lower()` override. Don't replace it with `COLLATE NOCASE`.
- **Migrations:** `database/migrations.go` is append-only. Never edit an applied entry; the version is stored in `PRAGMA user_version`.
- **Entries:**
  - The only statuses are `read` and `unread`; there is no `removed`.
  - Deleted, archived or flushed entries are physically removed and their hash recorded in `entry_tombstones`, so they aren't re-created.
  - Starred and shared entries are never archived.
- **UI handlers:**
  - Render with `h.render(w, r, "page", map[string]any{"title": …, "menu": …})`.
  - Every mutation is a `POST` form that ends with a 303 redirect (`response.HTMLRedirect` or `h.redirectBack`).
  - Flash messages go through `request.WebSession(r).SetSuccessMessage/SetErrorMessage`.
  - On validation errors, re-render the form instead of redirecting.
  - CSRF protection is Go's `http.NewCrossOriginProtection()`. Do not add CSRF token fields.
  - New routes go in `ui/ui.go` or `registerAccountRoutes`. If they must work without login, also add them to `isPublicRoute` or `isStaticAssetRoute`. Admin pages are wrapped in `adminOnly(...)`.
- **Templates (`internal/ui/templates/*.html`, embedded):**
  - Each page is built from `layout.html` + `*partials.html` + the page file, which defines `{{define "content"}}`.
  - Strings are hard-coded English. Use `{{route …}}` for internal URLs and `{{untrustedURL …}}` for feed links.
  - **The markup contract:** no `class`, `id` or `style` attributes, no inline scripts, no `on*` handlers. Select on elements, ARIA and `data-*` instead:
    - `main > article[data-id][data-status][data-starred]`; `data-unread` and `data-errors` on feed and category lists.
    - `nav[aria-label="Main"|"Entry actions"|"Page actions"|"Pagination"|…]`, `aria-current="page"`, `role="status|alert"`.
    - Confirmations as `details > summary + form`.
  - User Custom CSS/JS and the built-in scripts depend on these selectors, on button labels (`"Mark as read"`/`"Mark as unread"`) and on form actions. Changing any of them means updating the README Themes table and `static/js/*`.
- **JS (`internal/ui/static/js/`):**
  - Plain ES2020 with no dependencies, loaded as `<script src defer>`.
  - Each script is included only when its user setting is on (`pageAssets` in `handler.go`).
  - Scripts reuse the existing forms: `fetch(form.action, {method: "POST", redirect: "manual"})`, treating `opaqueredirect` as success.
  - The CSP is `script-src 'self'`, `style-src 'self'`, and Trusted Types (policies `html`/`url`) are enforced.
- **Themes:**
  - `model.ThemeDefault = "dark_serif"`. Values are `{light,dark,system,time}_{serif,sans_serif}` or `none`.
  - `time_*` themes are resolved on the server by `User.ThemeAt`; `system_*` uses CSS `media` queries.
  - `light.css` and `dark.css` contain only color variables. A new variable must be added to both.

## Important Files
- `main.go`: flags, startup order, `runDaemon`, `setupLogger`, `healthCheck`.
- `internal/config/config.go`: config precedence and XDG defaults.
- `internal/database/database.go` and `migrations.go`: database settings and schema.
- `internal/storage/storage.go`: scan helpers. `entry.go`: refresh, tombstones, archiving. `entry_query_builder.go`: FTS5 search with `bm25`.
- `internal/ui/handler.go`: templates, asset hashing, `pageAssets`, `render`, `redirectBack`. `middleware.go`: CSP, session cookie `SssSessionID`, public routes.
- `internal/api/api.go`, `middleware.go`, `messages.go`: v1 routes, auth (`X-Auth-Token` first, then Basic), JSON shapes.
- `README.md`: user-facing contracts (Miniflux-difference table, Themes selector table, Docker and systemd layout). Keep it in sync.
- `NOTICE` / `LICENSE`: Apache-2.0 with Miniflux attribution.

## Runtime/Tooling Preferences
- `go.mod` pins the latest Go patch release (`go 1.27.3`) with the patch digit: CI and releases install exactly that version via `setup-go`'s `go-version-file`, so the pin is what keeps release binaries on a patched stdlib. A bare `go 1.27` makes builds fail.
- Builds always use `CGO_ENABLED=0 -trimpath`. Version info is set with `-X sss/internal/version.{Version,Commit,BuildDate}`.
- Don't add dependencies unless a few lines of code can't do the job. Current direct deps: sqlite, gofeed, goquery/cascadia, go-oidc + x/oauth2, go-webauthn, x/crypto, x/net, x/term, prometheus/client_golang (METRICS_*). Profiling and tracing use only the stdlib (`net/http/pprof`, `runtime/trace`); don't add OpenTelemetry or profiler SDKs.
- **Releases** (`.goreleaser.yaml`): binaries, deb/rpm/apk/archlinux packages, a Homebrew cask and `ghcr.io` images.
- **Docker:** there are two Dockerfiles, the root `Dockerfile` and `packaging/Dockerfile.goreleaser`. Keep them in sync:
  - `FROM scratch`, binary at `/sss`, volume `/config` holding `sss.conf` and `sss.db`, running as user 65534.
  - Defaults are written into `/config/sss.conf`, not `ENV`, because env vars would override a user's mounted config.
- **systemd:** the system unit sets only `XDG_DATA_HOME=%S` (no `DATABASE_PATH`) and is fully sandboxed (`SystemCallFilter=@system-service ~@privileged`, `ProtectHome`, and so on). The user unit has only light hardening that works without privileges. Don't add `PrivateUsers` or network restrictions.
- **CI:**
  - GitHub Actions are pinned as `owner/action@<commit-sha> # vX.Y.Z`.
  - `.github/dependabot.yml` updates Go modules weekly, Docker base images and actions monthly. Base images need explicit tags (`alpine:3.24`), or Dependabot can't bump them.
- **Git:** work in git worktrees (`wt switch -c <branch>`, under `.worktrees/`), and don't commit unless asked.

## Profiling & Tracing
Stdlib only; full guide in `docs/instrumentation.md`.
- Start with `PPROF_LISTEN_ADDR=127.0.0.1:6060` (unauthenticated, localhost only).
- CPU: `go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/profile?seconds=30`. Swap `profile` for `heap`, `allocs`, `goroutine`, `mutex` or `block`; add `-top` for text output.
- Latency breakdown: `curl -o t.out 'http://127.0.0.1:6060/debug/pprof/trace?seconds=10'`, reproduce, then `go tool trace t.out` (User-defined tasks: `http` per request, `feed.refresh` per refresh with regions `fetch`/`read_body`/`parse`/`process`/`store`/`icon`). For text output use `go tool trace -d=parsed t.out`, or `-pprof=net|sync|syscall|sched`.
- New hot paths get a task or region: `ctx, task := trace.NewTask(ctx, "name"); defer task.End()` or `defer trace.StartRegion(ctx, "step").End()`. They cost almost nothing while no trace is running. Wrap `trace.Log` arguments that are expensive to build in `if trace.IsEnabled()`.

## Testing & QA
- **Style:**
  - Only the standard `testing` package (no testify or mocks). Tests live in the same package as the code (not `_test`).
  - Mostly table tests with `t.Run`. Use `t.Helper()`, `t.Cleanup` and `t.TempDir()`.
- **Database tests:** use a real migrated SQLite file, never a mocked store:
  ```go
  db, _ := database.Open(filepath.Join(t.TempDir(), "db")); database.Migrate(db); store := storage.New(db)
  ```
  Per-package helpers: `storage_test.go:newTestStorage`, `api_test.go:newTestEnv` (user `alice`/`secret123` plus an API key), `ui/account_test.go:newAccountTestServer`.
- **Config in tests:** `config.Opts, _ = config.Parse([]string{"KEY=VALUE"})`, then restore the previous value in `t.Cleanup`. Don't use `os.Setenv`, and don't add `t.Parallel()` to tests that change `config.Opts`.
- **Network:** no real network access. Use `httptest.NewServer`, and set `FETCHER_ALLOW_PRIVATE_NETWORKS=1` or `INTEGRATION_ALLOW_PRIVATE_NETWORKS=1`, because the HTTP client blocks loopback addresses by default.
- **Test data:** parser (`large_atom.xml`, `large_rss.xml`) and scraper keep fixtures in `testdata/`. Ported upstream tests keep their wording, and differences are marked with `// sss:`.
- **What to test:** behavior that matters to users or clients (API shapes, state changes, edge cases). Don't write tests that only check wiring or restate the implementation.
- **Shared state:** CI uses `-race`, so tests that start goroutines must guard shared state (see `integration_test.go`).
