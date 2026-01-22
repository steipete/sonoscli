package cli

import (
	"testing"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestFindServiceByName(t *testing.T) {
	services := []sonos.MusicServiceDescriptor{
		{Name: "Spotify", ID: "2311"},
		{Name: "Plex", ID: "123"},
		{Name: "Spotify US", ID: "3079"},
	}

	if _, err := findServiceByName(services, ""); err == nil {
		t.Fatalf("expected error for empty")
	}

	s, err := findServiceByName(services, "spotify")
	if err != nil {
		t.Fatalf("exact: %v", err)
	}
	if s.Name != "Spotify" {
		t.Fatalf("expected Spotify, got %q", s.Name)
	}

	// Unique partial match.
	s, err = findServiceByName(services, "plex")
	if err != nil {
		t.Fatalf("partial: %v", err)
	}
	if s.Name != "Plex" {
		t.Fatalf("expected Plex, got %q", s.Name)
	}

	// Ambiguous partial match.
	if _, err := findServiceByName(services, "spot"); err == nil {
		t.Fatalf("expected ambiguous error")
	}

	if _, err := findServiceByName(services, "nope"); err == nil {
		t.Fatalf("expected not found error")
	}
}
