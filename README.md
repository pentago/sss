# sss

A minimalist, self-hosted feed reader written in Go:

- **SQLite** storage. It uses the pure Go driver (no CGO), so the binary stays static.
- **Classless semantic HTML.** Pages use only plain elements, with no `class`, `id` or `style` attributes, so a theme needs very little CSS.
- **No JavaScript by default.** Every action is a plain HTML form. Keyboard shortcuts, swipe gestures, mark-as-read-on-scroll, the offline service worker and passkeys are opt-in.
- **XDG paths when run as a user:** the config file lives at `~/.config/sss/sss.conf` and the database at `~/.local/share/sss/sss.db`.
- **Integrations:** Apprise, Discord, Gotify, Instapaper, Linkwarden, Matrix, Notion, Ntfy, Pushover, RSS-Bridge, Slack, Telegram and Webhook.
- **API compatibility:** the Miniflux REST API (`/v1/`), the Fever API (`/fever/`) and the Google Reader API (`/reader/api/0/`), so existing Miniflux clients keep working. Webhooks send `X-Sss-Signature` and `X-Sss-Event-Type` headers. OPML per-feed settings use the `sss:` namespace (`https://github.com/pentago/sss/opml`), so a Miniflux OPML export imports its feeds and categories but not their per-feed settings.
- **English only.**

## Install

Each `v*` tag builds release artifacts in GitHub Actions (`.github/workflows/release.yml` runs GoReleaser):

| Platform | Install |
|---|---|
| Debian / Ubuntu | `sudo apt install ./sss_<version>_linux_amd64.deb` |
| Fedora / RHEL / openSUSE | `sudo dnf install ./sss-<version>-1.x86_64.rpm` |
| Arch Linux | `sudo pacman -U ./sss-<version>-1-x86_64.pkg.tar.zst` |
| Alpine | `sudo apk add --allow-untrusted ./sss_<version>_x86_64.apk` |
| macOS | `brew install --cask <owner>/tap/sss` |
| Anything else | Unpack the `tar.gz` for your OS/architecture (Linux, macOS, FreeBSD) |

The Linux packages install `/usr/bin/sss`, a system unit, a user unit and `/etc/sss.conf`.

### Run as your user (XDG)

```sh
mkdir -p ~/.config/sss
cp /usr/share/doc/sss/sss.conf.example ~/.config/sss/sss.conf   # RUN_MIGRATIONS=1 is already set
sss -create-admin                                               # interactive
systemctl --user enable --now sss                               # Linux; or just run `sss`
```

- **Config file:** `$XDG_CONFIG_HOME/sss/sss.conf` (default `~/.config/sss/sss.conf`).
- **Database:** `$XDG_DATA_HOME/sss/sss.db` (default `~/.local/share/sss/sss.db`); the directory is created automatically.
- **Linux:** to keep the user service running after you log out, run `loginctl enable-linger $USER`.
- **macOS:** there is no launchd service; run `sss` directly. It uses the same paths.

### Run as a system service (Linux)

Edit `/etc/sss.conf`. It is root-only and systemd reads it as an `EnvironmentFile`. For the first start, set `CREATE_ADMIN=1`, `ADMIN_USERNAME` and `ADMIN_PASSWORD` there; remove them once the service has started. Then run:

```sh
sudo systemctl enable --now sss
```

The unit (`packaging/systemd/sss.service`) uses `Type=notify` with a watchdog and `DynamicUser`. It is sandboxed following the [Arch wiki](https://wiki.archlinux.org/title/Systemd/Sandboxing): read-only system, no home directories, a system-call allowlist, and only IPv4/IPv6/Unix sockets. Its only capability is binding ports below 1024. Because of `ProtectHome`, `CERT_FILE` and `KEY_FILE` must not be under `/home`. It sets only `XDG_DATA_HOME=/var/lib`, so the database lives in `/var/lib/sss/sss.db`.

### From source

```sh
go build -o sss .
RUN_MIGRATIONS=1 CREATE_ADMIN=1 ADMIN_USERNAME=admin ADMIN_PASSWORD=change-me-please ./sss
```

The server listens on `127.0.0.1:8080` by default. Run `sss -h` to list the maintenance flags (`-migrate`, `-reset-password`, `-healthcheck auto`, `-config-dump`, …). For cron-driven polling, set `DISABLE_SCHEDULER_SERVICE=1` and run `sss -refresh-feeds` periodically.

### Docker

```sh
docker build -t sss .
docker run -d -p 8080:8080 -v sss-config:/config \
  -e CREATE_ADMIN=1 -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD=change-me-please sss
```

The image is built from `scratch` and runs as an unprivileged user. Its layout:

- `/sss` is the binary.
- `/config` is the volume. It holds the config file `/config/sss.conf` and the database `/config/sss.db`.

The image ships a default `sss.conf` with `LISTEN_ADDR=0.0.0.0:8080`, `DATABASE_URL=/config/sss.db` and `RUN_MIGRATIONS=1`. A named volume picks that file up automatically. If you bind-mount your own directory, it must contain an `sss.conf` with those lines. Environment variables override the file.

`docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/riscv64 .` builds the image for several architectures.

### Releasing

Push a `v*` tag. The release workflow publishes binaries, the four Linux package types, and multi-arch images to `ghcr.io/<owner>/sss` (tagged with the version and `latest`). The Homebrew cask goes to `<owner>/homebrew-tap`, but only when the repository secret `HOMEBREW_TAP_TOKEN` is set. That secret needs to be a token with write access to the tap repository.

`.github/workflows/ci.yml` runs formatting, vet, race tests and a Docker build on every PR and push to `main`. After those pass on `main` (a merged PR), it publishes multi-arch images tagged `main` and `sha-<commit>`.

Release artifacts, release images and `main` images get signed build provenance attestations. Verify one with `gh attestation verify <file> --owner <owner>` or `gh attestation verify oci://ghcr.io/<owner>/sss:<tag> --owner <owner>`.

## Configuration

Settings use [Miniflux's configuration names](https://miniflux.app/docs/configuration.html), including the `_FILE` variants for secrets. They can come from two places:

- **A config file:** `-c`/`-config-file <path>`, or `$XDG_CONFIG_HOME/sss/sss.conf` when it exists. It holds one `KEY=VALUE` per line, and lines starting with `#` are comments.
- **Environment variables**, which override the file.

These differ from Miniflux:

| Variable | Status |
|---|---|
| `DATABASE_URL` | Path to the SQLite file or a `file:` URI. Default `$XDG_DATA_HOME/sss/sss.db`, where `XDG_DATA_HOME` defaults to `~/.local/share`. When there is no home directory, the default is `sss.db` in the working directory. |
| `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_CONNECTION_LIFETIME` | Removed (SQLite) |
| `METRICS_*` | Removed (no Prometheus) |
| `HTTP_CLIENT_PROXIES`, `FETCHER_ALLOW_PRIVATE_FEED_PROXY` | Removed |
| `FETCH_NEBULA_WATCH_TIME`, `FETCH_ODYSEE_WATCH_TIME`, `FETCH_BILIBILI_WATCH_TIME` | Removed (only YouTube watch time is supported) |

Run `./sss -config-dump` to see the values in effect.

## Themes

User settings → Theme. Values use Miniflux's names, so API clients round-trip unchanged: a color scheme followed by `_serif` or `_sans_serif` (the font of the page and entry content), or `none`. The default is `dark_serif`.

- `light_*` / `dark_*`: the built-in reference themes.
- `system_*`: follows the device's light or dark preference (`prefers-color-scheme`).
- `time_*`: light between two hours of the day in your timezone, dark otherwise. Both hours are configurable. The server picks the theme, so no JavaScript is needed.
- `none`: no built-in CSS; only your Custom CSS applies.

Custom CSS and Custom JS are user settings. The markup is stable and uses only semantic elements:

| What | Selector |
|---|---|
| Main navigation | `body > header nav[aria-label="Main"]` (current item: `[aria-current="page"]`) |
| Entry, feed and category list items | `main > article` |
| Read / unread entry | `article[data-status="read"]`, `article[data-status="unread"]` |
| Starred entry | `article[data-starred="true"]` |
| Feed with errors / unread items | `article[data-errors]`, `article[data-unread]` |
| Entry content | `main > article > section` |
| Action buttons | `article footer form button`, `nav[aria-label="Entry actions"]` |
| Page actions, pagination | `nav[aria-label="Page actions"]`, `nav[aria-label="Pagination"] a[rel=prev\|next]` |
| Messages | `[role="alert"]` (errors), `[role="status"]` (info) |
| Confirmations | `details > summary` + `form` |

The reference stylesheets in `internal/ui/static/css/` are a good starting point: `base.css` holds the layout, and `light.css` / `dark.css` hold the colors.

## License

Apache-2.0. Portions are derived from Miniflux; see `NOTICE`.
