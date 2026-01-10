package applemusic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		token   Token
		expired bool
	}{
		{
			name: "fresh token with explicit expiry",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test",
				CreatedAt:      time.Now(),
				ExpiresAt:      time.Now().Add(30 * 24 * time.Hour),
			},
			expired: false,
		},
		{
			name: "expired token with explicit expiry",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test",
				CreatedAt:      time.Now().Add(-200 * 24 * time.Hour),
				ExpiresAt:      time.Now().Add(-10 * 24 * time.Hour),
			},
			expired: true,
		},
		{
			name: "old token without explicit expiry",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test",
				CreatedAt:      time.Now().Add(-200 * 24 * time.Hour),
			},
			expired: true,
		},
		{
			name: "recent token without explicit expiry",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test",
				CreatedAt:      time.Now().Add(-30 * 24 * time.Hour),
			},
			expired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestTokenIsValid(t *testing.T) {
	tests := []struct {
		name  string
		token Token
		valid bool
	}{
		{
			name: "valid token",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test-token",
				CreatedAt:      time.Now(),
			},
			valid: true,
		},
		{
			name: "empty user token",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "",
				CreatedAt:      time.Now(),
			},
			valid: false,
		},
		{
			name: "empty developer token",
			token: Token{
				DeveloperToken: "",
				MusicUserToken: "test-token",
				CreatedAt:      time.Now(),
			},
			valid: false,
		},
		{
			name: "whitespace user token",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "   ",
				CreatedAt:      time.Now(),
			},
			valid: false,
		},
		{
			name: "whitespace developer token",
			token: Token{
				DeveloperToken: "   ",
				MusicUserToken: "test-token",
				CreatedAt:      time.Now(),
			},
			valid: false,
		},
		{
			name: "expired token",
			token: Token{
				DeveloperToken: "dev-token",
				MusicUserToken: "test-token",
				CreatedAt:      time.Now().Add(-200 * 24 * time.Hour),
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestFileTokenStore(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_token.json")

	store, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}

	// Test Load on non-existent file
	_, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load (empty): %v", err)
	}
	if ok {
		t.Error("Load (empty): expected ok=false")
	}

	// Test Save
	testToken := Token{
		DeveloperToken: "test-developer-token-jwt",
		MusicUserToken: "test-music-user-token-12345",
		StorefrontID:   "us",
		CreatedAt:      time.Now().UTC(),
	}
	if err := store.Save(testToken); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Save: file was not created")
	}

	// Test Load after save
	loaded, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load (after save): %v", err)
	}
	if !ok {
		t.Error("Load (after save): expected ok=true")
	}
	if loaded.DeveloperToken != testToken.DeveloperToken {
		t.Errorf("DeveloperToken = %q, want %q", loaded.DeveloperToken, testToken.DeveloperToken)
	}
	if loaded.MusicUserToken != testToken.MusicUserToken {
		t.Errorf("MusicUserToken = %q, want %q", loaded.MusicUserToken, testToken.MusicUserToken)
	}
	if loaded.StorefrontID != testToken.StorefrontID {
		t.Errorf("StorefrontID = %q, want %q", loaded.StorefrontID, testToken.StorefrontID)
	}
	if loaded.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set automatically")
	}

	// Test Delete
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Delete: file still exists")
	}

	// Test Delete on non-existent file (should not error)
	if err := store.Delete(); err != nil {
		t.Errorf("Delete (non-existent): %v", err)
	}
}

func TestFileTokenStoreEmptyPath(t *testing.T) {
	_, err := NewFileTokenStore("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSaveEmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_token.json")
	store, _ := NewFileTokenStore(path)

	err := store.Save(Token{})
	if err == nil {
		t.Error("expected error for empty token")
	}
}
