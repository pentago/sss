# Repository Guidelines

## Project Overview
`sss` is a minimalist, self-hosted feed reader written in Go (module `sss`, single static binary). It is an opinionated fork of [Miniflux](https://github.com/miniflux/v2): it keeps Miniflux's behavior and its policy ("the purpose is to read feeds, nothing else"), but focuses on a cleaner web UI. The deliberate differences are:
- SQLite only (`modernc.org/sqlite`, pure Go, `CGO_ENABLED=0`). No Postgres.
- Classless semantic HTML UI. JavaScript is opt-in per user, and every feature works without it.
- Config comes from env vars and an optional `KEY=VALUE` file. English only (no i18n).
- Clients stay compatible: Miniflux REST API `/v1/`, Fever `/fever/`, and Google Reader `/reader/api/0/`.
- Trimmed integration set: Apprise, Discord, Gotify, Instapaper, Linkwarden, Matrix, Notion, Ntfy, Pushover, RSS-Bridge, Slack, Telegram, Webhook. Do not re-add the other upstream integrations.

README's "Key differences" section lists more intentional changes; read it before touching user-visible behavior. Other deliberate choices it no longer spells out: new users get newest-first ordering (the API default stays `asc`); the default category is `Uncategorized`, not `All`, and an empty one is always hidden; the `FORCE_REFRESH_INTERVAL` cooldown is per category (`WebSession.ForceRefreshedAt`, keyed by category ID, `0` = all feeds). Don't "fix" these back to upstream. Known gaps vs upstream, not choices: no SIGHUP TLS-certificate reload, no systemd socket activation. Otherwise, when unsure how something should behave, check upstream Miniflux (`git clone --depth 1 https://github.com/miniflux/v2 /tmp/mfx`) and match it.

## Architecture & Data Flow
- **Startup (`main.go`):**
  1. Flags are parsed, then `config.Load(-c)` sets the global `config.Opts`. A missing config file is not an error, even with an explicit `-c`; the warning is logged once the logger exists.
  2. `-config-dump`, `-info` and `-version` exit here, before the logger and database exist.
  3. `setupLogger()` runs, then `-healthcheck`.
  4. The database is opened with `database.Open`, wrapped by `storage.New(db)`, and checked with `store.Ping()`.
  5. Maintenance flags run and exit: `-migrate`, `-reset-*`, `-export-user-feeds`, `-flush-sessions`, `-create-admin`, `-reset-password`. They run before `RUN_MIGRATIONS` is applied (same as upstream), so on a fresh database `-create-admin` fails with `no such table: users` until `-migrate` has run. `database.Open` creates the database's parent directory (0700); nothing creates the config directory.
  6. Migrations run if `RUN_MIGRATIONS` is set; otherwise sss exits if the schema is behind. Then `CREATE_ADMIN` is applied.
  7. `-refresh-feeds` and `-run-cleanup-tasks` run once and exit.
  8. Otherwise `runDaemon` starts, in order:
     - the pprof listener, if `PPROF_LISTEN_ADDR` is set;
     - `go metric.Collect`, if `METRICS_COLLECTOR` is set;
     - the worker pool;
     - the scheduler, unless `DISABLE_SCHEDULER_SERVICE` or `MAINTENANCE_MODE` is set;
     - the HTTP server, unless `DISABLE_HTTP_SERVICE` is set;
     - sd_notify, and the DB-pinging watchdog if `WATCHDOG` is set.

     On shutdown the HTTP servers get 5s, then `pool.Shutdown()` runs.
- **HTTP (`internal/server/server.go`):**
  - The probes `/liveness`, `/healthz`, `/readiness` and `/readyz` sit at the server root.
  - Everything else is under `BasePath`: `/healthcheck`, `/fever/`, `/accounts/ClientLogin` and `/reader/api/0/` (Google Reader), `/v1/` (API, skipped when `DISABLE_API` is set), `/metrics` (only when `METRICS_COLLECTOR` is set), and `/` (UI).
  - The handler chain is `recovery(middleware(app))`. With `MAINTENANCE_MODE`, it becomes `recovery(middleware(maintenance(app)))`, which answers 503 on everything except `/healthcheck`.
  - Each `LISTEN_ADDR` entry can be a TCP address or a Unix socket path (`/…`).
  - TLS comes from `CERT_FILE`/`KEY_FILE` or from autocert (`CERT_DOMAIN`), which adds an ACME listener on `:http` and caches certificates in the `acme_cache` SQLite table.
  - pprof is served on `http.DefaultServeMux`, so app routes must never be registered on it.
- **Feed pipeline:**
  1. `worker.StartScheduler` runs `BatchBuilder.FetchJobs()` and pushes the jobs to the pool. `-refresh-feeds` goes through the same `worker.RefreshBatch`.
  2. Each job runs `reader/handler.RefreshFeed`: `fetcher` → `parser.ParseFeed` → `processor.ProcessFeedEntries` → `store.RefreshFeedEntries` (one transaction per entry) → `go integration.PushEntries` (only for new entries) → icon update (if forced or the feed has no icon) → `store.UpdateFeed`.
  3. The processor order is fixed:
     1. filter
     2. urlcleaner
     3. `RewriteEntryURL`
     4. scraper, for new or forced entries when `feed.Crawler || user.GlobalCrawler`
     5. content rewrite
     6. re-filter, only if the scraper replaced the content
     7. `sanitizer.SanitizeHTML`, always last
     8. reading time

     YouTube watch times are then fetched in bulk. "Fetch original content" uses `ProcessEntryWebPage`: scrape → rewrite → sanitize, with no filter.
- **Integrations:** fire-and-forget goroutines, dispatched in `internal/integration/integration.go`.
  - `SendEntry` is the Save action, called from the UI, API, Fever and Google Reader handlers (Google Reader calls it on every newly starred entry). It covers Instapaper, Notion, Linkwarden and Webhook.
  - `PushEntries` runs on feed refresh and covers Matrix, Webhook, Ntfy, Apprise, Discord, Slack, Pushover, Gotify and Telegram. Ntfy and Pushover also need a per-feed flag.
  - RSS-Bridge is only used for feed discovery (`reader/subscription`).

## Key Directories
| Path | Purpose |
|---|---|
| `internal/config` | `Options` struct, `Parse` (pure, used by tests), `Load` (file + env), `Dump` |
| `internal/database` | `Open` (pragmas, Unicode `lower()`), `Migrate`, append-only `migrations.go` |
| `internal/storage` | All SQL: `Storage`, `EntryQueryBuilder`, `EntryPaginationBuilder`, `FeedQueryBuilder`, `BatchBuilder`, `CertificateCache` |
| `internal/model` | Structs with Miniflux JSON tags; `*ModificationRequest` types use pointer fields with `Patch()` |
| `internal/api`, `internal/fever`, `internal/googlereader` | Client APIs, wire-compatible with Miniflux |
| `internal/ui` | HTML UI: routes (`ui.go`, `account_routes.go`), `handler.go`, `middleware.go`, `templates/`, `static/` |
| `internal/server` | Listeners, probes, BasePath mounting, recovery, logging and maintenance middleware, TLS |
| `internal/reader/*` | handler (`RefreshFeed`, `CreateFeed`), fetcher, parser (gofeed + `encoding/json`), processor, sanitizer, scraper, readability, rewrite, urlcleaner, filter, icon, opml, subscription, readingtime |
| `internal/integration/*` | One subpackage per integration |
| `internal/http/{client,request,response}` | Outbound client for integrations, request context helpers, `HTMLRedirect`/`JSON*` |
| `internal/worker` | Pool, scheduler, cleanup tasks |
| `internal/metric`, `internal/mediaproxy` | Prometheus collector (`sss_` prefix); `/proxy/` media route |
| `internal/validator` | Request validation behind a small `validator.Store` interface (it does not import storage) |
| `packaging/` | `sss.conf`, systemd system and user units, `Dockerfile.goreleaser` |
| `examples/` | `compose.yaml` (the `:dev` image) and `config/sss.conf` |
| `docs/` | `instrumentation.md`, and `screenshot.png`, which the README shows |

## Development Commands
```sh
gofmt -l .                                  # must print nothing
go vet ./...
go test -race ./...                         # CI runs with -race
CGO_ENABLED=0 go build -o /tmp/sss .
/tmp/sss -config-dump                       # effective config, secrets redacted
docker build -t sss:ci . && docker run --rm sss:ci -version   # same as CI's docker job
```
There is no golangci-lint, staticcheck, Makefile or CHANGELOG. The checks above are the whole of CI.

To run locally: `RUN_MIGRATIONS=1 CREATE_ADMIN=1 ADMIN_USERNAME=admin ADMIN_PASSWORD=test123456 LISTEN_ADDR=127.0.0.1:8097 DATABASE_PATH=/tmp/sss-dev.db /tmp/sss`

To check API compatibility, start sss with a fresh database and `FETCHER_ALLOW_PRIVATE_NETWORKS=1`, then run upstream's suite against it from the Miniflux clone:
```sh
TEST_MINIFLUX_BASE_URL=http://127.0.0.1:8097 TEST_MINIFLUX_ADMIN_USERNAME=admin TEST_MINIFLUX_ADMIN_PASSWORD=test123456 go test -count=1 ./internal/api/
```
Three tests are expected to fail:
- `TestGetUsersEndpointAsAdmin` and `TestCreateUserEndpointAsAdmin` assert upstream defaults (theme `light_serif`, gesture nav `tap`), while sss defaults to `dark_serif` with opt-in JS off.
- `TestGetCategoriesEndpoint` expects the default category `All`, which sss names `Uncategorized`.

**Never run privileged containers, systemd-in-Docker, or live system-unit tests on the dev workstation.** It has crashed the host session. Check the system unit only with `systemd-analyze verify` and `systemd-analyze security --offline=true`.

The user unit may be tested live, with the user's approval:
1. Copy it to `~/.config/systemd/user/`.
2. Add a drop-in that sets `ExecStart=%h/<path>/sss`. The unit's `PrivateTmp=yes` hides binaries under `/tmp` (the start fails with 203/EXEC).
3. Put `LISTEN_ADDR=127.0.0.1:8097` in `~/.config/sss/sss.conf`, because port 8080 is taken on the workstation.
4. Back up `~/.local/share/sss/sss.db` first, then remove everything afterwards.

## Code Conventions & Common Patterns
- **License headers:** a file ported or adapted from Miniflux starts with exactly:
  ```go
  // Copyright The Miniflux Authors. Modified by The sss Authors.
  // SPDX-License-Identifier: Apache-2.0
  ```
  This satisfies Apache-2.0 §4(b) (states the file was modified) and §4(c) (keeps Miniflux's copyright notice). Files written from scratch get no header. Never copy upstream's `SPDX-FileCopyrightText` header.
- **The name "Miniflux":** use it only in license headers, `NOTICE`, the README, this file, and the rules-documentation link in `partials.html`. Code identifiers, comments, test data and wire names use `sss`: session cookie `SssSessionID`, webhook headers `X-Sss-Signature` / `X-Sss-Event-Type`, OPML namespace `sss:` (`https://github.com/pentago/sss/opml`), metric prefix `sss_`.
- **Marker comments:**
  - `// sss: <reason>` marks an intentional difference from upstream behavior, including in ported tests.
  - `// ponytail: <ceiling>` marks a known shortcut and how to upgrade it.
- **Errors:** wrap with the package name as prefix, e.g. `fmt.Errorf("storage: unable to …: %w", err)`. User-facing and API error strings copy upstream's wording exactly, because clients compare them.
- **Logging:** `log/slog` only, with structured attributes (`slog.Int64("user_id", …)`, `slog.Any("error", err)`).
- **Config (`internal/config/config.go`):** read `config.Opts.Field` directly; there are no getters.
  - **Values:** an empty value means "use the default" (`KEY=` can't clear one). In the config file, `#` lines are comments and there is no quote handling. Env vars override file values.
  - **Adding an option:**
    1. Add a field to `Options`, alphabetically by env name. Related or derived fields sit next to their source field (e.g. `KeyFile` after `CertFile`).
    2. Add one line to the `Parse` struct literal using the matching helper:
       - `str(name, def, choices...)`, `boolean`, `integer(name, def, min)`, `duration(name, def, unit, min)`, `list`, `url(name, secret)`;
       - `secret` for credentials: adds `_FILE` support and redacts the value in `Dump`;
       - `fileValue(name, def, false)`: `_FILE` support without redaction;
       - `p.raw(name, def, true)`: redaction without `_FILE`.

       Checks on a single value go after the literal and use `p.fail`. Cross-field checks go in `validate()`.
    3. Add cases to `config_test.go`.
    4. Update the README table of differences from Miniflux, and `packaging/sss.conf` if it's a common setting. That file must parse both as a systemd `EnvironmentFile=` (it is installed as `/etc/sss.conf` for the system unit; sss itself never reads that path) and as an sss config file (it is also installed as `sss.conf.example`, which users copy to `~/.config/sss/sss.conf`).
- **Storage:**
  - No `context.Context` parameters (except `CertificateCache`, which implements autocert's interface).
  - Raw SQL with `$N` placeholders. ID lists are passed as one JSON argument: `id IN (SELECT value FROM json_each($N))` with `jsonArray(ids)`. JSON columns (`entries.tags`) are written with `jsonArray` and scanned with `jsonStrings`.
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
  - The middleware chain is `withSecurityHeaders(CrossOriginProtection(withSession(mux)))`. CSRF protection is Go's `http.NewCrossOriginProtection()`; do not add CSRF token fields.
  - New routes go in `ui/ui.go` or `registerAccountRoutes`. If they must work without login, also add them to `isPublicRoute` or `isStaticAssetRoute`. Static assets skip the session, except `/custom.css`, `/custom.js` and `/manifest.json`, which load the user. Admin pages are wrapped in `adminOnly(...)`.
- **Templates (`internal/ui/templates/*.html`, embedded):**
  - Each page is built from `layout.html` + `*partials.html` + the page file, which defines `{{define "content"}}`.
  - Strings are hard-coded English. Use `{{route …}}` for internal URLs and `{{untrustedURL …}}` for feed links.
  - Every func a template uses must be in `templateFuncs` (`handler.go`). A missing one panics at startup, and no test catches it, so run the binary.
  - **The markup contract:** no `class`, `id` or `style` attributes, no inline scripts, no `on*` handlers. Select on elements, ARIA and `data-*` instead:
    - `body[data-page]`: entry-list type, settings section, or template name. It comes from `data["page"]`, which handlers set and `render` defaults to the template name (`handler.go`, `account_routes.go`). Renaming a value breaks user CSS. `data-entry-swipe` and `data-gesture-nav` are also on `<body>`.
    - `main > article[data-id][data-status][data-starred]`; `data-unread` and `data-errors` on feed and category lists.
    - `nav[aria-label="Main"|"Entry actions"|"Page actions"|"Bottom page actions"|"Pagination"|"Settings"|…]`, `aria-current="page"`, `role="status|alert"`.
    - The star button uses `aria-pressed` with the labels `Star`/`Unstar`; the WebAuthn buttons use `button[data-webauthn="login|register"]`.
    - Confirmations as `details > summary + form`.
  - User Custom CSS/JS, `static/js/*` and `base.css` (e.g. `form[action$="/entry/status"]`) all depend on these selectors, on button labels (`"Mark as read"`/`"Mark as unread"`) and on form actions. Treat them as a public API. If you change one, update every consumer. The README no longer lists the selectors; this section is the reference.
- **JS (`internal/ui/static/js/`):**
  - Plain ES2020 with no dependencies, loaded as `<script src defer>`.
  - Each script is included only when its user setting is on (`pageAssets` in `handler.go`). `swipe.js` loads when entry swipe is on or gesture nav isn't `none`. `webauthn.js` loads only when the handler sets `data["webauthn"]=true` and `WEBAUTHN` is enabled.
  - Scripts reuse the existing forms: `fetch(form.action, {method: "POST", redirect: "manual"})`, treating `opaqueredirect` as success. Entry list items have no forms (sss keeps lists to feed, title, reading time and date), so read/star toggles there post to `<base>/entry/status` and `<base>/entry/star/{id}` built from `data-id`/`data-status`, with `<base>` derived from `document.currentScript.src`.
  - The CSP (`ui/middleware.go`, set per user) is `default-src 'self'`, `script-src 'self'`, `style-src 'self'` plus the user's external font hosts, and `img-src`/`media-src`/`frame-src *`. Trusted Types are enforced: the policy names `html` and `url` are allowed, but only `url` is created so far (`sw-register.js`). Any DOM sink a new script uses needs one of these policies.
- **Themes:**
  - `model.ThemeDefault = "dark_serif"`. Values are `{light,dark,oled,system,time,time_oled}_{serif,sans_serif}` or `none`.
  - The `time_*` and `time_oled_*` themes are resolved on the server by `User.ThemeAt`; `system_*` uses CSS `media` queries.
  - `light.css`, `dark.css` and `oled.css` contain only color variables. A new variable must be added to all three.

## Important Files
- `main.go`: flags, startup order, `runDaemon`, `setupLogger`, `healthCheck`.
- `internal/config/config.go`: config precedence and XDG defaults.
- `internal/database/database.go` and `migrations.go`: database settings and schema.
- `internal/storage/storage.go`: scan helpers. `entry.go`: refresh, tombstones, archiving. `entry_query_builder.go`: FTS5 search with `bm25`.
- `internal/ui/handler.go`: templates, `templateFuncs`, asset hashing, `pageAssets`, `render`, `redirectBack`. `middleware.go`: CSP, session cookie `SssSessionID`, public routes.
- `internal/api/api.go`, `middleware.go`, `messages.go`: v1 routes, auth (`X-Auth-Token` first, then Basic), JSON shapes.
- `internal/version/version.go`: the default `Version` is `2.3.x-dev`. Keep that format, because some clients parse it as a number.
- `README.md`: user-facing contracts (differences from Miniflux, config table, Docker and systemd layout). Keep it in sync.
- `NOTICE` / `LICENSE`: Apache-2.0 with Miniflux attribution, plus the Noto fonts under OFL 1.1. Bundled third-party assets need a `NOTICE` entry.

## Runtime/Tooling Preferences
- `go.mod` pins the latest Go patch release (`go 1.27.3`) with the patch digit: CI and releases install exactly that version via `setup-go`'s `go-version-file`, so the pin is what keeps release binaries on a patched stdlib. A bare `go 1.27` makes builds fail. On a minor Go bump, also update `golang:1.27-alpine` in the root `Dockerfile`.
- Builds always use `CGO_ENABLED=0 -trimpath`. Version info is set with `-X sss/internal/version.{Version,Commit,BuildDate}`. The root `Dockerfile` and releases also pass `-s -w`. CI never sets `VERSION`, so `:dev` images fall back to `2.3.x-dev`.
- Don't add dependencies unless a few lines of code can't do the job.
  - Current direct deps: sqlite, gofeed, goquery/cascadia, go-oidc + x/oauth2, go-webauthn, x/crypto, x/net, x/term, prometheus/client_golang (`METRICS_*`).
  - Profiling and tracing use only the stdlib (`net/http/pprof`, `runtime/trace`); don't add OpenTelemetry or profiler SDKs.
- **Releases** (`.goreleaser.yaml`, `release.yml` on `v*` tags):
  - Binaries for linux, darwin and freebsd; deb/rpm/apk/archlinux packages; a Homebrew cask; and `ghcr.io` images tagged `{{.Version}}` and `latest`.
  - Packages install `packaging/sss.conf` as `/etc/sss.conf` (0600, `config|noreplace`) and as `/usr/share/doc/sss/sss.conf.example`, which the README and the user unit reference. `archives.files` and `nfpms.contents` list the packaged files separately, so a new file must be added to both.
  - The image platform list (`linux/amd64,linux/arm64,linux/arm/v7,linux/riscv64`) appears in `ci.yml`, `.goreleaser.yaml` (`dockers_v2`) and the README. Keep all three in sync.
- **Docker:** there are two Dockerfiles. The root `Dockerfile` builds from source and is the one CI builds. `packaging/Dockerfile.goreleaser` copies the released binary. Keep them in sync:
  - `FROM scratch`, binary at `/sss`, volume `/config` holding `sss.conf` and `sss.db`, running as user 65534.
  - Both use the same `ENTRYPOINT ["/sss","-c","/config/sss.conf"]`, `HEALTHCHECK … -healthcheck auto`, `EXPOSE 8080` and CA bundle.
  - Defaults (`LISTEN_ADDR`, `DATABASE_PATH`, `RUN_MIGRATIONS`) are written into `/config/sss.conf`, not `ENV`, because env vars would override a user's mounted config.
  - `-healthcheck auto` uses the first `LISTEN_ADDR` entry, so it must be a TCP `host:port`.
  - Drift in `Dockerfile.goreleaser` only shows up at release.
- **systemd:**
  - The system unit sets only `XDG_DATA_HOME=%S` (no `DATABASE_PATH`) and is fully sandboxed: `DynamicUser`, `Type=notify` with `WatchdogSec`, `SystemCallFilter=@system-service ~@privileged`, `ProtectHome`, and so on.
  - The user unit has the same seccomp filter and `NoNewPrivileges`, but none of the `Protect*`/`DynamicUser` settings.
  - Don't add `PrivateUsers` or network restrictions; the units have comments explaining why.
- **CI (`.github/workflows/ci.yml`):**
  - Jobs: `test` (gofmt, vet, `go test -race`, build), `docker` (build + `-version`), and `publish`. `publish` runs after both, on main pushes only, and pushes a multi-arch `ghcr.io/…/sss` image tagged `dev` and `pr-<N>`, with a provenance attestation.
  - GitHub Actions are pinned as `owner/action@<commit-sha> # vX.Y.Z`.
  - `.github/dependabot.yml` updates Go modules weekly, Docker base images and actions monthly, grouped into one PR per ecosystem. Base images need explicit tags (`alpine:3.24`), or Dependabot can't bump them.
- **Git:** work in git worktrees (`wt switch -c <branch>`, under `.worktrees/`), and don't commit unless asked.

## Profiling & Tracing
Stdlib only; full guide in `docs/instrumentation.md`.
- Start with `PPROF_LISTEN_ADDR=127.0.0.1:6060` (unauthenticated, localhost only). Enabling it also turns on mutex and block profiling.
- CPU: `go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/profile?seconds=30`. Swap `profile` for `heap`, `allocs`, `goroutine`, `mutex` or `block`; add `-top` for text output.
- Latency breakdown:
  1. `curl -o t.out 'http://127.0.0.1:6060/debug/pprof/trace?seconds=10'`, then reproduce the problem.
  2. `go tool trace t.out`. Look under User-defined tasks: `http` is one task per request, and `feed.refresh` one per refresh, with the regions `fetch`/`read_body`/`parse`/`process`/`store`/`icon`.
  3. For text output, use `go tool trace -d=parsed t.out` or `-pprof=net|sync|syscall|sched`.
- New hot paths get a task or region: `ctx, task := trace.NewTask(ctx, "name"); defer task.End()` or `defer trace.StartRegion(ctx, "step").End()`. They cost almost nothing while no trace is running. Wrap `trace.Log` arguments that are expensive to build in `if trace.IsEnabled()`.

## Testing & QA
- **Style:**
  - Only the standard `testing` package (no testify or mocks). Tests live in the same package as the code (not `_test`). There are no build tags and no in-repo e2e suite.
  - Mostly table tests with `t.Run`. Use `t.Helper()`, `t.Cleanup` and `t.TempDir()`.
  - Fuzz targets: `parser` (`FuzzParse`) and `sanitizer` (`FuzzSanitizer`). New benchmarks use `b.Loop()`; `readability` still uses `b.N`.
- **Database tests:** use a real migrated SQLite file, never a mocked store:
  ```go
  db, _ := database.Open(filepath.Join(t.TempDir(), "db")); database.Migrate(db); store := storage.New(db)
  ```
  Per-package helpers:
  - `storage_test.go`: `newTestStorage`, `mustCreateUser` (password `secret123`), `mustCreateFeed`, `newEntry`.
  - `api_test.go`: `newTestEnv` (admin `alice`/`secret123` plus an API key) runs the real handler on `httptest.NewServer`. Its methods `(*testEnv).do` (sends `X-Auth-Token`) and `(*testEnv).seedFeed` (category "News", a feed, 2 unread entries) build on it.
  - `ui/account_test.go`: `newAccountTestServer(t, env...)` (non-admin `alice`), plus `login`, `postForm` and `sessionCookie`.
- **Config in tests:** `prev := config.Opts; config.Opts, _ = config.Parse([]string{"KEY=VALUE"}); t.Cleanup(func() { config.Opts = prev })`, as in `integration_test.go`, or the helper `setOptions(t, env...)` (`http/client/client_test.go`).
  - Use `t.Setenv` only when the code reads the environment itself (`config.Load`, `systemd`).
  - Don't add `t.Parallel()` to tests that change `config.Opts`.
  - Legacy debt, don't copy it: the ported tests in `mediaproxy`, `reader/rewrite` and `reader/sanitizer` still call `os.Setenv`/`os.Clearenv`. Many tests (`api`, `ui`, `fever`, `googlereader`, `processor`, `scraper`, …) set `config.Opts` without restoring it, and `model`/`validator` reset it to `nil`.
- **HTTP:**
  - Outbound fetches and integration targets use `httptest.NewServer`.
  - Handlers (`ui`, `fever`, `googlereader`, `metric`) are tested with `httptest.NewRecorder` + `h.ServeHTTP`.
  - There is no real network access. The HTTP clients block loopback addresses by default, so set `FETCHER_ALLOW_PRIVATE_NETWORKS=1` for fetcher, icon, scraper and processor tests, and `INTEGRATION_ALLOW_PRIVATE_NETWORKS=1` for `integration/*` and `http/client`. `subscription` needs both.
- **Test data:**
  - Scraper golden files are `testdata/<name>.html` + `<name>.html-result`.
  - Parser `large_atom.xml`/`large_rss.xml` are used only by `BenchmarkParse`.
  - Ported upstream tests keep their wording, and differences are marked with `// sss:`.
- **What to test:** behavior that matters to users or clients (API shapes, state changes, edge cases). Don't write tests that only check wiring or restate the implementation.
- **Shared state:** CI uses `-race`, so tests that start goroutines must guard shared state with a mutex (see `integration_test.go`, `reader/handler/handler_test.go`).
