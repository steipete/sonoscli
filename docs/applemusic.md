# Apple Music Integration

This document describes the Apple Music integration in `sonoscli`, enabling search and playback of Apple Music content on Sonos speakers.

## Overview

Apple Music integration allows you to:
- Search the Apple Music catalog (songs, albums, playlists, artists)
- Play Apple Music content directly on your Sonos speakers
- Use Apple Music URLs for direct playback

**Prerequisites:**
- An active Apple Music subscription
- Apple Music linked to your Sonos system via the Sonos app
- Authentication tokens from the Apple Music web player

## Authentication

Apple Music uses a two-token authentication system obtained from the web player:

| Token | Description |
|-------|-------------|
| Developer Token | JWT for API access (starts with `eyJ...`) |
| Music User Token | User session token (starts with `Av...`) |

Tokens are stored locally at `~/.config/sonoscli/applemusic_token.json` and typically last ~6 months.

### Authenticate

```bash
# Interactive login (opens browser)
sonos auth applemusic login

# Direct token entry
sonos auth applemusic login \
  --developer-token "eyJ..." \
  --user-token "Av..." \
  --storefront us
```

**To extract tokens manually:**
1. Sign in at https://music.apple.com
2. Open Developer Tools (F12 or Cmd+Option+I)
3. In the Console tab, run:
   ```javascript
   MusicKit.getInstance().developerToken
   MusicKit.getInstance().musicUserToken
   ```
4. Copy both token values

### Check Status

```bash
sonos auth applemusic status
```

Output shows authentication state, storefront, creation date, and expiry.

### Logout

```bash
sonos auth applemusic logout
```

## Search

Search the Apple Music catalog using authenticated API access.

```bash
# Search for songs (default)
sonos search applemusic "flying lotus"

# Search specific categories
sonos search applemusic "chill vibes" --category playlists
sonos search applemusic "miles davis" --category albums
sonos search applemusic "kendrick lamar" --category artists

# Limit results
sonos search applemusic "jazz" --limit 5

# JSON output
sonos search applemusic "jazz" --format json
```

**Categories:** `songs` (default), `albums`, `playlists`, `artists`

**Output columns:** INDEX, TYPE, TITLE, ARTIST, ID

## Playback

Search and play Apple Music content on your Sonos speakers.

```bash
# Search and play the top result
sonos play applemusic "taylor swift" --name "Living Room"

# Play a specific category
sonos play applemusic --category albums "abbey road" --name "Kitchen"
sonos play applemusic --category playlists "workout" --name "Office"

# Play a specific result by index (0-based)
sonos play applemusic "jazz" --index 2 --name "Bedroom"

# Enqueue without starting playback
sonos play applemusic "miles davis" --enqueue --name "Living Room"

# Override the queue item title
sonos play applemusic "kind of blue" --title "My Jazz" --name "Office"
```

### Apple Music URLs

You can also play Apple Music content using URLs:

```bash
# Album
sonos open --name "Living Room" "https://music.apple.com/us/album/album-name/1234567890"

# Track within album (uses ?i= parameter)
sonos open --name "Living Room" "https://music.apple.com/us/album/album-name/1234567890?i=9876543210"

# Playlist
sonos open --name "Living Room" "https://music.apple.com/us/playlist/playlist-name/pl.u-xyz"

# Station
sonos open --name "Living Room" "https://music.apple.com/us/station/station-name/ra.xyz"
```

## Architecture

### Package Structure

```
internal/applemusic/     # Apple Music API client and auth
  client.go              # API client with search functionality
  auth.go                # Browser-based auth flow
  token_store.go         # Token persistence (~/.config/sonoscli/)

internal/sonos/
  applemusic.go          # URL parsing, Sonos enqueue logic

internal/cli/
  auth_applemusic.go     # Auth CLI commands
  search_applemusic.go   # Search CLI command
  play_applemusic.go     # Play CLI command
```

### API Integration

The integration uses the Apple Music API at `https://amp-api.music.apple.com`:

- **Search endpoint:** `/v1/catalog/{storefront}/search`
- **Authentication:** Bearer token + Music-User-Token header
- **Storefronts:** Region codes like `us`, `gb`, `jp` (default: `us`)

### Sonos Integration

Apple Music content is enqueued via Sonos SMAPI:
- **Service number:** 204 (Apple Music service ID in Sonos)
- **URI format:** `x-sonos-http:song%3a{ID}.mp4?sid=204&flags=8224&sn=10`
- **DIDL metadata:** Standard UPnP DIDL-Lite format for queue items

Content types use specific DIDL classes:
| Type | DIDL Class |
|------|------------|
| Song | `object.item.audioItem.musicTrack` |
| Album | `object.container.album.musicAlbum` |
| Playlist | `object.container.playlistContainer` |
| Station | `object.item.audioItem.audioBroadcast` |

## Comparison with Spotify

| Feature | Apple Music | Spotify |
|---------|-------------|---------|
| Credentials required | Yes (browser tokens) | Optional (Web API creds) |
| Search via CLI | Yes (`search applemusic`) | Yes (`search spotify`) |
| Play via CLI | Yes (`play applemusic`) | Yes (`open`/`enqueue`) |
| URL playback | Yes | Yes |
| Token storage | `applemusic_token.json` | Environment variables |
| Token lifespan | ~6 months | Session-based |
| SMAPI fallback | No | Yes (`smapi search`) |

## Troubleshooting

### "not authenticated with Apple Music"

Run `sonos auth applemusic login` to authenticate.

### "Apple Music token expired"

Tokens last ~6 months. Re-authenticate with `sonos auth applemusic login`.

### "apple music api error: status 401"

Token is invalid or expired. Re-authenticate.

### "apple music api error: status 403"

The storefront may not match your subscription region. Try:
```bash
sonos auth applemusic login --storefront gb  # or your region
```

### Playback doesn't start

Ensure Apple Music is linked to your Sonos system via the Sonos app. The service must be configured before `sonoscli` can enqueue content.

### Search returns no results

- Check your query spelling
- Try a different category (`--category albums` vs `--category songs`)
- Verify your storefront matches the content's availability region

## Testing

Manual test plan for Apple Music integration:

```bash
# 1. Authentication
sonos auth applemusic status                    # Check current state
sonos auth applemusic login                     # Authenticate
sonos auth applemusic status                    # Verify authenticated

# 2. Search
sonos search applemusic "test query"
sonos search applemusic "test" --category albums
sonos search applemusic "test" --format json

# 3. Playback
sonos play applemusic "taylor swift" --name "<room>"
sonos play applemusic --category albums "1989" --name "<room>"
sonos queue list --name "<room>"                # Verify item in queue

# 4. Cleanup
sonos auth applemusic logout
sonos auth applemusic status                    # Verify logged out
```
