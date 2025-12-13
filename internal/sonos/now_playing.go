package sonos

import "strings"

// AlbumArtURL returns an absolute URL for album art when Sonos returns a relative path.
func AlbumArtURL(deviceIP string, albumArtURI string) string {
	albumArtURI = strings.TrimSpace(albumArtURI)
	if albumArtURI == "" {
		return ""
	}
	if strings.HasPrefix(albumArtURI, "http://") || strings.HasPrefix(albumArtURI, "https://") {
		return albumArtURI
	}
	if strings.HasPrefix(albumArtURI, "/") && strings.TrimSpace(deviceIP) != "" {
		return "http://" + deviceIP + ":1400" + albumArtURI
	}
	return albumArtURI
}

// ParseNowPlaying attempts to parse a DIDL-Lite TrackMetaData payload into a single DIDLItem.
func ParseNowPlaying(trackMetaData string) (DIDLItem, bool) {
	items, err := ParseDIDLItems(trackMetaData)
	if err != nil || len(items) == 0 {
		return DIDLItem{}, false
	}
	return items[0], true
}
