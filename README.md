# sonoscli

`sonoscli` is a small Go CLI to control Sonos speakers over your local network (UPnP/SOAP).

Highlights:
- Auto-discover speakers (SSDP)
- Group-coordinator aware commands (targets the coordinator automatically)
- Playback controls: `play`, `pause`, `stop`, `next`, `prev`
- Status output (text or JSON)
- Volume/mute controls
- Spotify enqueue/play from `spotify:<type>:<id>` or Spotify share URLs (no Spotify credentials required)

This is not an official Sonos project.

## Install / Build

```bash
go build -o sonos ./cmd/sonos
```

Verify:

```bash
./sonos --version
./sonos --help
```

## Dev (format/lint/test)

```bash
make fmt
make test
make lint
```

`make lint` requires `golangci-lint` to be installed. Examples:

```bash
brew install golangci-lint
# or:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

CI runs formatting checks (`gofmt`), `go vet`, tests, and `golangci-lint` on every push/PR.

## Usage

Discover speakers:

```bash
./sonos discover
./sonos discover --json
```

Targeting:
- `--name "Kitchen"` targets a speaker by Sonos room name.
- `--ip 192.168.1.50` targets a speaker by IP.

Most commands are sent to the *group coordinator* (the Sonos device that controls transport for the group). This CLI resolves the coordinator automatically.

Show status:

```bash
./sonos status --name "Kitchen"
./sonos status --name "Kitchen" --json
```

Playback:

```bash
./sonos play --name "Kitchen"
./sonos pause --name "Kitchen"
./sonos stop --name "Kitchen"
./sonos next --name "Kitchen"
./sonos prev --name "Kitchen"
```

Volume / mute:

```bash
./sonos volume get --name "Kitchen"
./sonos volume set --name "Kitchen" 25

./sonos mute get --name "Kitchen"
./sonos mute on --name "Kitchen"
./sonos mute off --name "Kitchen"
./sonos mute toggle --name "Kitchen"
```

## Spotify

Prereqs:
- Spotify must already be linked in the Sonos app for your system.
- This CLI does not authenticate with Spotify; it only enqueues Sonos “Spotify” URIs/metadata.

Enqueue + play:

```bash
./sonos open --name "Kitchen" spotify:track:6NmXV4o6bmp704aPGyTVVG
./sonos open --name "Kitchen" https://open.spotify.com/track/6NmXV4o6bmp704aPGyTVVG
```

Enqueue only:

```bash
./sonos enqueue --name "Kitchen" spotify:playlist:37i9dQZF1DXcBWIGoYBM5M
```

Notes:
- The enqueue implementation tries Spotify Sonos service numbers `2311` and `3079` for compatibility.

## Global flags

- `--ip <ip>`: target by IP
- `--name <name>`: target by speaker name
- `--timeout <duration>`: discovery/network timeout (default `5s`)
- `--json`: JSON output where supported
- `--debug`: reserved for future logging controls

## Troubleshooting

- `discover` finds nothing:
  - Some networks block multicast/SSDP (VLANs, “client isolation”, some mesh Wi‑Fi setups).
  - Ensure your machine and Sonos devices are on the same L2 network/subnet.
- Commands fail with network/SOAP errors:
  - Verify `http://<speaker-ip>:1400/` is reachable.
  - Target the group coordinator (or use `--name` which resolves it).
- Spotify enqueue fails:
  - Confirm Spotify is linked and playable in the Sonos app.
  - Some Sonos setups/firmware variants behave differently for certain Spotify item types.

## Inspiration / references

This project was informed by the Sonos control ecosystem and the SoCo Python library:

```text
https://github.com/SoCo/SoCo
```

## License

See `LICENSE`.
