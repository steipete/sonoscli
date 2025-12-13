# Improvements / Roadmap

This is a living list of potential improvements to `sonoscli`, captured from current gaps vs. typical Sonos controller features. Use it as a backlog; we’ll implement items one-by-one.

Legend:
- **Value**: user-facing impact (High/Med/Low)
- **Effort**: estimated implementation effort (S/M/L)
- **Deps**: prerequisites (none / Sonos-linked services / Spotify Web API creds, etc.)

## P0 (high value)

1) **Queue management**
- Value: High | Effort: M | Deps: none
- Add commands:
  - `sonos queue list --name "<Room>" [--limit N] [--json]`
  - `sonos queue clear --name "<Room>"`
  - `sonos queue play --name "<Room>" <index>` (0-based or 1-based, pick one and document)
  - `sonos queue remove --name "<Room>" <index>`
- Notes:
  - Uses UPnP `ContentDirectory.Browse` (queue container `Q:0`) and `AVTransport.RemoveTrackFromQueue` / `RemoveAllTracksFromQueue` / `Seek TRACK_NR`.
 - Status:
   - Implemented in `0.1.3` (CLI uses 1-based positions).

2) **Better “now playing” metadata**
- Value: High | Effort: M | Deps: none
- Improve `sonos status`:
  - Parse `TrackMetaData` DIDL and show `title`, `artist`, `album`, `albumArtURI` (when available).
  - Optionally add `sonos now` as a friendlier alias.
- Notes:
  - Requires DIDL parsing (we can implement a small subset rather than full DIDL).
 - Status:
   - Implemented in `0.1.4` (adds `sonos now` alias and prints parsed metadata when present).

3) **Group volume + group mute**
- Value: High | Effort: S–M | Deps: none
- Add:
  - `sonos group volume get|set --name "<AnyMember>" <0-100>`
  - `sonos group mute get|on|off|toggle --name "<AnyMember>"`
- Notes:
  - Uses `GroupRenderingControl` service (similar to SoCo patterns).

4) **Presets / scenes**
- Value: High | Effort: L | Deps: none
- Add:
  - `sonos scene save <name>`: capture grouping + volumes (+ optionally what’s playing)
  - `sonos scene apply <name>`
  - `sonos scene list`
- Notes:
  - Needs a config store (file under `~/.config/sonoscli` or similar).

## P1 (nice-to-have)

5) **Music sources (Sonos Favorites)**
- Value: Med–High | Effort: M | Deps: favorites must exist
- Add:
  - `sonos favorites list [--json]`
  - `sonos favorites open --name "<Room>" "<favorite title>"` (or by index)
- Notes:
  - Favorites are available via `ContentDirectory.Browse` (e.g. `FV:2`), and often include metadata needed to play.

6) **Music sources (radio / TuneIn / URI play)**
- Value: Med | Effort: M | Deps: depends on source
- Add:
  - `sonos play-uri --name "<Room>" "<uri>" [--title "..."] [--radio]`
  - `sonos linein --name "<Room>" [--from "<RoomWithLineIn>"]`
  - `sonos tv --name "<Room>"`

7) **Grouping ergonomics**
- Value: Med | Effort: S–M | Deps: none
- Improve:
  - `group join` should accept fuzzy matching and show suggestions on ambiguity.
  - `party` mode: join all speakers to a target.
  - `group dissolve`: unjoin all members of a group.

8) **Output formats**
- Value: Med | Effort: S | Deps: none
- Add:
  - `--format json|tsv|plain` (or expand `--json` to a more general format flag)
  - Consistent machine-readable JSON across all commands.

## P2 (advanced / optional)

9) **Event subscriptions + watch mode**
- Value: Med | Effort: L | Deps: network accessibility to event listener port
- Add:
  - `sonos watch --name "<Room>"` (stream live changes: track/volume/transport)
- Notes:
  - Requires UPnP eventing server on the CLI machine and subscriptions to `AVTransport`/`RenderingControl`.

10) **Sonos-side music-service browsing/search**
- Value: High | Effort: L | Deps: music service linked in Sonos
- Goal:
  - Search/browse via Sonos (SMAPI) so you can find Spotify content without Spotify Web API credentials.
- Notes:
  - This is more complex and service-dependent.

11) **Credential/config management**
- Value: Med | Effort: M | Deps: none
- Add:
  - `sonos config set spotify.client_id ...`
  - Store secrets in Keychain (macOS) or an encrypted local store; fallback to env vars.

## What needs the Spotify Web API?

- Required:
  - `sonos search spotify ...` (human query → IDs/URIs)
  - Rich metadata (covers/artist lists) even when Sonos doesn’t provide it cleanly
- Not required:
  - `sonos open/enqueue` for Spotify when you already have a Spotify URI/share link and Spotify is linked in the Sonos app.
