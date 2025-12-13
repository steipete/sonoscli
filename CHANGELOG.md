# Changelog

All notable changes to this project will be documented in this file.

The format is based on “Keep a Changelog”, and this project aims to follow Semantic Versioning.

## [0.1.18] - 2025-12-13

### Added
- `sonos play spotify "<query>"`:
  - Sonos SMAPI search (no Spotify Web API credentials).
  - Enqueues and starts playback on the target room (`--name/--ip`).

## [0.1.17] - 2025-12-13

### Added
- `sonos group solo --name "<Room>"` to ungroup the current group and leave only that room for playback.

## [0.1.16] - 2025-12-13

### Added
- `sonos config` for local defaults:
  - `sonos config set defaultRoom "Office"` to make `--name` optional.
  - `sonos config set format json` to default `--format`.

## [0.1.15] - 2025-12-13

### Changed
- `sonos group status` now hides invisible/bonded devices by default (use `--all` to include them).

## [0.1.14] - 2025-12-13

### Added
- Discovery improvements:
  - Topology parsing now includes nested home-theater satellites (and other nested members) when present.
  - `sonos discover --all` to include invisible/bonded devices (advanced).
  - `sonos discover --timeout` now bounds the whole discovery operation (SSDP + topology fallback).

## [0.1.13] - 2025-12-13

### Added
- More SMAPI helpers:
  - `sonos smapi categories --service "Spotify"` to list available search categories for a service.
  - `sonos smapi browse --service "Spotify" --id root` to browse containers via SMAPI `getMetadata` (drill down by passing returned ids).

## [0.1.12] - 2025-12-13

### Added
- Sonos-side music-service search via SMAPI (no Spotify Web API credentials required):
  - `sonos smapi services` to list available services and auth types.
  - `sonos smapi auth begin|complete --service "Spotify"` for DeviceLink/AppLink linking.
  - `sonos smapi search --service "Spotify" --category tracks "<query>"` to print canonical Spotify URIs (e.g. `spotify:track:...`).
  - Optional `--open/--enqueue` to immediately play/enqueue a selected search result on a target speaker (`--name/--ip`, `--index`).
- Local SMAPI token store under your user config dir (`~/.config/sonoscli/smapi_tokens.json`, mode `0600`).

## [0.1.11] - 2025-12-13

### Added
- `sonos watch --name "<Room>"`:
  - UPnP event subscriptions (`AVTransport`, `RenderingControl`) with a local callback server.
  - `--format json` prints one event object per line; `--format tsv` prints one row per variable change.

## [0.1.10] - 2025-12-13

### Added
- Output formats:
  - Global `--format plain|json|tsv` flag.
  - Deprecated `--json` (alias for `--format json`).
  - JSON output is now consistent across action commands (prints an `{ ok, action, ... }` object).

## [0.1.9] - 2025-12-13

### Added
- Grouping ergonomics:
  - Fuzzy room matching for `sonos group join --to ...` (substring match, with suggestions on ambiguity).
  - `sonos group party --to "<RoomOrIP>"` to join all visible speakers to a target group.
  - `sonos group dissolve --name "<Room>"` to ungroup every member of a group (coordinator last).

## [0.1.8] - 2025-12-13

### Added
- Additional music source commands:
  - `sonos play-uri --name "<Room>" "<uri>" [--title "..."] [--radio]`
  - `sonos linein --name "<Room>" [--from "<RoomWithLineIn>"]`
  - `sonos tv --name "<Room>"`

## [0.1.7] - 2025-12-13

### Added
- Sonos Favorites:
  - `sonos favorites list --name "<Room>" [--start N] [--limit N]` (and `--json`)
  - `sonos favorites open --name "<Room>" --index <N>` or `sonos favorites open --name "<Room>" "<title>"`

## [0.1.6] - 2025-12-13

### Added
- Scenes (presets) stored under your user config directory:
  - `sonos scene save <name>`: captures grouping + per-room volume/mute
  - `sonos scene apply <name>`: restores grouping + per-room volume/mute
  - `sonos scene list`, `sonos scene delete <name>`

## [0.1.5] - 2025-12-13

### Added
- Group audio controls via `GroupRenderingControl`:
  - `sonos group volume get|set --name "<Room>"`
  - `sonos group mute get|on|off|toggle --name "<Room>"`

## [0.1.4] - 2025-12-13

### Added
- `sonos status` now parses `TrackMetaData` DIDL (when available) and shows:
  - `Title`, `Artist`, `Album`, and `AlbumArt` (absolute URL when possible)
- `sonos now` as an alias for `sonos status`.

## [0.1.3] - 2025-12-13

### Added
- Queue management:
  - `sonos queue list --name "<Room>" [--start N] [--limit N]` (and `--json`)
  - `sonos queue play --name "<Room>" <pos>` (1-based)
  - `sonos queue remove --name "<Room>" <pos>` (1-based)
  - `sonos queue clear --name "<Room>"`

## [0.1.2] - 2025-12-13

### Added
- Grouping commands:
  - `sonos group status` (lists coordinators + members, `--json` supported)
  - `sonos group join --name "<Room>" --to "<RoomOrIP>"`
  - `sonos group unjoin --name "<Room>"`
- `docs/spec.md` documenting the full CLI design and feature set.

## [0.1.1] - 2025-12-13

### Added
- `sonos search spotify "<query>"`:
  - Searches Spotify via Spotify Web API client credentials and prints playable `spotify:<type>:<id>` URIs.
  - Supports `--type track|album|playlist|show|episode`, `--limit`, optional `--market`.
  - Optional `--open` / `--enqueue` to immediately play/enqueue a selected result on Sonos (`--index`).
  - Credentials via `SPOTIFY_CLIENT_ID` / `SPOTIFY_CLIENT_SECRET` or `--client-id` / `--client-secret`.

## [0.1.0] - 2025-12-13

### Added
- `sonos discover`:
  - SSDP M-SEARCH discovery (fast path).
  - Topology discovery via `ZoneGroupTopology.GetZoneGroupState` (reliable path; finds all rooms).
  - Fallback subnet scan (checks port `1400`, verifies `device_description.xml`) for networks where SSDP is blocked/unreliable.
  - Optional `--json` output.
- Coordinator-aware targeting via `--name` / `--ip` (commands sent to the group coordinator when possible).
- `sonos status` with text and `--json` output (transport + position + volume/mute).
- Transport controls: `sonos play`, `pause`, `stop`, `next`, `prev`.
- Volume controls: `sonos volume get|set`.
- Mute controls: `sonos mute get|on|off|toggle`.
- Spotify enqueue/play:
  - `sonos enqueue <spotify-uri-or-link>`
  - `sonos open <spotify-uri-or-link>`
  - Supports `spotify:<type>:<id>` and common `open.spotify.com/...` share URLs.
  - Tries Sonos Spotify service numbers `2311` and `3079`.
- `--version` support (prints `sonos 0.1.0`).
- Developer tooling:
  - `Makefile` targets: `fmt`, `fmt-check`, `test`, `build`, `lint`, `ci`
  - `.golangci.yml` for `golangci-lint`
  - `package.json` pnpm helper scripts: `pnpm sonos`, `pnpm build`, `pnpm test`, `pnpm format`, `pnpm lint`
- GitHub Actions CI (format check, `go vet`, tests, `golangci-lint`).
- Tests for key parsers and SOAP helpers (SSDP, ZoneGroupState, Spotify refs, SOAP response/error parsing).
- `.gitignore` improvements (macOS `.DS_Store`, pnpm/node artifacts, common Go build outputs).
