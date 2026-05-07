---
title: Spotify & SMAPI
description: Two ways to play and search music — open Spotify URIs without credentials, or search via Sonos SMAPI / Spotify Web API.
---

# Spotify & SMAPI

`sonoscli` supports two paths for music. Pick whichever matches what you're trying to do.

## Path 1 — Open a Spotify URI (no credentials)

If Spotify is already linked in the Sonos app, `sonoscli` can play any `spotify:` URI or share link without any Spotify Web API setup:

```bash
sonos open    --name "Kitchen" "https://open.spotify.com/track/6NmXV4o6bmp704aPGyTVVG"
sonos open    --name "Kitchen" spotify:album:0nrRP2bk19rLc0orkWPQk2
sonos enqueue --name "Kitchen" spotify:playlist:37i9dQZF1DXcBWIGoYBM5M --next
```

What happens internally: `sonoscli` builds the canonical `x-sonos-spotify:` URI plus DIDL-Lite metadata that Sonos expects, then calls `AVTransport.AddURIToQueue` (and `Play` for `open`). Sonos talks to Spotify with the credentials you already linked.

Supported types: `track`, `album`, `playlist`, `show`, `episode`.

## Path 2 — Search via Sonos SMAPI

SMAPI is the Sonos Music API — the same protocol the Sonos app uses to browse and search music services (Spotify, Apple Music, TuneIn, …). No third-party app credentials are needed, but some services require local SMAPI auth before search or playback works.

```bash
sonos smapi services
sonos smapi categories  --service "Spotify"
sonos smapi search      --service "Spotify" --category tracks "miles davis"
sonos smapi browse      --service "Spotify" --id root
```

To search-and-play in one step:

```bash
sonos play spotify --name "Kitchen" --category albums "kind of blue"
```

For non-Spotify services, `--open` and `--enqueue` use Sonos queue metadata compatible with SoCo's generic music-service path. This is the path needed by AppLink services such as QQ Music and NetEase Cloud Music once the service is authenticated.

### One-time auth for some services

A few services (Spotify is a notable one) require a DeviceLink/AppLink handshake before SMAPI search works — even if the service is already playable in the Sonos app. Run:

```bash
sonos auth smapi begin    --service "Spotify"
# print/open the link URL, log in, then:
sonos auth smapi complete --service "Spotify" --wait 2m
```

Tokens are stored locally so you only do this once per machine.

## Path 3 — Spotify Web API (optional)

Want to search Spotify directly with full Spotify catalog filters? Set up a Spotify Web API app (client credentials) and use `sonos search spotify`:

```bash
export SPOTIFY_CLIENT_ID=…
export SPOTIFY_CLIENT_SECRET=…

sonos search spotify "miles davis" --type album --limit 5
sonos search spotify "miles davis" --type track --open --name "Kitchen"
```

You only need this if you specifically want Spotify-side search behavior; SMAPI is sufficient for most cases.

## Which one should I use?

| Goal                                                                    | Use this                |
| ----------------------------------------------------------------------- | ----------------------- |
| Already have a Spotify URI / link, just play it                         | `sonos open`            |
| Search "spotify in the Sonos app", play top result                      | `sonos play spotify`    |
| Inspect what services are linked / browse / debug                       | `sonos smapi …`         |
| Want raw Spotify catalog responses (markets, episodes, shows)           | `sonos search spotify`  |

In short: prefer `open` and `play spotify`. Drop down to `smapi` for browsing or to `search spotify` only when you specifically need the Spotify Web API.
