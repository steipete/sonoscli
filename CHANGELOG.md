# Changelog

All notable changes to this project will be documented in this file.

The format is based on “Keep a Changelog”, and this project aims to follow Semantic Versioning.

## [0.1.0] - 2025-12-13

### Added
- `sonos discover` (SSDP) with optional `--json` output.
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
- Formatter/linter/dev tooling:
  - `Makefile` targets: `fmt`, `fmt-check`, `test`, `lint`, `ci`
  - `.golangci.yml` for `golangci-lint`
- GitHub Actions CI (format check, `go vet`, tests, `golangci-lint`).
- Tests for key parsers and SOAP helpers (SSDP, ZoneGroupState, Spotify refs, SOAP response/error parsing).
