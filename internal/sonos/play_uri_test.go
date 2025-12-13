package sonos

import (
	"strings"
	"testing"
)

func TestForceRadioURI(t *testing.T) {
	t.Parallel()

	if got := ForceRadioURI("http://example.com/stream"); got != "x-rincon-mp3radio://example.com/stream" {
		t.Fatalf("got %q", got)
	}
	if got := ForceRadioURI("https://example.com/stream"); got != "x-rincon-mp3radio://example.com/stream" {
		t.Fatalf("got %q", got)
	}
	if got := ForceRadioURI("x-sonosapi-stream:abc"); got != "x-rincon-mp3radio:abc" {
		t.Fatalf("got %q", got)
	}
	if got := ForceRadioURI("notauri"); got != "notauri" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRadioMeta(t *testing.T) {
	t.Parallel()

	meta := BuildRadioMeta("My Station")
	if meta == "" {
		t.Fatalf("expected meta")
	}
	if !containsAll(meta, []string{"My Station", "object.item.audioItem.audioBroadcast", "SA_RINCON65031_"}) {
		t.Fatalf("unexpected meta: %s", meta)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
