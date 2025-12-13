# Testing

This document is the manual + semi-automated test plan for `sonoscli`.

Goals:
- Catch regressions in discovery/topology, grouping, and playback control.
- Provide a repeatable “does this work on my network?” checklist.

## Prereqs

- Go `1.22+`
- `golangci-lint` installed (for `make lint` / `pnpm lint`)
- Sonos speakers reachable on the local network (UDP SSDP + TCP 1400)

## Quick checks (automated)

Run from repo root:

- `pnpm -s build`
- `pnpm -s format:check`
- `pnpm -s test`
- `pnpm -s lint`
- `make ci`

Expected:
- All commands exit `0`
- CI should match `.github/workflows/ci.yml` (`gofmt`, `go vet`, `go test`, `golangci-lint`)

## Live network test plan (manual)

Notes:
- Some tests change grouping and playback. Prefer using a “test room” (e.g. `Office`).
- Use `--timeout 10s` if your network is slow.

### 1) CLI basics

- `sonos --version` prints the version (matches `internal/cli/version.go`)
- `sonos --help` works
- `sonos <cmd> --help` works for core commands (`discover`, `status`, `group`, `open`, `scene`, `smapi`)

### 2) Discovery + topology

### 2.5) Discovery (advanced)

- `sonos discover --all` includes invisible/bonded devices (useful for debugging)
- `sonos discover --format json` prints structured results
- `sonos discover --format tsv` prints tab-separated output


- `sonos discover --timeout 6s`
  - Expected: prints all visible rooms (name, IP, UUID)
- `sonos group status`
  - Expected: prints group coordinators + members
- `sonos status --name "<room>"`
  - Expected: prints speaker metadata + playback state

Regression checks:
- If SSDP multicast fails, discovery should fall back to subnet scan + topology and still find rooms.

### 3) Volume + mute

### 3.5) Config defaults

- `sonos config path` prints where config is stored
- `sonos config set defaultRoom "Office"` then run a command without `--name`/`--ip`:
  - `sonos volume get` (should target the default room)
- `sonos config unset defaultRoom` then run `sonos volume get` (should error and ask for `--name/--ip`)


Pick a room:

- `sonos volume get --name "<room>"`
- `sonos mute get --name "<room>"`
- `sonos volume set --name "<room>" 12`
- `sonos mute on --name "<room>"`
- `sonos mute off --name "<room>"`

Expected:
- Values change immediately and `sonos status` reflects the new values.

### 4) Grouping controls

### 4.5) Group volume/mute

Create a small temporary group (recommended: join `Pantry` to `Office`) and validate group-wide controls:

- `sonos group join --name Pantry --to Office`
- `sonos group volume get --name Office`
- `sonos group volume set --name Office 18`
- `sonos group mute toggle --name Office` (twice to return to original)
- `sonos group dissolve --name Office` (splits the test group)


Pick a coordinator room and a second room:

- `sonos group join --name "<member>" --to "<coordinator>"`
- `sonos group status` shows member under coordinator
- `sonos group unjoin --name "<member>"`
- `sonos group status` shows member as its own coordinator

Optional:
- `sonos group party --name "<coordinator>"` (joins all visible rooms)
- `sonos group dissolve --name "<coordinator>"` (ungroups all members)
- `sonos group solo --name "<room>"` (ensures it plays by itself)

### 5) Inputs (TV/Line-in)

TV input (soundbars/home theater):
- Ensure the target is the *soundbar* (e.g. `Living Room`) and it is a standalone coordinator:
  - `sonos group solo --name "<soundbar room>"`
- `sonos tv --name "<soundbar room>"`
- `sonos status --name "<soundbar room>"` should show a URI like `x-sonos-htastream:<UUID>:spdif`

Line-in (devices with analog-in, e.g. Sonos Five):
- `sonos linein --name "<room>"` (defaults `--from` to the same room)
- `sonos status --name "<room>"` should show a URI like `x-rincon-stream:<UUID>`

### 6) Spotify playback (no Spotify Web API creds)

This uses Sonos queueing (AVTransport) + Spotify share links.

- `sonos open --name "<room>" "https://open.spotify.com/track/<id>"`
- `sonos open --name "<room>" "https://open.spotify.com/album/<id>"`
- `sonos enqueue --name "<room>" "spotify:track:<id>"`
- `sonos next --name "<room>"`
- `sonos pause --name "<room>"`
- `sonos play --name "<room>"`
- `sonos stop --name "<room>"`

Expected:
- Playback starts, track info updates in `sonos status`

### 7) Queue management

- `sonos queue list --name "<room>"`
- `sonos queue clear --name "<room>"`

Expected:
- List shows items when queue is in use
- Clear empties the queue

### 8) Scenes (grouping + volume presets)

Scenes should only include *visible* rooms (not bonded satellites/subs).

- `sonos scene save __tmp_test`
- `sonos scene apply __tmp_test`
- `sonos scene list`
- `sonos scene delete __tmp_test`

Expected:
- No `soap http 500` errors on systems with home theater / stereo bonded devices.

### 9) Sonos Favorites

- `sonos favorites list --name "<room>" --timeout 10s`
- `sonos favorites open --name "<room>" --index 1`

Expected:
- Lists favorites; plays selected item (if any exist).

### 10) SMAPI (Sonos music services)

SMAPI is Sonos-side browsing/search for linked services (e.g. Spotify). It may require a one-time DeviceLink/AppLink auth flow.

- `sonos smapi services`
- `sonos smapi categories --service "Spotify"`
- `sonos smapi search --service "Spotify" --category tracks "miles davis"`

If auth is required:
- `sonos auth smapi begin --service "Spotify"` (follow the `regUrl` and link code)
- `sonos auth smapi complete --service "Spotify" --code "<linkCode>" --wait 2m`

Expected:
- Categories show at least `tracks`, `albums`, `artists`, `playlists` for Spotify.
- Search returns results after auth is completed.

### 11) Event watching (manual)

- `sonos watch --name "<room>" --duration 15s` (or omit `--duration` and Ctrl+C)
- Change volume / skip track in another controller/app.

Expected:
- Events stream in (may take a few seconds after the change); stop with Ctrl+C.

### 12) Shell completions

- `sonos completion zsh`
- `sonos completion bash`
- `sonos completion fish`
- `sonos completion powershell`

Expected: prints a completion script to stdout.

## Latest run (example record)

Fill this in when doing an end-to-end run.

- Date: `2025-12-13T17:19:17Z`
- Commit SHA: `e44aca3`
- Network: `192.168.0.0/24`
- Discovery result (rooms found): `Bar, Bedroom, Hallway, Kitchen, Living Room, Master Bathroom, Office, Pantry`
- Notes/issues:
  - Verified: `sonos --version` prints `0.1.27`.
  - Verified: `sonos discover --timeout 6s` finds all expected rooms.
  - Verified: `sonos group solo --name Office` isolates `Office` for playback.
  - Verified: Spotify playback on Office:
    - `sonos open --name Office https://open.spotify.com/album/4o9BvaaFDTBLFxzK70GT1E?...` starts playback.
    - `sonos next|prev|pause|play|stop --name Office` work (`prev` does not error).
  - Verified: Group controls:
    - `sonos group join --name Pantry --to Office`
    - `sonos group volume get/set --name Office`, `sonos group mute toggle --name Office`, `sonos group dissolve --name Office`
  - Verified: SMAPI / Spotify search:
    - `sonos smapi categories --service Spotify` works.
    - `sonos play spotify --name Office "Gareth Emery"` starts playback.
  - Verified: Favorites:
    - `sonos favorites list --name Office` and `sonos favorites open --name Office --index 1` work.
  - Verified: TV input + stop no-op:
    - `sonos tv --name "Living Room"` sets URI to `x-sonos-htastream:...:spdif`
    - `sonos stop --name "Living Room"` returns success (no-op for TV input).
  - Restored original state via `sonos scene save/apply/delete __restore_before_testing_2025_12_13` (used `--timeout 25s`).
