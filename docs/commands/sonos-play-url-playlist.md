---
title: sonos play-url-playlist
description: Play a YouTube/yt-dlp playlist by enqueueing every track through the local stream proxy.
---

# `sonos play-url-playlist`

Enumerates a yt-dlp-supported playlist URL, starts a single local stream proxy that exposes one HTTP path per track, then replaces the speaker's queue with the resolved track URLs and plays from the first track.

## Synopsis

```
sonos play-url-playlist <url> --name "<Room>" [flags]
```

## What It Accepts

- YouTube and YouTube Music playlist URLs (`?list=...`).
- Any other `yt-dlp`-supported playlist page that exposes one item per `--flat-playlist` line.

For single-track playback see [`sonos play-url`](sonos-play-url.md).

## Flags

| Flag | What it does |
| --- | --- |
| `--yt-dlp string` | Path to `yt-dlp`. |
| `--ffmpeg string` | Path to `ffmpeg`. |
| `--media-format string` | `yt-dlp` format selector applied to every track. Defaults to AAC/M4A first. |
| `--bitrate string` | MP3 proxy bitrate. Default: `192k`. |
| `--port int` | Local proxy port. Default: random free port. |
| `--limit int` | Maximum number of playlist items to enqueue. `0` (default) means no limit. |

## How It Works

1. `yt-dlp --flat-playlist` enumerates the playlist; each line is `<id>\t<title>`.
2. The CLI starts one local stream proxy daemon. Each track gets its own HTTP path (`/track-001.mp3`, `/track-002.mp3`, …) backed by a separate `yt-dlp -o - … | ffmpeg → MP3` pipeline that runs lazily when Sonos requests that track.
3. The CLI clears the speaker's queue, calls `AddURIToQueue` once per track with the proxy URL and DIDL metadata derived from the track title, then binds the AVTransport to the queue and plays from track 1.
4. The daemon stays alive across track transitions (idle timeout is bumped to 60 s for playlist mode) and shuts down once the queue stops fetching.

## Examples

```bash
sonos play-url-playlist --name "Office" "https://music.youtube.com/playlist?list=PLbHH0K6cM5avw4u_whcICc_v9iawvxze7"
sonos play-url-playlist --name "Office" --limit 10 "https://www.youtube.com/playlist?list=PLxxxxxxxxxxxxxxxx"
```

## Caveats

- Each track is resolved fresh by `yt-dlp` at playback time — long playlists work but the daemon must keep running for the entire session, so don't background it on a flaky network.
- The whole queue is replaced (matches the Sonos app's behaviour for "play this playlist now"). Use `sonos queue list` afterwards to inspect.
- Playlist URLs that yt-dlp can't enumerate (e.g. private playlists requiring auth) return an error from `yt-dlp` itself.
