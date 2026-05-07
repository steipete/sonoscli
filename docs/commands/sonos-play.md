---
title: sonos play
description: Resume playback on a room, or search-and-play via Spotify.
---

# `sonos play`

Sends `AVTransport.Play` to the group coordinator. Has one subcommand for search-and-play through Sonos.

## Synopsis

```
sonos play --name "<Room>"
sonos play-url --name "<Room>" <url> [flags]
sonos play spotify --name "<Room>" <query> [flags]
sonos play youtube --name "<Room>" <url> [flags]
```

## Examples

```bash
sonos play --name "Kitchen"
sonos play-url --name "Kitchen" "https://www.youtube.com/watch?v=-n_rdQIVahw"
sonos play spotify --name "Kitchen" --category albums "kind of blue"
sonos play youtube --name "Kitchen" "https://www.youtube.com/watch?v=-n_rdQIVahw"
```

## Subcommands

- [`sonos play-url`](sonos-play-url.md) — stream YouTube, podcasts, radio, and other URLs through a Sonos-safe local proxy.
- [`sonos play spotify`](sonos-play-spotify.md) — search Spotify via SMAPI and play the top result.
- [`sonos play youtube`](sonos-play-youtube.md) — resolve a YouTube URL with `yt-dlp` and play the direct audio stream.

## How it works

`sonos play` resolves the target's coordinator and calls `AVTransport.Play(InstanceID=0, Speed=1)`. There must already be something on the queue / current URI; if not, the speaker stays silent.

To start a fresh source, use [`sonos open`](sonos-open.md) (Spotify URI), [`sonos play-uri`](sonos-play-uri.md) (any URI), or [`sonos favorites open`](sonos-favorites.md).
