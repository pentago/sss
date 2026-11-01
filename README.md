# sss

A minimalist self-hosted feed reader written in Go.

sss is an opinionated fork of already opinionated and beloved [Miniflux](https://miniflux.app/) project. I know, lots of opinions here.

Primarily, I use web UI (PWA) for reading thus the main goal of this fork was to make the web UI more usable by improving readability, removing clutter, and removing unnecessary fat. Full API and trimmed roster of integrations is still there to use with clients.

![sss screenshot](docs/screenshot.png)

Key differences from Miniflux:

**Storage and runtime**

- **SQLite** storage. It uses the pure Go driver (no CGO), so the binary stays static. Search uses SQLite FTS5 over titles and content: words, `"quoted phrases"`, `or` and `-term`, ranked by relevance and recency. A query that only excludes words is ignored.
- **Honoring XDG on Linux:** the config file lives at `~/.config/sss/sss.conf` and the database at `~/.local/share/sss/sss.db`.
- **Configuration:** same variable names, with a few removed or added (see [Configuration](#configuration)). A missing `-c` file only logs a warning. A non-numeric value for a numeric setting stops startup instead of falling back to the default.
- **English only.** There is no language setting.

**Web UI**

- **Classless semantic HTML.** Pages use only plain elements, with no `class`, `id` or `style` attributes, so themes are easier to write CSS for.
- **Mark as read on scroll** - optional. It marks entries as read when they scroll out of view.
- **No JavaScript by default.** Every action is a plain HTML form. Keyboard shortcuts, swipe gestures, gesture navigation, mark-as-read-on-scroll, the offline service worker and passkeys are opt-in.
- **Themes:** dark_serif theme by default. sss adds `oled_*` (true black), `time_*` / `time_oled_*` (light by day, dark or OLED black by night, picked by the server) and `none` (Custom CSS only), see [Themes](#themes).
- **Readability:** Bundled-in amazing [Noto fonts](https://fonts.google.com/noto) (Latin and Latin Extended; other scripts fall back to system fonts).
- **Simpler header:** Top of reader is now less crowded, with links collapsable into a menu on narrow screens.
- **Minimal entry lists:** Less crowded lists with focus on content and less misclicks.
- **Global feed settings:** Settings → Global provides a way to configure certain settings globally, for all feeds.
- **Per-category refresh cooldown:** Not all categories are equally important.
- **Hide empty categories:** - optional. Settings → Reading section provides way to hide categories with no feeds.
- **The first user is an admin**, however it is created.

**Packaging**

- **size matters:** A smaller binary and minimal `scratch` container image.
- **systemd:** provided secure user systemd unit.

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

The Linux packages install `/usr/bin/sss`, a systemd user unit and `/usr/share/doc/sss/sss.conf.example`. sss is configured per user only; there is no system-wide config in `/etc`.

### Run as your user (XDG)

```sh
mkdir -p ~/.config/sss
cp /usr/share/doc/sss/sss.conf.example ~/.config/sss/sss.conf   # sets RUN_MIGRATIONS=1, so upgrades migrate on start
sss -migrate                                                    # creates ~/.local/share/sss/sss.db
sss -create-admin                                               # interactive; needs the migrated database
systemctl --user enable --now sss                               # Linux; or just run `sss`
```

- **Config file:** `$XDG_CONFIG_HOME/sss/sss.conf` (default `~/.config/sss/sss.conf`).
- **Database:** `$XDG_DATA_HOME/sss/sss.db` (default `~/.local/share/sss/sss.db`); the directory is created automatically.
- **Linux:** to keep the user service running after you log out, run `loginctl enable-linger $USER`.
- **macOS:** there is no launchd service; run `sss` directly. It uses the same paths.

### From source

```sh
go build -o sss .
RUN_MIGRATIONS=1 CREATE_ADMIN=1 ADMIN_USERNAME=admin ADMIN_PASSWORD=changeme ./sss
```

The server listens on `127.0.0.1:8080` by default. Run `sss -h` to list the maintenance flags (`-migrate`, `-reset-password`, `-healthcheck auto`, `-config-dump`, …). For cron-driven polling, set `DISABLE_SCHEDULER_SERVICE=1` and run `sss -refresh-feeds` periodically.

### Docker

```sh
docker build -t sss .
docker run -d -p 8080:8080 -v sss-config:/config \
  -e CREATE_ADMIN=1 -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD=changeme sss
```

The image is built from `scratch` and runs as an unprivileged user. Its layout:

- `/sss` is the binary.
- `/config` is the volume. It holds the config file `/config/sss.conf` and the database `/config/sss.db`.

The image ships a default `sss.conf` with `LISTEN_ADDR=0.0.0.0:8080`, `DATABASE_PATH=/config/sss.db` and `RUN_MIGRATIONS=1`. A named volume picks that file up automatically. A bind-mounted directory needs either an `sss.conf` with those lines or the same settings as environment variables; without a file, sss logs a warning and uses the environment only. Environment variables override the file.

`examples/compose.yaml` runs the `:dev` image with `examples/config` (a ready `sss.conf`; the database is created there) bind-mounted to `/config`.

`docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/riscv64 .` builds the image for several architectures.

### Releasing

Push a `v*` tag. The release workflow publishes binaries, the four Linux package types, and multi-arch images to `ghcr.io/<owner>/sss` (tagged with the version and `latest`). The Homebrew cask goes to `<owner>/homebrew-tap`, but only when the repository secret `HOMEBREW_TAP_TOKEN` is set. That secret needs to be a token with write access to the tap repository.

`.github/workflows/ci.yml` runs formatting, vet, race tests and a Docker build on every PR and push to `main`. After those pass on `main` (a merged PR), it publishes multi-arch images tagged `dev` and `pr-<number>`.

Release artifacts, release images and `main` images get signed build provenance attestations. Verify one with `gh attestation verify <file> --owner <owner>` or `gh attestation verify oci://ghcr.io/<owner>/sss:<tag> --owner <owner>`.

## Configuration

Settings use [Miniflux's configuration names](https://miniflux.app/docs/configuration.html), including the `_FILE` variants for secrets. They can come from two places:

- **A config file:** `-c`/`-config-file <path>`, or `$XDG_CONFIG_HOME/sss/sss.conf` when it exists. It holds one `KEY=VALUE` per line, and lines starting with `#` are comments. Unlike Miniflux, a missing `-c` file is not an error: sss logs a warning and uses the environment only.
- **Environment variables**, which override the file.

These differ from Miniflux:

| Variable | Status |
|---|---|
| `DATABASE_PATH` | Replaces `DATABASE_URL`. Path to the SQLite file or a `file:` URI. Default `$XDG_DATA_HOME/sss/sss.db`, where `XDG_DATA_HOME` defaults to `~/.local/share`. When there is no home directory, the default is `sss.db` in the working directory. |
| `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_CONNECTION_LIFETIME` | Removed (SQLite) |
| `METRICS_*` | Same as Miniflux, but metric names start with `sss_` instead of `miniflux_`. |
| `PPROF_LISTEN_ADDR` | New. Serves Go's `/debug/pprof/` profiles and execution traces on this address (e.g. `127.0.0.1:6060`). Default off. Unauthenticated, so keep it on localhost. See [docs/instrumentation.md](docs/instrumentation.md). |
| `HTTP_CLIENT_PROXIES`, `FETCHER_ALLOW_PRIVATE_FEED_PROXY` | Removed |
| `FETCH_NEBULA_WATCH_TIME`, `FETCH_ODYSEE_WATCH_TIME`, `FETCH_BILIBILI_WATCH_TIME` | Removed (only YouTube watch time is supported) |

Run `./sss -config-dump` to see the values in effect.

## Themes

User settings → Theme. A theme is a color scheme followed by `_serif` or `_sans_serif` (the font of the page and entry content), or `none`. `light_*`, `dark_*` and `system_*` use Miniflux's names; `oled_*`, `time_*`, `time_oled_*` and `none` are sss-only. The default is `dark_serif`. The fonts are Noto Serif and Noto Sans, served by sss itself (Latin scripts only; other scripts use the system serif or sans-serif).

- `light_*` / `dark_*`: the built-in reference themes.
- `oled_*`: true black background with dimmed text, for OLED screens and night reading.
- `system_*`: follows the device's light or dark preference (`prefers-color-scheme`).
- `time_*`: light between two hours of the day in your timezone, dark otherwise. Both hours are configurable. The server picks the theme, so no JavaScript is needed.
- `time_oled_*`: like `time_*`, but true black OLED at night.
- `none`: no built-in CSS; only your Custom CSS applies.

Custom CSS and Custom JS are user settings. The markup is stable and uses only semantic elements.

The reference stylesheets in `internal/ui/static/css/` are a good starting point: `base.css` holds the layout, and `light.css` / `dark.css` / `oled.css` hold the colors.

Project name is a play on next letter after "r" in "rss" → "sss", suggesting moving forward in [RSS](https://en.wikipedia.org/wiki/RSS) ecosystem. It is pronounced whatever you want it to be.

## License

Apache-2.0. Portions are derived from Miniflux; see `NOTICE`.
