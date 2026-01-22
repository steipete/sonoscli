package cli

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestNameFlagCompletion_ReturnsSortedUniqueAndFiltersByPrefix(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

	cacheDir := t.TempDir()
	t.Setenv("SONOSCLI_COMPLETION_CACHE_DIR", cacheDir)

	var gotTimeout time.Duration
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		gotTimeout = opts.Timeout
		return []sonos.Device{
			{Name: "Kitchen"},
			{Name: "Living Room"},
			{Name: "Kitchen"},
			{Name: "  Office  "},
			{Name: "Office Sonos"},
			{Name: ""},
		}, nil
	}

	flags := &rootFlags{Timeout: 250 * time.Millisecond}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	completeName := nameFlagCompletion(flags)

	got, directive := completeName(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if gotTimeout != 250*time.Millisecond {
		t.Fatalf("discover timeout = %s, want %s", gotTimeout, 250*time.Millisecond)
	}
	want := []string{"Kitchen", `Living\ Room`, "Office", `Office\ Sonos`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %#v, want %#v", got, want)
	}

	got, _ = completeName(cmd, nil, "ki")
	want = []string{"Kitchen"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions(ki) = %#v, want %#v", got, want)
	}

	got, _ = completeName(cmd, nil, "li")
	want = []string{`Living\ Room`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions(li) = %#v, want %#v", got, want)
	}
}

func TestNameFlagCompletion_SkipsOnDiscoverError(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

	cacheDir := t.TempDir()
	t.Setenv("SONOSCLI_COMPLETION_CACHE_DIR", cacheDir)

	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return nil, errors.New("boom")
	}

	flags := &rootFlags{Timeout: 10 * time.Second}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	completeName := nameFlagCompletion(flags)
	got, directive := completeName(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if len(got) != 0 {
		t.Fatalf("completions = %#v, want none", got)
	}
}

func TestNameFlagCompletion_UsesDiskCache(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

	cacheDir := t.TempDir()
	t.Setenv("SONOSCLI_COMPLETION_CACHE_DIR", cacheDir)

	callCount := 0
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		callCount++
		return []sonos.Device{
			{Name: "Living Room"},
		}, nil
	}

	flags := &rootFlags{Timeout: time.Second}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	completeName := nameFlagCompletion(flags)

	got, _ := completeName(cmd, nil, "")
	want := []string{`Living\ Room`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %#v, want %#v", got, want)
	}
	if callCount != 1 {
		t.Fatalf("discover calls = %d, want 1", callCount)
	}

	// If we can read from cache, discovery shouldn't be called again.
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		t.Fatalf("expected cache hit; discovery should not be called")
		return nil, nil
	}
	got, _ = completeName(cmd, nil, "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions(cache) = %#v, want %#v", got, want)
	}
}

func TestNameFlagCompletion_UsesStaleDiskCacheOnDiscoverError(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

	cacheDir := t.TempDir()
	t.Setenv("SONOSCLI_COMPLETION_CACHE_DIR", cacheDir)

	staleAt := time.Now().Add(-nameCompletionCacheTTL - 1*time.Second)
	if err := storeNameCompletions(staleAt, []string{"Living Room"}); err != nil {
		t.Fatalf("store cache: %v", err)
	}

	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return nil, errors.New("boom")
	}

	flags := &rootFlags{Timeout: 10 * time.Second}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	completeName := nameFlagCompletion(flags)
	got, directive := completeName(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	want := []string{`Living\ Room`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions(stale cache) = %#v, want %#v", got, want)
	}
}
