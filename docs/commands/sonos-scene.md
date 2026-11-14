---
title: sonos scene
description: Save, list, apply, and delete scenes — named snapshots of grouping plus per-room volume / mute.
---

# `sonos scene`

A scene captures the current grouping (coordinators + members) and per-room volume / mute, and can be re-applied later.

```
sonos scene save   <name>
sonos scene list
sonos scene apply  <name> [--only "<Room>"]
sonos scene delete <name>
```

## `sonos scene save`

```bash
sonos scene save evening
sonos scene save weekday-morning
```

Walks the live topology and writes a JSON snapshot under your config directory.

## `sonos scene list`

```bash
sonos scene list
sonos scene list --format json
```

## `sonos scene apply`

```bash
sonos scene apply evening
sonos scene apply evening --only "Kitchen"   # experimental
```

| Flag | What it does |
| --- | --- |
| `--only string` | Restore only the saved state for one room (experimental). |

Apply is idempotent. Speakers absent from the current topology are skipped; a failed command to a visible speaker stops the operation with an error. Completed changes are not rolled back.

## `sonos scene delete`

```bash
sonos scene delete evening
```

## See also

- [Scenes](../scenes.md) — what's captured, what isn't, and how apply works under the hood.

Save preserves the existing file when a volume or mute read fails. Save and apply accept `--ip <speaker-ip>` to bypass discovery for the topology query.
