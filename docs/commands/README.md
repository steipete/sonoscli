---
title: Command Reference
description: Every sonos subcommand at a glance, with one page per command.
---

# Command Reference

The full surface of `sonos`. Every page documents the command, its flags, examples, and what it talks to on the speaker.

Global flags apply to every command:

| Flag | Default | What it does |
| --- | --- | --- |
| `--name string` | — | Target speaker by name (matched against topology). |
| `--ip string` | — | Target speaker by IP — skips discovery. |
| `--format string` | `plain` | Output format: `plain`, `json`, or `tsv`. |
| `--timeout duration` | `5s` | Discovery and per-call timeout. |
| `--debug` | off | Print SOAP traces to stderr. |

## Discovery & status

- [`sonos discover`](sonos-discover.md)
- [`sonos status`](sonos-status.md) (alias: `now`)
- [`sonos watch`](sonos-watch.md)

## Playback

- [`sonos play`](sonos-play.md)
- [`sonos pause`](sonos-pause.md)
- [`sonos stop`](sonos-stop.md)
- [`sonos next`](sonos-next.md)
- [`sonos prev`](sonos-prev.md)
- [`sonos play-url`](sonos-play-url.md)
- [`sonos play-uri`](sonos-play-uri.md)
- [`sonos play youtube`](sonos-play-youtube.md)
- [`sonos linein`](sonos-linein.md)
- [`sonos tv`](sonos-tv.md)

## Volume & mute

- [`sonos volume`](sonos-volume.md)
- [`sonos mute`](sonos-mute.md)

## Grouping

- [`sonos group`](sonos-group.md)

## Queue

- [`sonos queue`](sonos-queue.md)

## Favorites & scenes

- [`sonos favorites`](sonos-favorites.md)
- [`sonos scene`](sonos-scene.md)

## Spotify & SMAPI

- [`sonos open`](sonos-open.md)
- [`sonos enqueue`](sonos-enqueue.md)
- [`sonos play spotify`](sonos-play-spotify.md)
- [`sonos search spotify`](sonos-search-spotify.md)
- [`sonos smapi`](sonos-smapi.md)
- [`sonos auth smapi`](sonos-auth-smapi.md)

## Local config

- [`sonos config`](sonos-config.md)
