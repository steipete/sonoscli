# sonoscli

`sonoscli` is a modern Go CLI to control Sonos speakers over your local network (UPnP/SOAP).

It’s designed for:
- Easy discovery of speakers on your LAN
- Coordinator-aware control (talks to the group coordinator automatically)
- Simple, scriptable commands
- Spotify enqueue/play from `spotify:<type>:<id>` or Spotify share URLs (no Spotify credentials required)

This is not an official Sonos project.

## Requirements

- Your machine must be on the same network as your Sonos system.
- Speakers must be reachable on TCP port `1400` (e.g. `http://<speaker-ip>:1400/`).

Spotify:
- Spotify must already be linked in the Sonos app.
- This CLI does not authenticate with Spotify; it enqueues Sonos “Spotify” URIs/metadata.

## Install / build

Build a local binary:

```bash
go build -o sonos ./cmd/sonos
./sonos --version
./sonos --help
```

## Quick start

Discover speakers:

```bash
./sonos discover
./sonos discover --json
```

Show status (text or JSON):

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
./sonos mute toggle --name "Kitchen"
```

## Targeting and groups

Target a speaker by:
- `--name "Kitchen"` (Sonos room name)
- `--ip 192.168.0.250` (speaker IP)

Most commands must be sent to the *group coordinator* (the device that owns transport state for the group). `sonoscli` resolves the coordinator automatically so commands behave like the Sonos app.

## Spotify

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
- Use `--title` to override the queue display title for some entries.

## Dev workflow

### Makefile

```bash
make fmt
make test
make build
make lint
```

`make lint` requires `golangci-lint`:

```bash
brew install golangci-lint
# or:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### pnpm helper scripts

This repo includes a minimal `package.json` so you can drive the workflow with `pnpm` (no Node dependencies required):

```bash
pnpm build
pnpm test
pnpm format
pnpm lint

pnpm sonos -- discover
pnpm sonos -- status --name "Kitchen"
pnpm sonos -- open --name "Kitchen" spotify:track:6NmXV4o6bmp704aPGyTVVG
```

CI runs: `gofmt` check, `go vet`, `go test`, and `golangci-lint`.

## Global flags

- `--ip <ip>`: target by IP
- `--name <name>`: target by speaker name
- `--timeout <duration>`: discovery/network timeout (default `5s`)
- `--json`: JSON output where supported
- `--debug`: reserved for future logging controls

## Troubleshooting

- `discover` is empty:
  - Some networks block multicast/SSDP; `sonoscli` falls back to scanning local /24 subnets for port `1400` and then uses Sonos topology to list all rooms.
  - Ensure Wi‑Fi client isolation is off and you’re on the same LAN/subnet.
- Commands fail with UPnP/SOAP errors:
  - Verify you can reach `http://<speaker-ip>:1400/` from this machine.
  - Try targeting by `--name` (it resolves the coordinator).
- Spotify enqueue fails:
  - Confirm Spotify is linked and playable in the Sonos app.
  - Some systems behave differently per firmware/service configuration.

## Inspiration / references

This project was informed by the Sonos control ecosystem and the SoCo Python library:

```text
https://github.com/SoCo/SoCo
```

## License

See `LICENSE`.

