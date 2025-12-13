package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const nameCompletionCacheTTL = 30 * time.Second

type nameCompletionCacheFile struct {
	UpdatedAt time.Time `json:"updatedAt"`
	Names     []string  `json:"names"`
}

func cachedNameCompletions(now time.Time) ([]string, bool) {
	path, err := nameCompletionCachePath()
	if err != nil {
		return nil, false
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	// Avoid large reads if the cache ever gets corrupted.
	if len(raw) > 64*1024 {
		return nil, false
	}

	var cache nameCompletionCacheFile
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, false
	}
	if cache.UpdatedAt.IsZero() || now.Sub(cache.UpdatedAt) > nameCompletionCacheTTL {
		return nil, false
	}
	if len(cache.Names) == 0 {
		return nil, false
	}
	return cache.Names, true
}

func storeNameCompletions(now time.Time, names []string) error {
	if len(names) == 0 {
		return errors.New("no names")
	}
	path, err := nameCompletionCachePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cache := nameCompletionCacheFile{
		UpdatedAt: now,
		Names:     names,
	}
	raw, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func nameCompletionCachePath() (string, error) {
	if override := os.Getenv("SONOSCLI_COMPLETION_CACHE_DIR"); override != "" {
		return filepath.Join(override, "sonoscli", "name-completions.json"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sonoscli", "name-completions.json"), nil
}
