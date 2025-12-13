# Changelog

All notable changes to this project will be documented in this file.

The format is based on “Keep a Changelog”, and this project aims to follow Semantic Versioning.

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
