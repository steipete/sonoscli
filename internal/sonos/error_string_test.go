package sonos

import "testing"

func TestUPnPErrorError(t *testing.T) {
	if (&UPnPError{Code: "701"}).Error() != "upnp error 701" {
		t.Fatalf("unexpected error string")
	}
	if (&UPnPError{Code: "701", Description: "Transition not available"}).Error() != "upnp error 701: Transition not available" {
		t.Fatalf("unexpected error string with description")
	}
}
