---
title: sonos smapi
description: Browse and search any music service linked in your Sonos app via the SMAPI protocol.
---

# `sonos smapi`

SMAPI (Sonos Music API) is the same protocol the Sonos app uses to browse and search music services — Spotify, Apple Music, TuneIn, Mixcloud, and friends. No third-party app credentials are needed, but some services require local SMAPI auth before search or playback works.

```
sonos smapi services
sonos smapi categories --service "<Service>"
sonos smapi search     --service "<Service>" --category <cat> <query>
sonos smapi browse     --service "<Service>" [--id <container>] [--recursive]
```

## `sonos smapi services`

Lists the Sonos music-service catalog available to the household/region. A listed service is not necessarily authenticated for local SMAPI search yet.

```bash
sonos smapi services
sonos smapi services --format json
```

## `sonos smapi categories`

Lists the searchable categories for one service.

```bash
sonos smapi categories --service "Spotify"
```

| Flag | Default | What it does |
| --- | --- | --- |
| `--service string` | `Spotify` | Music service name (as shown in `sonos smapi services`). |

## `sonos smapi search`

Searches a linked service.

```bash
sonos smapi search --service "Spotify" --category tracks "miles davis"
sonos smapi search --service "Spotify" --category albums --limit 25 "kind of blue"

# Pick a result and play it
sonos smapi search --service "Spotify" --category tracks "miles davis" \
  --open --name "Kitchen" --index 1
```

| Flag | Default | What it does |
| --- | --- | --- |
| `--service string` | `Spotify` | Service name. |
| `--category string` | `tracks` | Category — service-dependent (`tracks`, `albums`, `artists`, `playlists`). |
| `--limit int` | `10` | Max results (1–200, service-dependent). |
| `--index int` | `1` | Result index for `--open` / `--enqueue` (1-based). |
| `--open` | off | Play the selected result. Requires `--name` / `--ip`. |
| `--enqueue` | off | Enqueue the selected result. Requires `--name` / `--ip`. |

## `sonos smapi browse`

Browse a music service container hierarchically.

```bash
sonos smapi browse --service "Spotify" --id root
sonos smapi browse --service "Spotify" --id <some-container-id>
sonos smapi browse --service "Spotify" --id root --recursive
```

| Flag | Default | What it does |
| --- | --- | --- |
| `--service string` | `Spotify` | Service name. |
| `--id string` | `root` | Container/item id. Use `root` to start. |
| `--limit int` | `50` | Max results. |
| `--recursive` | off | Recursively browse (service-dependent). |
| `--index int` | `1` | Result index for `--open` / `--enqueue`. |
| `--open` | off | Play selected item. |
| `--enqueue` | off | Enqueue selected item. |

## Playback compatibility

For Spotify results, `sonoscli` uses the dedicated Spotify queue URI forms. For other SMAPI services, `--open` and `--enqueue` use the generic Sonos queue path with SoCo-compatible metadata. This is required by some AppLink services, including QQ Music and NetEase Cloud Music.

## Authenticating a service

Some services need a one-time DeviceLink/AppLink flow before SMAPI search works. See [`sonos auth smapi`](sonos-auth-smapi.md).

## See also

- [Spotify & SMAPI](../spotify-and-smapi.md) — picking the right path.
