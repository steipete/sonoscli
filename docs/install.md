---
title: Install
description: Install sonoscli via Homebrew, go install, prebuilt release archives, or from source.
---

# Install

`sonoscli` ships a single binary called `sonos`. Pick whichever install option suits your machine.

## Homebrew (recommended on macOS / Linux)

```bash
brew install steipete/tap/sonoscli
```

Upgrade later:

```bash
brew upgrade steipete/tap/sonoscli
```

## go install

If you already have a Go toolchain (Go 1.26+):

```bash
go install github.com/steipete/sonoscli/cmd/sonos@latest
sonos --version
```

The binary lands in `$(go env GOBIN)` (defaults to `$HOME/go/bin`) — make sure that's on your `PATH`.

## Prebuilt release archives

Each tagged release publishes archives for macOS (universal, plus Intel and Apple Silicon compatibility archives), Linux (amd64 + arm64), and Windows (amd64 + arm64) on the [GitHub releases page](https://github.com/steipete/sonoscli/releases). Download, extract, drop the `sonos` binary somewhere on your `PATH`.

## From source

Use this when you want the current `main` branch before the next release:

```bash
git clone https://github.com/steipete/sonoscli
cd sonoscli
make build
./bin/sonos --version
```

`sonos play-url` needs `ffmpeg`, and uses `yt-dlp` for YouTube, YouTube Music playlists, SoundCloud-style pages, and other media pages:

```bash
brew install ffmpeg yt-dlp
```

## Network requirements

- Your machine must be on the same network as your Sonos system.
- Speakers must be reachable on TCP port `1400` (e.g. `http://<speaker-ip>:1400/`). Test with `curl -s http://<speaker-ip>:1400/xml/device_description.xml | head`.
- Multicast / SSDP (`239.255.255.250:1900`) helps discovery but is not required — `sonoscli` falls back to a subnet scan.
- For [`sonos watch`](commands/sonos-watch.md), Sonos has to reach *your* machine on a callback port; your firewall may prompt the first time.

## Shell completion

Cobra-generated completion is available for bash, zsh, fish, and PowerShell:

```bash
sonos completion zsh > "${fpath[1]}/_sonos"        # zsh
sonos completion bash > /usr/local/etc/bash_completion.d/sonos
sonos completion fish > ~/.config/fish/completions/sonos.fish
```

The CLI also caches discovered speaker names so `--name <Tab>` completes against your real rooms.
