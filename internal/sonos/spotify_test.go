package sonos

import "testing"

func TestParseSpotifyRef(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		ref, ok := ParseSpotifyRef("spotify:track:6NmXV4o6bmp704aPGyTVVG")
		if !ok {
			t.Fatalf("expected ok")
		}
		if ref.Kind != SpotifyTrack {
			t.Fatalf("kind: %v", ref.Kind)
		}
		if ref.EncodedID != "spotify%3atrack%3a6NmXV4o6bmp704aPGyTVVG" {
			t.Fatalf("encoded: %q", ref.EncodedID)
		}
	})

	t.Run("shareURL", func(t *testing.T) {
		ref, ok := ParseSpotifyRef("https://open.spotify.com/track/6NmXV4o6bmp704aPGyTVVG?si=abc")
		if !ok {
			t.Fatalf("expected ok")
		}
		if ref.Kind != SpotifyTrack {
			t.Fatalf("kind: %v", ref.Kind)
		}
		if ref.ID != "6NmXV4o6bmp704aPGyTVVG" {
			t.Fatalf("id: %q", ref.ID)
		}
	})
}
