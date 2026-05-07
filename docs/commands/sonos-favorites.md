---
title: sonos favorites
description: List Sonos Favorites and play one by index or title.
---

# `sonos favorites`

Lists and plays Sonos Favorites — the `FV:2` container in `ContentDirectory`.

```
sonos favorites list [--start N] [--limit N]
sonos favorites open --name "<Room>" [<title>] [--index <n>]
```

## `sonos favorites list`

```bash
sonos favorites list
sonos favorites list --limit 10
sonos favorites list --format json | jq -r '.[].title'
```

| Flag | Default | What it does |
| --- | --- | --- |
| `--start int` | `0` | Starting index (0-based). |
| `--limit int` | `50` | Max results. |

## `sonos favorites open`

Plays a favorite by exact title (case-insensitive) or by 1-based index from `sonos favorites list`.

```bash
sonos favorites open --name "Kitchen" "Morning Coffee"
sonos favorites open --name "Kitchen" --index 3
```

## How it works

- `list` calls `ContentDirectory.Browse(ObjectID=FV:2)` and parses the DIDL-Lite favorites.
- `open` resolves the favorite. Stream and track favorites are sent directly with `AVTransport.SetAVTransportURI`, then `Play`.
- Container favorites such as service-side albums and playlists use the Sonos queue path: clear the queue, enqueue the container with `AddURIToQueue`, switch playback to the queue, seek to the first enqueued track, then `Play`.
