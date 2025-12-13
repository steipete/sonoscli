package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/appconfig"
)

func TestConfigSetGetUnset(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatPlain}

	dir := t.TempDir()
	store, err := appconfig.NewFileStore(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	orig := newConfigStore
	t.Cleanup(func() { newConfigStore = orig })
	newConfigStore = func() (appconfig.Store, error) { return store, nil }

	// set defaultRoom
	{
		cmd := newConfigSetCmd(flags)
		cmd.SetOut(newDiscardWriter())
		cmd.SetErr(newDiscardWriter())
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"defaultRoom", "Office"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("set defaultRoom: %v", err)
		}
	}

	// get
	{
		cmd := newConfigGetCmd(flags)
		var out captureWriter
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("get: %v", err)
		}
		if !strings.Contains(out.String(), "defaultRoom=Office") {
			t.Fatalf("unexpected get output: %s", out.String())
		}
	}

	// unset
	{
		cmd := newConfigUnsetCmd(flags)
		cmd.SetOut(newDiscardWriter())
		cmd.SetErr(newDiscardWriter())
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"defaultRoom"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("unset: %v", err)
		}
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultRoom != "" {
		t.Fatalf("expected defaultRoom to be empty, got %q", cfg.DefaultRoom)
	}
}
