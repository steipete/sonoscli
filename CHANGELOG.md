# Changelog

## [0.3.0] - 2026-05-07

### Added
- `sonos play youtube` now resolves YouTube URLs with `yt-dlp` and plays the direct audio stream on Sonos.
- `sonos play-url` now starts a short-lived local stream proxy for YouTube, podcasts, radio streams, and other URLs, with `Sonos CLI` ICY metadata and automatic idle/EOF exit.

### Changed
- CI now runs coverage with atomic counters, enforces a stricter whole-repo coverage floor, and enforces an 85% coverage floor for the new stream proxy package.
- Go linting now enables stricter correctness linters for response-body handling, error wrapping, nil-error mistakes, wasted assignments, and standard-library constant usage.

## [0.2.0] - 2026-05-06

### Added
- Release automation now publishes GoReleaser builds from tags and can backfill an existing tag.
- Release artifacts now include Linux arm64 and Windows arm64 builds.
- Release automation now dispatches the Homebrew tap updater after publishing, using the GoReleaser macOS arm64 archive name.

### Fixed
- Spotify track/episode enqueue now tries Sonos `sid`/`sn` queue URIs before legacy forms, improving compatibility with speakers that reject the older track URI shape. Thanks @WinnCook.
- `sonos smapi search|browse --open/--enqueue` can now enqueue non-Spotify SMAPI items using the generic Sonos queue path instead of rejecting them as unsupported Spotify refs.
- SMAPI AppLink auth requests now include the client metadata expected by AppLink services such as QQ Music.
- SMAPI AppLink auth now reports native app authentication URLs instead of printing empty link instructions for services such as Apple Music that do not return a device-link code.

### Changed
- GitHub Actions workflows now use the Node 24 based official checkout/setup-go actions.

## [0.1.1] - 2025-12-14

### Added
- `--name` flag shell completion suggests discovered speaker names (with a short-lived cache). Thanks @javisoto.

### Fixed
- `--name` completion now falls back to the on-disk cache if discovery fails (even if the cache is slightly stale).

## [0.1.0] - 2025-12-13

### Added
- `sonos discover`:
  - SSDP M-SEARCH discovery (fast path).
  - Topology discovery via `ZoneGroupTopology.GetZoneGroupState` (reliable path; finds all rooms).
  - Fallback subnet scan (checks port `1400`, verifies `device_description.xml`) for networks where SSDP is blocked/unreliable.
  - Curl-based HTTP/SOAP retry for private LAN endpoints when Go's HTTP stack times out (workaround for some Sonos/network keep-alive quirks).
  - `--timeout` bounds the overall discovery operation; `--all` includes invisible/bonded devices.
- Output formats:
  - Global `--format plain|json|tsv` flag (and deprecated `--json` alias for `--format json`).
  - Consistent JSON output shape for action commands (`{ ok, action, ... }`).
- Coordinator-aware targeting via `--name` / `--ip` (commands sent to the group coordinator when possible).
- `sonos status` (and `sonos now` alias) showing transport/position + volume/mute, plus parsed DIDL `TrackMetaData` when available (`Title`, `Artist`, `Album`, `AlbumArt`).
- Transport controls: `sonos play`, `pause`, `stop`, `next`, `prev`.
- Volume controls: `sonos volume get|set`.
- Mute controls: `sonos mute get|on|off|toggle`.
- Grouping:
  - `sonos group status` (coordinators + members; `--all` to include invisible/bonded devices).
  - `sonos group join --name "<Room>" --to "<RoomOrIP>"`, `sonos group unjoin --name "<Room>"`.
  - `sonos group party --to "<RoomOrIP>"` to join all visible speakers to a target group.
  - `sonos group dissolve --name "<Room>"` to ungroup every member of a group (coordinator last).
  - `sonos group solo --name "<Room>"` to isolate a single room for playback.
  - Group audio controls via `GroupRenderingControl`:
    - `sonos group volume get|set --name "<Room>"`
    - `sonos group mute get|on|off|toggle --name "<Room>"`
- Queue management:
  - `sonos queue list --name "<Room>" [--start N] [--limit N]`
  - `sonos queue play --name "<Room>" <pos>` (1-based)
  - `sonos queue remove --name "<Room>" <pos>` (1-based)
  - `sonos queue clear --name "<Room>"`
- Sonos Favorites:
  - `sonos favorites list --name "<Room>" [--start N] [--limit N]`
  - `sonos favorites open --name "<Room>" --index <N>` or `sonos favorites open --name "<Room>" "<title>"`
- Scenes (presets) stored under your user config directory:
  - `sonos scene save <name>`: captures grouping + per-room volume/mute
  - `sonos scene apply <name>`: restores grouping + per-room volume/mute
  - `sonos scene list`, `sonos scene delete <name>`
- Extra music sources:
  - `sonos play-uri --name "<Room>" "<uri>" [--title "..."] [--radio]`
  - `sonos linein --name "<Room>" [--from "<RoomWithLineIn>"]`
  - `sonos tv --name "<Room>"`
- Spotify playback on Sonos:
  - `sonos enqueue <spotify-uri-or-link>` (does not start playback)
  - `sonos open <spotify-uri-or-link>` (enqueue + start playback)
  - Accepts `spotify:<type>:<id>` and common `open.spotify.com/...` share URLs.
  - Tries Sonos Spotify service numbers `2311` and `3079`.
- Spotify Web API search (requires client credentials):
  - `sonos search spotify "<query>"` prints playable `spotify:<type>:<id>` URIs.
  - Supports `--type track|album|playlist|show|episode`, `--limit`, optional `--market`.
  - Optional `--open` / `--enqueue` to immediately play/enqueue a selected result on Sonos (`--index`).
  - Credentials via `SPOTIFY_CLIENT_ID` / `SPOTIFY_CLIENT_SECRET` or `--client-id` / `--client-secret`.
- Sonos music services (SMAPI):
  - `sonos smapi services` to list available services and auth types.
  - `sonos smapi categories --service "Spotify"` to list available search categories for a service.
  - `sonos smapi browse --service "Spotify" --id root` to browse containers via SMAPI `getMetadata`.
  - `sonos smapi search --service "Spotify" --category tracks "<query>"` to print canonical Spotify URIs (e.g. `spotify:track:...`).
  - Optional `--open/--enqueue` to immediately play/enqueue a selected search result on a target speaker (`--name/--ip`, `--index`).
  - `sonos play spotify --name "<Room>" "<query>"` to search via SMAPI and play the top result (no Spotify Web API creds).
- Authentication helpers:
  - `sonos auth smapi begin|complete --service "Spotify"` for DeviceLink/AppLink linking.
  - Backwards-compatible: `sonos smapi auth ...` still works but is hidden from help output.
  - Local SMAPI token store under your user config dir (`~/.config/sonoscli/smapi_tokens.json`, mode `0600`).
- Event watching:
  - `sonos watch --name "<Room>"` subscribes to `AVTransport` + `RenderingControl` and prints live updates (`--format json|tsv`).
- Local CLI defaults:
  - `sonos config set defaultRoom "Office"` to make `--name` optional for commands that require a target.
  - `sonos config set format json` to default `--format`.
  - Stored under your user config dir (e.g. `~/.config/sonoscli/config.json`, mode `0600`).
- Diagnostics:
  - `--debug` enables detailed trace logs for SSDP discovery, topology queries, and SOAP calls.
- Developer tooling:
  - `Makefile` targets: `fmt`, `fmt-check`, `test`, `build`, `lint`, `ci`
  - `.golangci.yml` for `golangci-lint`
  - `package.json` pnpm helper scripts: `pnpm sonos`, `pnpm build`, `pnpm test`, `pnpm format`, `pnpm lint`
  - GitHub Actions CI (format check, `go vet`, tests with coverage floor, `golangci-lint`)
  - `.gitignore` includes macOS `.DS_Store`, pnpm/node artifacts, and common Go build outputs.
- Docs: `docs/spec.md` documenting the CLI design and feature set; `docs/testing.md` manual test plan and run log.

### Changed
- `sonos group status` hides invisible/bonded devices by default (use `--all` to include them).
- `sonos auth smapi complete --wait ...` prints progress while waiting (so it doesn’t look hung).
- `sonos discover` now returns a non-zero exit status when no speakers are found in non-JSON output modes (instead of silently printing nothing).

### Fixed
- Speaker name resolution prefers visible rooms over invisible/bonded devices when names collide (common with home-theater surrounds), fixing commands like `sonos group solo --name "Living Room"` selecting the wrong device.
- `sonos scene save/apply` ignores invisible/bonded devices (satellites, subs, etc.), preventing `soap http 500` failures on systems with home theater/stereo setups.
- `sonos smapi categories` correctly detects nested `<SearchCategories>` in modern presentation maps (e.g. Spotify).
- Reduced timeouts/hangs for topology/device-description calls on some systems by disabling HTTP keep-alives for Sonos requests (and bypassing proxy env vars for private IPs).
- `sonos auth smapi complete --wait <duration>` polls for account-link completion (handles Spotify `NOT_LINKED_RETRY`).
- `sonos prev` restarts the current track (seek to `0:00:00`) when the source rejects `Previous` (UPnP `701`/`711`), instead of failing.
- `sonos stop` is treated as a no-op when the source rejects stop with UPnP `701` (e.g. TV input).
