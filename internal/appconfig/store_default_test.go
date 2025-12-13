package appconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDefaultStore_PathSuffix(t *testing.T) {
	s, err := NewDefaultStore()
	if err != nil {
		t.Fatalf("NewDefaultStore: %v", err)
	}
	p := filepath.ToSlash(s.Path())
	if !strings.HasSuffix(p, "/sonoscli/config.json") {
		t.Fatalf("unexpected path: %q", p)
	}
}
