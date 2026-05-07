---
title: Quickstart
description: A 5-minute tour of sonoscli — discover speakers, control playback, group rooms, save a scene, and play a Spotify link.
---

# Quickstart

This walks through the moves you'll use every day. Replace `Kitchen` / `Living Room` with your actual room names.

## 1. Discover speakers

```bash
sonos discover
```

You should see one row per visible Sonos zone, with name, model, and IP. JSON works too:

```bash
sonos discover --format json
```

The first run also caches speaker names so `--name <Tab>` autocompletes.

## 2. Check what's playing

```bash
sonos status --name "Kitchen"
sonos now --name "Kitchen"           # alias
sonos status --name "Kitchen" --format json
```

`status` reports on the **group coordinator** — even if you ask a satellite, you get the truthful playback state.

## 3. Control playback

```bash
sonos play  --name "Kitchen"
sonos pause --name "Kitchen"
sonos next  --name "Kitchen"
sonos prev  --name "Kitchen"
sonos volume set --name "Kitchen" 25
sonos mute toggle --name "Kitchen"
```

## 4. Open a Spotify link without credentials

If Spotify is already linked in the Sonos app, you don't need any Spotify Web API setup:

```bash
sonos open --name "Kitchen" "https://open.spotify.com/track/6NmXV4o6bmp704aPGyTVVG"
sonos open --name "Kitchen" spotify:album:0nrRP2bk19rLc0orkWPQk2
sonos enqueue --name "Kitchen" spotify:playlist:37i9dQZF1DXcBWIGoYBM5M --next
```

## 5. Play a URL

For YouTube, podcast links, radio streams, and odd formats, use the local proxy path. It resolves common media pages with `yt-dlp`, transcodes with `ffmpeg`, and exits when playback ends or goes idle:

```bash
sonos play-url --name "Kitchen" "https://www.youtube.com/watch?v=-n_rdQIVahw"
sonos play-url --name "Kitchen" "https://example.com/podcast/episode.mp3"
```

## 6. Group rooms

```bash
sonos group status
sonos group join   --name "Kitchen" --to "Living Room"
sonos group party  --to "Living Room"
sonos group volume set --name "Living Room" 18
sonos group unjoin --name "Kitchen"
sonos group dissolve --name "Living Room"
```

## 7. Save and apply scenes

A scene captures grouping plus per-room volume/mute. Save what's good now, restore it later.

```bash
sonos scene save evening
sonos scene list
sonos scene apply evening
```

## 8. Watch live events

```bash
sonos watch --name "Kitchen"
```

Subscribes to AVTransport + RenderingControl and prints state changes as they arrive (Ctrl+C to stop).

## 9. Search via Sonos (no Spotify credentials)

```bash
sonos smapi services
sonos smapi search --service "Spotify" --category tracks "miles davis kind of blue"
sonos play spotify --name "Kitchen" --category albums "kind of blue"
```

If a service needs a one-time link first:

```bash
sonos auth smapi begin    --service "Spotify"
sonos auth smapi complete --service "Spotify" --wait 2m
```

## Where to go next

- The full [Command Reference](commands/) lists every flag.
- [Spotify & SMAPI](spotify-and-smapi.md) explains the two search paths.
- [Architecture](architecture.md) covers the moving parts inside the binary.
