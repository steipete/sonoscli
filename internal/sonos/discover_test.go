package sonos

import "testing"

func TestPreferDeviceSet(t *testing.T) {
	best := map[string]Device{
		"1.1.1.1": {IP: "1.1.1.1", Name: "A"},
	}

	// Smaller candidate should be ignored.
	best2 := preferDeviceSet(best, map[string]Device{})
	if len(best2) != 1 {
		t.Fatalf("expected best unchanged, got %d", len(best2))
	}

	// Larger candidate should replace.
	candidateLarger := map[string]Device{
		"2.2.2.2": {IP: "2.2.2.2", Name: "B"},
		"3.3.3.3": {IP: "3.3.3.3", Name: "C"},
	}
	best3 := preferDeviceSet(best, candidateLarger)
	if len(best3) != 2 || best3["2.2.2.2"].Name != "B" {
		t.Fatalf("expected replace with candidate, got %#v", best3)
	}

	// Equal-size candidate should merge missing keys.
	bestEqual := map[string]Device{
		"10.0.0.1": {IP: "10.0.0.1", Name: "X"},
		"10.0.0.2": {IP: "10.0.0.2", Name: "Y"},
	}
	candidateEqual := map[string]Device{
		"10.0.0.2": {IP: "10.0.0.2", Name: "Y2"}, // existing key should not overwrite
		"10.0.0.3": {IP: "10.0.0.3", Name: "Z"},
	}
	merged := preferDeviceSet(bestEqual, candidateEqual)
	if len(merged) != 3 {
		t.Fatalf("expected merge size 3, got %d", len(merged))
	}
	if merged["10.0.0.2"].Name != "Y" {
		t.Fatalf("expected existing key preserved, got %q", merged["10.0.0.2"].Name)
	}
	if merged["10.0.0.3"].Name != "Z" {
		t.Fatalf("expected new key added, got %#v", merged["10.0.0.3"])
	}
}
