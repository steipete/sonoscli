package sonos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SMAPITokenPair struct {
	AuthToken   string    `json:"authToken"`  //nolint:gosec // Persisted Sonos SMAPI credential.
	PrivateKey  string    `json:"privateKey"` //nolint:gosec // Persisted Sonos SMAPI credential.
	UpdatedAt   time.Time `json:"updatedAt"`
	LinkCode    string    `json:"linkCode,omitempty"`    // optional, for debugging
	DeviceID    string    `json:"deviceId,omitempty"`    // optional, for debugging
	HouseholdID string    `json:"householdId,omitempty"` // optional, for debugging
}

type SMAPITokenStore interface {
	Has(serviceID, householdID string) bool
	Load(serviceID, householdID string) (SMAPITokenPair, bool, error)
	Save(serviceID, householdID string, pair SMAPITokenPair) error
}

type FileSMAPITokenStore struct {
	path string
}

func NewFileSMAPITokenStore(path string) (*FileSMAPITokenStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path is required")
	}
	return &FileSMAPITokenStore{path: path}, nil
}

func NewDefaultSMAPITokenStore() (*FileSMAPITokenStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &FileSMAPITokenStore{path: filepath.Join(dir, "sonoscli", "smapi_tokens.json")}, nil
}

func (s *FileSMAPITokenStore) Has(serviceID, householdID string) bool {
	_, ok, _ := s.Load(serviceID, householdID)
	return ok
}

func (s *FileSMAPITokenStore) Load(serviceID, householdID string) (SMAPITokenPair, bool, error) {
	serviceID = strings.TrimSpace(serviceID)
	householdID = strings.TrimSpace(householdID)
	if serviceID == "" || householdID == "" {
		return SMAPITokenPair{}, false, nil
	}
	ff, err := s.readAll()
	if err != nil {
		return SMAPITokenPair{}, false, err
	}
	key := smapiTokenKey(serviceID, householdID)
	pair, ok := ff.Tokens[key]
	return pair, ok, nil
}

func (s *FileSMAPITokenStore) Save(serviceID, householdID string, pair SMAPITokenPair) error {
	serviceID = strings.TrimSpace(serviceID)
	householdID = strings.TrimSpace(householdID)
	if serviceID == "" || householdID == "" {
		return errors.New("serviceID and householdID are required")
	}
	pair.AuthToken = strings.TrimSpace(pair.AuthToken)
	pair.PrivateKey = strings.TrimSpace(pair.PrivateKey)
	if pair.AuthToken == "" {
		return errors.New("auth token is empty")
	}
	if pair.UpdatedAt.IsZero() {
		pair.UpdatedAt = time.Now().UTC()
	}

	ff, err := s.readAll()
	if err != nil {
		return err
	}
	key := smapiTokenKey(serviceID, householdID)
	if ff.Tokens == nil {
		ff.Tokens = map[string]SMAPITokenPair{}
	}
	ff.Tokens[key] = pair
	return s.writeAll(ff)
}

type smapiTokenFileFormat struct {
	Version int                       `json:"version"`
	Tokens  map[string]SMAPITokenPair `json:"tokens"`
}

func (s *FileSMAPITokenStore) readAll() (smapiTokenFileFormat, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return smapiTokenFileFormat{Version: 1, Tokens: map[string]SMAPITokenPair{}}, nil
		}
		return smapiTokenFileFormat{}, err
	}
	var ff smapiTokenFileFormat
	if err := json.Unmarshal(b, &ff); err != nil {
		return smapiTokenFileFormat{}, fmt.Errorf("parse token store: %w", err)
	}
	if ff.Version == 0 {
		ff.Version = 1
	}
	if ff.Tokens == nil {
		ff.Tokens = map[string]SMAPITokenPair{}
	}
	return ff, nil
}

func (s *FileSMAPITokenStore) writeAll(ff smapiTokenFileFormat) error {
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

func smapiTokenKey(serviceID, householdID string) string {
	return strings.TrimSpace(serviceID) + "#" + strings.TrimSpace(householdID)
}
