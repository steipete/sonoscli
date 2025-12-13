package cli

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

func TestNameFlagCompletion_ReturnsSortedUniqueAndFiltersByPrefix(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

	var gotTimeout time.Duration
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		gotTimeout = opts.Timeout
		return []sonos.Device{
			{Name: "Kitchen"},
			{Name: "Living Room"},
			{Name: "Kitchen"},
			{Name: "  Office  "},
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
	want := []string{"Kitchen", "Living Room", "Office"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %#v, want %#v", got, want)
	}

	got, _ = completeName(cmd, nil, "ki")
	want = []string{"Kitchen"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions(ki) = %#v, want %#v", got, want)
	}
}

func TestNameFlagCompletion_SkipsOnDiscoverError(t *testing.T) {
	origDiscover := sonosDiscover
	t.Cleanup(func() { sonosDiscover = origDiscover })

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
