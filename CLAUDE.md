# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Build
make build                    # Builds to bin/sonos
go build -o sonos ./cmd/sonos # Alternative direct build

# Test
make test                     # Run all tests
go test ./...                 # Run all tests directly
go test ./internal/cli/...    # Run tests for a specific package
go test -run TestFoo ./...    # Run a single test by name

# Format & Lint
make fmt                      # Format code with gofmt
make fmt-check                # Check formatting (CI uses this)
make lint                     # Run golangci-lint (requires golangci-lint installed)

# CI (format check + tests + go vet)
make ci
```

## Architecture

This is a Go CLI for controlling Sonos speakers over UPnP/SOAP on the local network.

### Package Structure

- `cmd/sonos/` - Main entrypoint, just calls `cli.Execute()`
- `internal/cli/` - Cobra commands, output formatting, and CLI logic. Each command lives in its own file (e.g., `group.go`, `volume.go`, `smapi.go`)
- `internal/sonos/` - Core Sonos UPnP/SOAP client: SSDP discovery, topology parsing, AVTransport/RenderingControl actions, event subscriptions, DIDL metadata parsing
- `internal/spotify/` - Spotify Web API search helper (optional, requires credentials)
- `internal/appconfig/` - User config storage (`~/.config/sonoscli/config.json`)
- `internal/scenes/` - Scene (preset) storage (`~/.config/sonoscli/scenes.json`)

### Key Concepts

**Coordinator Awareness**: Transport commands (play/pause/stop/next/prev, queue operations) must be sent to the group coordinator, not individual speakers. The CLI resolves coordinators automatically via `ZoneGroupTopology.GetZoneGroupState`.

**Discovery Strategy**: SSDP multicast → find any Sonos device → query topology for full room list. Falls back to subnet scanning on port 1400 if SSDP fails.

**Dependency Injection for Testing**: CLI commands use function variables (e.g., `newSonosClient`, `sonosDiscover`) that tests can replace with stubs. See `*_test.go` files for patterns.

### Output Formats

Commands support `--format plain|json|tsv`. The `formatOutput()` helper in `internal/cli/output.go` handles structured output consistently.

## Testing Notes

- Unit tests focus on parsing/transformation logic (SSDP, SOAP, topology, DIDL, Spotify refs)
- CLI tests use dependency injection to stub external dependencies
- Integration tests against real speakers are manual only (see `docs/testing.md`)
- CI enforces 70% minimum coverage
