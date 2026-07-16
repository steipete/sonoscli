package sonos

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileSMAPITokenStore_SaveLoadHas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	if _, err := NewFileSMAPITokenStore(""); err == nil {
		t.Fatalf("expected error")
	}
	s, err := NewFileSMAPITokenStore(path)
	if err != nil {
		t.Fatalf("NewFileSMAPITokenStore: %v", err)
	}

	if s.Has("svc", "hh") {
		t.Fatalf("expected Has=false for missing file")
	}
	if _, ok, err := s.Load("", "hh"); err != nil || ok {
		t.Fatalf("Load empty keys: ok=%v err=%v", ok, err)
	}
	if err := s.Save("", "hh", SMAPITokenPair{AuthToken: "a", PrivateKey: "b"}); err == nil {
		t.Fatalf("expected error for missing serviceID")
	}
	if err := s.Save("svc", "hh", SMAPITokenPair{}); err == nil {
		t.Fatalf("expected error for empty token pair")
	}
	if err := s.Save("svc", "token-only", SMAPITokenPair{AuthToken: "a"}); err != nil {
		t.Fatalf("Save token-only: %v", err)
	}
	tokenOnly, ok, err := s.Load("svc", "token-only")
	if err != nil || !ok {
		t.Fatalf("Load token-only: ok=%v err=%v", ok, err)
	}
	if tokenOnly.AuthToken != "a" || tokenOnly.PrivateKey != "" {
		t.Fatalf("unexpected token-only pair: %#v", tokenOnly)
	}

	now := time.Date(2025, 12, 13, 0, 0, 0, 0, time.UTC)
	if err := s.Save("svc", "hh", SMAPITokenPair{AuthToken: "a", PrivateKey: "b", UpdatedAt: now}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !s.Has("svc", "hh") {
		t.Fatalf("expected Has=true")
	}
	pair, ok, err := s.Load("svc", "hh")
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if pair.AuthToken != "a" || pair.PrivateKey != "b" || !pair.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected pair: %#v", pair)
	}
}

func TestNewDefaultSMAPITokenStore(t *testing.T) {
	s, err := NewDefaultSMAPITokenStore()
	if err != nil {
		t.Fatalf("NewDefaultSMAPITokenStore: %v", err)
	}
	if s == nil || s.path == "" {
		t.Fatalf("expected store path")
	}
	if filepath.Base(s.path) != "smapi_tokens.json" {
		t.Fatalf("unexpected default path: %q", s.path)
	}
}
