package spotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearch_Tracks_ClientCredentials(t *testing.T) {
	t.Parallel()

	var tokenCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			atomic.AddInt32(&tokenCalls, 1)
			auth := r.Header.Get("Authorization")
			want := "Basic " + base64.StdEncoding.EncodeToString([]byte("id:secret"))
			if auth != want {
				t.Fatalf("unexpected auth header: got %q want %q", auth, want)
			}
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
				t.Fatalf("unexpected content-type: %q", ct)
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Fatalf("unexpected grant_type: %q", r.Form.Get("grant_type"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/v1/search":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Fatalf("unexpected bearer token: %q", r.Header.Get("Authorization"))
			}
			q, _ := url.ParseQuery(r.URL.RawQuery)
			if q.Get("type") != "track" {
				t.Fatalf("unexpected type: %q", q.Get("type"))
			}
			if q.Get("q") != "hello world" {
				t.Fatalf("unexpected q: %q", q.Get("q"))
			}
			if q.Get("limit") != "2" {
				t.Fatalf("unexpected limit: %q", q.Get("limit"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tracks": map[string]any{
					"items": []any{
						map[string]any{
							"id":   "t1",
							"name": "Song 1",
							"uri":  "spotify:track:t1",
							"external_urls": map[string]any{
								"spotify": "https://open.spotify.com/track/t1",
							},
							"artists": []any{
								map[string]any{"name": "Artist A"},
							},
							"album": map[string]any{"name": "Album X"},
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("id", "secret", &http.Client{Timeout: 2 * time.Second})
	c.AccountsBaseURL = srv.URL
	c.APIBaseURL = srv.URL

	ctx := context.Background()
	results, err := c.Search(ctx, "hello world", TypeTrack, 2, "")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("unexpected results length: %d", len(results))
	}
	if results[0].URI != "spotify:track:t1" || results[0].ID != "t1" || results[0].Title != "Song 1" {
		t.Fatalf("unexpected result: %+v", results[0])
	}
	if results[0].Subtitle != "Artist A — Album X" {
		t.Fatalf("unexpected subtitle: %q", results[0].Subtitle)
	}

	// Second search should reuse cached token.
	_, err = c.Search(ctx, "hello world", TypeTrack, 2, "")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("expected 1 token call, got %d", got)
	}
}

func TestParseSearchType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want SearchType
	}{
		{"track", TypeTrack},
		{" Album ", TypeAlbum},
		{"PLAYLIST", TypePlaylist},
		{"show", TypeShow},
		{"episode", TypeEpisode},
	}
	for _, tt := range tests {
		got, err := ParseSearchType(tt.in)
		if err != nil {
			t.Fatalf("ParseSearchType(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ParseSearchType(%q): got %q want %q", tt.in, got, tt.want)
		}
	}

	if _, err := ParseSearchType("nope"); err == nil {
		t.Fatalf("expected error for invalid type")
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		t.Setenv("SPOTIFY_CLIENT_ID", "")
		t.Setenv("SPOTIFY_CLIENT_SECRET", "")
		_, err := NewFromEnv(nil)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("present", func(t *testing.T) {
		t.Setenv("SPOTIFY_CLIENT_ID", "id")
		t.Setenv("SPOTIFY_CLIENT_SECRET", "secret")
		hc := &http.Client{Timeout: 123 * time.Millisecond}

		c, err := NewFromEnv(hc)
		if err != nil {
			t.Fatalf("NewFromEnv: %v", err)
		}
		if c.ClientID != "id" || c.ClientSecret != "secret" {
			t.Fatalf("unexpected client creds: %+v", c)
		}
		if c.HTTP != hc {
			t.Fatalf("expected provided http client to be used")
		}
	})
}

func TestSearch_ParsesAllTypes(t *testing.T) {
	t.Parallel()

	var tokenCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/v1/search":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Fatalf("unexpected bearer token: %q", r.Header.Get("Authorization"))
			}
			q, _ := url.ParseQuery(r.URL.RawQuery)
			typ := q.Get("type")
			switch typ {
			case "album":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"albums": map[string]any{
						"items": []any{
							map[string]any{
								"id":   "a1",
								"name": "Album 1",
								"uri":  "spotify:album:a1",
								"external_urls": map[string]any{
									"spotify": "https://open.spotify.com/album/a1",
								},
								"artists": []any{map[string]any{"name": "Artist A"}},
							},
						},
					},
				})
			case "playlist":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"playlists": map[string]any{
						"items": []any{
							map[string]any{
								"id":   "p1",
								"name": "Playlist 1",
								"uri":  "spotify:playlist:p1",
								"external_urls": map[string]any{
									"spotify": "https://open.spotify.com/playlist/p1",
								},
								"owner":  map[string]any{"display_name": "User X"},
								"tracks": map[string]any{"total": 42},
							},
						},
					},
				})
			case "show":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"shows": map[string]any{
						"items": []any{
							map[string]any{
								"id":   "s1",
								"name": "Show 1",
								"uri":  "spotify:show:s1",
								"external_urls": map[string]any{
									"spotify": "https://open.spotify.com/show/s1",
								},
								"publisher": "Publisher P",
							},
						},
					},
				})
			case "episode":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"episodes": map[string]any{
						"items": []any{
							map[string]any{
								"id":   "e1",
								"name": "Episode 1",
								"uri":  "spotify:episode:e1",
								"external_urls": map[string]any{
									"spotify": "https://open.spotify.com/episode/e1",
								},
								"show": map[string]any{"name": "Show Name"},
							},
						},
					},
				})
			default:
				t.Fatalf("unexpected type query param: %q", typ)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("id", "secret", &http.Client{Timeout: 2 * time.Second})
	c.AccountsBaseURL = srv.URL
	c.APIBaseURL = srv.URL

	ctx := context.Background()

	tests := []struct {
		typ          SearchType
		wantID       string
		wantURI      string
		wantTitle    string
		wantSubtitle string
	}{
		{TypeAlbum, "a1", "spotify:album:a1", "Album 1", "Artist A"},
		{TypePlaylist, "p1", "spotify:playlist:p1", "Playlist 1", "User X — 42 tracks"},
		{TypeShow, "s1", "spotify:show:s1", "Show 1", "Publisher P"},
		{TypeEpisode, "e1", "spotify:episode:e1", "Episode 1", "Show Name"},
	}
	for _, tt := range tests {
		res, err := c.Search(ctx, "q", tt.typ, 1, "")
		if err != nil {
			t.Fatalf("Search(%s): %v", tt.typ, err)
		}
		if len(res) != 1 {
			t.Fatalf("Search(%s): expected 1 result, got %d", tt.typ, len(res))
		}
		if res[0].ID != tt.wantID || res[0].URI != tt.wantURI || res[0].Title != tt.wantTitle || res[0].Subtitle != tt.wantSubtitle {
			t.Fatalf("Search(%s): unexpected result: %+v", tt.typ, res[0])
		}
	}

	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("expected 1 token call, got %d", got)
	}
}
