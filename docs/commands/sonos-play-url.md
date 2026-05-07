---
title: sonos play-url
description: Play YouTube, podcast, radio, and other URLs through a short-lived Sonos-safe proxy.
---

# `sonos play-url`

Starts a local daemon that turns a URL into a Sonos-safe MP3 stream, points the target room at it, and lets the daemon exit when playback ends or goes idle.

## Synopsis

```
sonos play-url <url> --name "<Room>" [flags]
```

## What It Accepts

- YouTube and other `yt-dlp` supported pages (single track).
- YouTube / YouTube Music playlist pages (e.g. `https://music.youtube.com/playlist?list=…`) — auto-detected and every track is enqueued.
- Other `yt-dlp` playlist pages when forced with `--playlist`, provided `yt-dlp` reports usable item URLs.
- Direct podcast/audio URLs such as `.mp3`, `.m4a`, `.aac`, `.flac`, `.m3u8`.
- Radio streams and other URLs that `ffmpeg` can read.

`play-uri` sends an exact URI to Sonos. `play-url` is the smarter compatibility path.

## Playlist Mode

A URL is treated as a playlist when it points to an unambiguous YouTube / YouTube Music playlist page (`?list=…` with no video id). In that case `play-url`:

1. Enumerates every item with `yt-dlp --flat-playlist`.
2. Starts a single local proxy that exposes `/track-001.mp3`, `/track-002.mp3`, … each backed by its own `yt-dlp -o - | ffmpeg → MP3` pipeline.
3. Clears the queue, calls `AddURIToQueue` once per track (with DIDL metadata derived from the title and reported duration), then plays from track 1.

Use `--playlist` to force playlist mode on an ambiguous watch+playlist URL (`?v=…&list=…`) or another `yt-dlp` playlist page. Use `--no-playlist` to force single-track mode. `--playlist-limit N` caps the number of items enqueued.

## Flags

| Flag | What it does |
| --- | --- |
| `--resolver auto|direct|yt-dlp` | Pick URL resolution strategy. Default: `auto`. |
| `--yt-dlp string` | Path to `yt-dlp`. |
| `--ffmpeg string` | Path to `ffmpeg`. |
| `--media-format string` | `yt-dlp` format selector. Defaults to AAC/M4A first. |
| `--title string` | Override stream title. |
| `--provider string` | Override source/provider label. |
| `--bitrate string` | MP3 proxy bitrate. Default: `192k`. |
| `--port int` | Local proxy port. Default: random free port. |
| `--playlist` | Force playlist mode (enumerate every track and enqueue). |
| `--no-playlist` | Force single-track mode for playlist URLs. |
| `--playlist-limit int` | Maximum number of items to enqueue in playlist mode. `0` (default) means no limit. |

## Examples

```bash
sonos play-url --name "Office" "https://www.youtube.com/watch?v=-n_rdQIVahw"
sonos play-url --name "Office" "https://example.com/podcast/episode.mp3"
sonos play-url --name "Office" --resolver yt-dlp "https://soundcloud.com/example/track"
```

## Metadata

The daemon serves the stream as `Sonos CLI` and sends ICY metadata with the resolved media title and provider. This usually gives a cleaner Sonos app display than handing Sonos a temporary `googlevideo.com` URL directly.

## Lifecycle

The command launches the proxy in the background, starts playback, then returns. The proxy exits when the source naturally ends or after it has been idle for a short grace period.
