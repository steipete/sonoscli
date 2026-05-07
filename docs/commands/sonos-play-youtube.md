---
title: sonos play youtube
description: Resolve a YouTube URL with yt-dlp and play the direct audio stream on Sonos.
---

# `sonos play youtube`

Uses `yt-dlp` to resolve a YouTube URL to a temporary direct audio URL, sets that URL on the group coordinator, and starts playback.

For day-to-day playback, prefer [`sonos play-url`](sonos-play-url.md). It runs a short-lived local proxy, transcodes to a Sonos-safe MP3 stream, and provides cleaner `Sonos CLI` stream metadata in the Sonos app.

## Synopsis

```
sonos play youtube <url> --name "<Room>" [--yt-dlp <path>] [--media-format <selector>] [--title "<title>"] [--radio]
```

## Requirements

- `yt-dlp` must be installed and available on `PATH`, or passed with `--yt-dlp`.
- The Sonos speaker must be able to fetch the resolved `googlevideo.com` URL directly.
- Resolved YouTube URLs expire; rerun the command if playback starts failing later.

## Flags

| Flag | What it does |
| --- | --- |
| `--yt-dlp string` | Path to `yt-dlp` (default: `yt-dlp`). |
| `--media-format string` | yt-dlp media selector. Defaults to an AAC/M4A-first selector that Sonos is likely to play. |
| `--title string` | Override the display title (defaults to the YouTube title). |
| `--radio` | Force radio-style playback for the resolved stream. |

## Examples

```bash
sonos play youtube --name "Kitchen" "https://www.youtube.com/watch?v=-n_rdQIVahw"
```

For a URL that includes a playlist or YouTube radio queue, `sonoscli` plays only the selected/current video:

```bash
sonos play youtube --name "Kitchen" "https://www.youtube.com/watch?v=-n_rdQIVahw&list=RD-n_rdQIVahw&start_radio=1"
```

If Sonos rejects normal track-style playback, retry as a radio stream:

```bash
sonos play youtube --name "Kitchen" --radio "https://www.youtube.com/watch?v=-n_rdQIVahw"
```

## How it works

The command runs `yt-dlp --no-playlist -f <selector> -j <url>`, reads the selected format URL and title from JSON, then calls `AVTransport.SetAVTransportURI` followed by `AVTransport.Play`.
