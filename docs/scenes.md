---
title: Scenes
description: Capture grouping plus per-room volume and mute as a named scene, then re-apply later.
---

# Scenes

A **scene** is a named snapshot of your Sonos system: which speakers are grouped, who is the coordinator of each group, and what volume / mute each room is at. Save one when the house feels right, restore it on demand.

## Save what you have right now

```bash
sonos scene save evening
```

This walks the live topology, snapshots every visible zone (and its members, volume, and mute), and stores the scene under your config directory.

## List, apply, delete

```bash
sonos scene list
sonos scene apply evening
sonos scene delete evening
```

Apply reports an error when ungrouping, joining, mute, or volume restore fails. Running `sonos scene apply evening` again is safe when the speakers accept the calls.

## Targeting one room (experimental)

```bash
sonos scene apply evening --only "Kitchen"
```

Only restores the parts of the scene that involve the named room. Useful when you don't want to disturb other zones.

## What's captured (and what isn't)

Captured:

- Grouping (coordinator + members) per zone
- Per-room volume (0–100)
- Per-room mute state

**Not** captured:

- Currently playing track or queue contents
- EQ / loudness / surround levels
- Bonded satellite topology (preserved but not edited)

If you want a scene that also re-loads a queue, save the queue separately (e.g. capture a Spotify URI) and chain commands in a shell script:

```bash
sonos scene apply evening
sonos open --name "Living Room" spotify:playlist:37i9dQZF1DXcBWIGoYBM5M
```

## How apply works

1. Read current topology.
2. Compute the diff against the saved scene.
3. Dissolve groups that conflict.
4. Re-create the saved groups (each member joins the saved coordinator).
5. Re-apply per-room volume and mute.

If a speaker named in the scene has gone offline, the corresponding step is logged and skipped — the rest of the scene still applies.

## File layout

Scenes live alongside your config:

```
~/.config/sonoscli/scenes/<name>.json
```

The format is intentionally readable and stable; you can hand-edit a scene file if you really want to.
