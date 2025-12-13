package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DefaultRoom string `json:"defaultRoom,omitempty"`
	Format      string `json:"format,omitempty"`
}

func (c Config) Normalize() Config {
	out := Config{
		DefaultRoom: strings.TrimSpace(c.DefaultRoom),
		Format:      strings.ToLower(strings.TrimSpace(c.Format)),
	}
	if out.Format == "" {
		out.Format = "plain"
	}
	if !isValidFormat(out.Format) {
		out.Format = "plain"
	}
	return out
}

func isValidFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "plain", "json", "tsv":
		return true
	default:
		return false
	}
}

type Store interface {
	Path() string
	Load() (Config, error)
	Save(cfg Config) error
}

type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path is required")
	}
	return &FileStore{path: path}, nil
}

func NewDefaultStore() (*FileStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &FileStore{path: filepath.Join(dir, "sonoscli", "config.json")}, nil
}

func (s *FileStore) Path() string { return s.path }

type fileFormat struct {
	Version int    `json:"version"`
	Config  Config `json:"config"`
}

func (s *FileStore) Load() (Config, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}.Normalize(), nil
		}
		return Config{}, err
	}
	var ff fileFormat
	if err := json.Unmarshal(raw, &ff); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return ff.Config.Normalize(), nil
}

func (s *FileStore) Save(cfg Config) error {
	cfg = cfg.Normalize()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	ff := fileFormat{Version: 1, Config: cfg}
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
