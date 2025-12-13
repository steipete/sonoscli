package sonos

import "testing"

func TestJoinURI(t *testing.T) {
	t.Parallel()

	if _, err := JoinURI(""); err == nil {
		t.Fatalf("expected error for empty uuid")
	}

	got, err := JoinURI("RINCON_123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "x-rincon:RINCON_123" {
		t.Fatalf("unexpected join uri: %q", got)
	}
}
