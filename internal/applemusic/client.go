package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// APIBaseURL is the Apple Music API endpoint
	APIBaseURL = "https://amp-api.music.apple.com"

	// DefaultStorefront is the default Apple Music storefront
	DefaultStorefront = "us"
)

// Client is an Apple Music API client.
type Client struct {
	Token      Token
	Storefront string
	HTTP       *http.Client
}

// NewClient creates a new Apple Music API client.
func NewClient(token Token) *Client {
	storefront := token.StorefrontID
	if storefront == "" {
		storefront = DefaultStorefront
	}

	return &Client{
		Token:      token,
		Storefront: storefront,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchResult represents the search response from Apple Music API.
type SearchResult struct {
	Results SearchResults `json:"results"`
}

// SearchResults contains categorized search results.
type SearchResults struct {
	Songs     *SongResults     `json:"songs,omitempty"`
	Albums    *AlbumResults    `json:"albums,omitempty"`
	Artists   *ArtistResults   `json:"artists,omitempty"`
	Playlists *PlaylistResults `json:"playlists,omitempty"`
}

// SongResults contains song search results.
type SongResults struct {
	Data []Song `json:"data"`
}

// AlbumResults contains album search results.
type AlbumResults struct {
	Data []Album `json:"data"`
}

// ArtistResults contains artist search results.
type ArtistResults struct {
	Data []Artist `json:"data"`
}

// PlaylistResults contains playlist search results.
type PlaylistResults struct {
	Data []Playlist `json:"data"`
}

// Song represents an Apple Music song.
type Song struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Href       string         `json:"href"`
	Attributes SongAttributes `json:"attributes"`
}

// SongAttributes contains song metadata.
type SongAttributes struct {
	Name        string   `json:"name"`
	AlbumName   string   `json:"albumName"`
	ArtistName  string   `json:"artistName"`
	DurationMs  int      `json:"durationInMillis"`
	TrackNumber int      `json:"trackNumber"`
	GenreNames  []string `json:"genreNames"`
	ISRC        string   `json:"isrc"`
	URL         string   `json:"url"`
	Artwork     *Artwork `json:"artwork,omitempty"`
}

// Album represents an Apple Music album.
type Album struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Href       string          `json:"href"`
	Attributes AlbumAttributes `json:"attributes"`
}

// AlbumAttributes contains album metadata.
type AlbumAttributes struct {
	Name       string   `json:"name"`
	ArtistName string   `json:"artistName"`
	TrackCount int      `json:"trackCount"`
	GenreNames []string `json:"genreNames"`
	URL        string   `json:"url"`
	Artwork    *Artwork `json:"artwork,omitempty"`
}

// Artist represents an Apple Music artist.
type Artist struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Href       string           `json:"href"`
	Attributes ArtistAttributes `json:"attributes"`
}

// ArtistAttributes contains artist metadata.
type ArtistAttributes struct {
	Name       string   `json:"name"`
	GenreNames []string `json:"genreNames"`
	URL        string   `json:"url"`
}

// Playlist represents an Apple Music playlist.
type Playlist struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Href       string             `json:"href"`
	Attributes PlaylistAttributes `json:"attributes"`
}

// PlaylistDescription can be a string or an object with short/standard fields.
type PlaylistDescription struct {
	Short    string `json:"short,omitempty"`
	Standard string `json:"standard,omitempty"`
}

// PlaylistAttributes contains playlist metadata.
type PlaylistAttributes struct {
	Name        string               `json:"name"`
	CuratorName string               `json:"curatorName"`
	Description *PlaylistDescription `json:"description,omitempty"`
	TrackCount  int                  `json:"trackCount,omitempty"`
	URL         string               `json:"url"`
	Artwork     *Artwork             `json:"artwork,omitempty"`
}

// Artwork represents artwork for an Apple Music resource.
type Artwork struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	URL        string `json:"url"`
	BgColor    string `json:"bgColor,omitempty"`
	TextColor1 string `json:"textColor1,omitempty"`
}

// SearchOptions configures a search request.
type SearchOptions struct {
	Types  []string // e.g., "songs", "albums", "artists", "playlists"
	Limit  int      // max results per type (1-25)
	Offset int      // pagination offset
}

// Search searches the Apple Music catalog.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	if !c.Token.IsValid() {
		return nil, fmt.Errorf("invalid or expired Apple Music token")
	}

	// Build search URL
	types := opts.Types
	if len(types) == 0 {
		types = []string{"songs", "albums", "playlists"}
	}

	limit := opts.Limit
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	params := url.Values{}
	params.Set("term", query)
	params.Set("types", strings.Join(types, ","))
	params.Set("limit", fmt.Sprintf("%d", limit))
	if opts.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}

	searchURL := fmt.Sprintf("%s/v1/catalog/%s/search?%s", APIBaseURL, c.Storefront, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	// Set required headers
	// DeveloperToken is the JWT for Bearer auth, MusicUserToken is the user session
	req.Header.Set("Authorization", "Bearer "+c.Token.DeveloperToken)
	req.Header.Set("Music-User-Token", c.Token.MusicUserToken)
	req.Header.Set("Origin", "https://music.apple.com")
	req.Header.Set("Referer", "https://music.apple.com/")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music api error: %s (status %d)", string(body), resp.StatusCode)
	}

	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	return &result, nil
}

// GetSong retrieves a song by ID.
func (c *Client) GetSong(ctx context.Context, id string) (*Song, error) {
	if !c.Token.IsValid() {
		return nil, fmt.Errorf("invalid or expired Apple Music token")
	}

	songURL := fmt.Sprintf("%s/v1/catalog/%s/songs/%s", APIBaseURL, c.Storefront, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, songURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token.DeveloperToken)
	req.Header.Set("Music-User-Token", c.Token.MusicUserToken)
	req.Header.Set("Origin", "https://music.apple.com")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music api error: %s (status %d)", string(body), resp.StatusCode)
	}

	var result struct {
		Data []Song `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse song response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("song not found: %s", id)
	}

	return &result.Data[0], nil
}
