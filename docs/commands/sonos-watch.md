---
title: sonos watch
description: Subscribe to live AVTransport and RenderingControl events from a speaker and stream changes.
---

# `sonos watch`

Initial notifications are queued during subscription setup and labeled with their service once the subscriptions are established.

Subscribes to UPnP eventing on the target speaker and prints state changes as they arrive. Ctrl+C to stop. Useful for debugging, dashboards, or piping into another script.

## Synopsis

```
sonos watch --name "<Room>" [--duration 30s] [--format plain|json]
```

## Flags

| Flag | Default | What it does |
| --- | --- | --- |
| `--duration duration` | `0` | Stop after this duration. `0` = until Ctrl+C. |

Plus all [global flags](README.md).

## Examples

```bash
sonos watch --name "Kitchen"
sonos watch --name "Kitchen" --duration 30s
sonos watch --name "Kitchen" --format json | jq -r '.event'
```

## How it works

1. Spins up a local HTTP server (the *callback*).
2. Sends `SUBSCRIBE` requests to `AVTransport/Event` and `RenderingControl/Event` on the speaker.
3. Renders each `NOTIFY` body as a structured event line.
4. On exit, sends `UNSUBSCRIBE` so the speaker stops calling back.

## Firewall

The speaker has to be able to reach **your machine** on the chosen callback port. On macOS / Windows you'll likely see a firewall prompt the first time. Allow it, or limit the rule to your Sonos VLAN.

If your network blocks inbound from the Sonos subnet, eventing won't work — fall back to polling [`sonos status`](sonos-status.md).
