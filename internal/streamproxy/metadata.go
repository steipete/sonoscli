package streamproxy

import (
	"net/url"
	"path"
	"strings"
)

const DefaultFormat = "bestaudio[ext=m4a][acodec!=none]/bestaudio[acodec^=mp4a]/bestaudio[acodec!=none]/bestaudio"

type Source struct {
	URL       string `json:"url"`
	InputURL  string `json:"inputUrl,omitempty"`
	Title     string `json:"title,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	UseYTDLP  bool   `json:"useYTDLP,omitempty"`
	FormatID  string `json:"formatId,omitempty"`
	Ext       string `json:"ext,omitempty"`
	ACodec    string `json:"acodec,omitempty"`
	// DurationSeconds is the known finite duration of the source, in seconds.
	// When > 0 the proxy treats the response as a finite track (no icy-*
	// headers) so Sonos schedules the next-queue-entry advance instead of
	// playing it like a radio station.
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
}

// IsFiniteTrack reports whether this source has a known finite duration and
// should therefore be served as a regular MP3 file rather than as an
// icy-tagged radio stream.
func (s Source) IsFiniteTrack() bool {
	return s.DurationSeconds > 0
}

func (s Source) DisplayTitle() string {
	if strings.TrimSpace(s.Title) != "" {
		return strings.TrimSpace(s.Title)
	}
	if strings.TrimSpace(s.URL) != "" {
		if u, err := url.Parse(s.URL); err == nil {
			base := path.Base(u.Path)
			if base != "." && base != "/" && base != "" {
				return base
			}
			if u.Host != "" {
				return u.Host
			}
		}
	}
	return "Stream"
}

func (s Source) DisplayProvider() string {
	if strings.TrimSpace(s.Provider) != "" {
		return strings.TrimSpace(s.Provider)
	}
	if strings.TrimSpace(s.URL) != "" {
		if u, err := url.Parse(s.URL); err == nil && u.Host != "" {
			return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		}
	}
	return "URL"
}

func LooksLikeDirectMedia(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(u.Path))
	switch ext {
	case ".aac", ".aif", ".aiff", ".flac", ".m3u", ".m3u8", ".m4a", ".mp3", ".mp4", ".ogg", ".opus", ".pls", ".wav", ".wma":
		return true
	default:
		return false
	}
}

func LooksLikeYouTube(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	return host == "youtube.com" || host == "music.youtube.com" || host == "youtu.be" || strings.HasSuffix(host, ".youtube.com")
}
