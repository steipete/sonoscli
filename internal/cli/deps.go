package cli

import "github.com/STop211650/sonoscli/internal/sonos"

// Dependency injection points for tests.
var newSMAPITokenStore = func() (sonos.SMAPITokenStore, error) {
	return sonos.NewDefaultSMAPITokenStore()
}
