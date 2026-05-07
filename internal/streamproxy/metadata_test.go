package streamproxy

import "testing"

func TestLooksLikeDirectMedia(t *testing.T) {
	t.Parallel()

	if !LooksLikeDirectMedia("https://example.com/episode.mp3?token=1") {
		t.Fatalf("expected mp3 to look direct")
	}
	if LooksLikeDirectMedia("https://www.youtube.com/watch?v=abc") {
		t.Fatalf("expected youtube watch URL not to look direct")
	}
}

func TestLooksLikeYouTube(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"https://www.youtube.com/watch?v=abc",
		"https://music.youtube.com/watch?v=abc",
		"https://youtu.be/abc",
	} {
		if !LooksLikeYouTube(rawURL) {
			t.Fatalf("expected %s to look like YouTube", rawURL)
		}
	}
}

func TestSourceDisplayFields(t *testing.T) {
	t.Parallel()

	src := Source{URL: "https://feeds.example.com/show/episode.mp3"}
	if got := src.DisplayTitle(); got != "episode.mp3" {
		t.Fatalf("title = %q", got)
	}
	if got := src.DisplayProvider(); got != "feeds.example.com" {
		t.Fatalf("provider = %q", got)
	}
}
