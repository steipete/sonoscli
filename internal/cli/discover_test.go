package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestDiscoverDefaultExcludesInvisible(t *testing.T) {
	flags := &rootFlags{Timeout: 123 * time.Millisecond, Format: formatPlain}
	cmd := newDiscoverCmd(flags)

	var got sonos.DiscoverOptions
	orig := discoverFunc
	t.Cleanup(func() { discoverFunc = orig })
	discoverFunc = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		got = opts
		return []sonos.Device{
			{Name: "Office", IP: "192.168.1.20", UDN: "RINCON_OFF1400"},
		}, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IncludeInvisible {
		t.Fatalf("expected IncludeInvisible=false by default")
	}
	if got.Timeout != flags.Timeout {
		t.Fatalf("expected timeout %s, got %s", flags.Timeout, got.Timeout)
	}
	if !strings.Contains(out.String(), "Office\t192.168.1.20\tRINCON_OFF1400") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestDiscoverAllIncludesInvisible(t *testing.T) {
	flags := &rootFlags{Timeout: 5 * time.Second, Format: formatPlain}
	cmd := newDiscoverCmd(flags)
	cmd.SetArgs([]string{"--all"})

	var got sonos.DiscoverOptions
	orig := discoverFunc
	t.Cleanup(func() { discoverFunc = orig })
	discoverFunc = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		got = opts
		return []sonos.Device{
			{Name: "Office", IP: "192.168.1.20", UDN: "RINCON_OFF1400"},
		}, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IncludeInvisible {
		t.Fatalf("expected IncludeInvisible=true with --all")
	}
}

func TestDiscoverJSONOutput(t *testing.T) {
	flags := &rootFlags{Timeout: 5 * time.Second, Format: formatJSON}
	cmd := newDiscoverCmd(flags)

	orig := discoverFunc
	t.Cleanup(func() { discoverFunc = orig })
	discoverFunc = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return []sonos.Device{
			{Name: "Kitchen", IP: "192.168.1.11", UDN: "RINCON_K1400"},
			{Name: "Bar", IP: "192.168.1.10", UDN: "RINCON_BAR1400"},
		}, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "\"name\": \"Bar\"") || !strings.Contains(s, "\"name\": \"Kitchen\"") {
		t.Fatalf("unexpected json output: %s", s)
	}
	// Ensure we didn't print tab-separated text.
	if strings.Contains(s, "\tRINCON_") {
		t.Fatalf("expected JSON output only, got: %s", s)
	}
}

func TestDiscoverNoDevicesPlainErrors(t *testing.T) {
	flags := &rootFlags{Timeout: 5 * time.Second, Format: formatPlain}
	cmd := newDiscoverCmd(flags)

	orig := discoverFunc
	t.Cleanup(func() { discoverFunc = orig })
	discoverFunc = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return nil, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "no speakers found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverNoDevicesJSONOutputsEmptyArray(t *testing.T) {
	flags := &rootFlags{Timeout: 5 * time.Second, Format: formatJSON}
	cmd := newDiscoverCmd(flags)

	orig := discoverFunc
	t.Cleanup(func() { discoverFunc = orig })
	discoverFunc = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return nil, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
