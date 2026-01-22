package applemusic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Token holds Apple Music authentication credentials.
// Two tokens are needed: DeveloperToken (JWT for Bearer auth) and
// MusicUserToken (user session token). Both are obtained from MusicKit web.
type Token struct {
	DeveloperToken string    `json:"developerToken"`          // JWT for Authorization: Bearer header
	MusicUserToken string    `json:"musicUserToken"`          // For Music-User-Token header
	StorefrontID   string    `json:"storefrontId,omitempty"`  // e.g., "us", "gb"
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt,omitempty"`     // estimated, not guaranteed
}

// IsExpired checks if the token has likely expired.
// Apple Music tokens last ~6 months but exact expiry is undocumented.
func (t Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		// If no expiry set, check if token is older than 6 months
		return time.Since(t.CreatedAt) > 180*24*time.Hour
	}
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the token has required fields and is not expired.
func (t Token) IsValid() bool {
	return strings.TrimSpace(t.DeveloperToken) != "" &&
		strings.TrimSpace(t.MusicUserToken) != "" &&
		!t.IsExpired()
}

// TokenStore interface for Apple Music token persistence.
type TokenStore interface {
	Load() (Token, bool, error)
	Save(token Token) error
	Delete() error
}

// FileTokenStore implements TokenStore using a JSON file.
type FileTokenStore struct {
	path string
}

// NewFileTokenStore creates a new file-based token store.
func NewFileTokenStore(path string) (*FileTokenStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path is required")
	}
	return &FileTokenStore{path: path}, nil
}

// NewDefaultTokenStore creates a token store at the default config location.
func NewDefaultTokenStore() (*FileTokenStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &FileTokenStore{path: filepath.Join(dir, "sonoscli", "applemusic_token.json")}, nil
}

// Load reads the token from disk.
func (s *FileTokenStore) Load() (Token, bool, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Token{}, false, nil
		}
		return Token{}, false, err
	}

	var ff tokenFileFormat
	if err := json.Unmarshal(b, &ff); err != nil {
		return Token{}, false, fmt.Errorf("parse apple music token: %w", err)
	}

	if ff.Token.DeveloperToken == "" || ff.Token.MusicUserToken == "" {
		return Token{}, false, nil
	}

	return ff.Token, true, nil
}

// Save writes the token to disk.
func (s *FileTokenStore) Save(token Token) error {
	token.DeveloperToken = strings.TrimSpace(token.DeveloperToken)
	token.MusicUserToken = strings.TrimSpace(token.MusicUserToken)
	if token.DeveloperToken == "" {
		return errors.New("developerToken is required")
	}
	if token.MusicUserToken == "" {
		return errors.New("musicUserToken is required")
	}

	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	// Set estimated expiry to 6 months if not provided
	if token.ExpiresAt.IsZero() {
		token.ExpiresAt = token.CreatedAt.Add(180 * 24 * time.Hour)
	}

	ff := tokenFileFormat{
		Version: 1,
		Token:   token,
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	b, err := json.MarshalIndent(ff, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Delete removes the token file.
func (s *FileTokenStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Path returns the file path of the token store.
func (s *FileTokenStore) Path() string {
	return s.path
}

type tokenFileFormat struct {
	Version int   `json:"version"`
	Token   Token `json:"token"`
}
