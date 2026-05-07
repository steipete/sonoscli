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
