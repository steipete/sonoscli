package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

type fakeSourceClient struct {
	setCalls  int
	uri       string
	meta      string
	playCalls int
}

func (f *fakeSourceClient) SetAVTransportURI(ctx context.Context, uri, meta string) error {
	f.setCalls++
	f.uri = uri
	f.meta = meta
	return nil
}

func (f *fakeSourceClient) Play(ctx context.Context) error {
	f.playCalls++
	return nil
}

func TestPlayURICmdRadio(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newPlayURICmd(flags)

	fake := &fakeSourceClient{}
	origClient := newSourceClient
	t.Cleanup(func() { newSourceClient = origClient })
	newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--radio", "--title", "My Station", "http://example.com/stream"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.setCalls != 1 || fake.playCalls != 1 {
		t.Fatalf("expected set+play once, got set=%d play=%d", fake.setCalls, fake.playCalls)
	}
	if fake.uri != "x-rincon-mp3radio://example.com/stream" {
		t.Fatalf("unexpected uri: %q", fake.uri)
	}
	if !strings.Contains(fake.meta, "My Station") {
		t.Fatalf("expected meta to include title, got: %q", fake.meta)
	}
}

func TestLineInCmdFrom(t *testing.T) {
	flags := &rootFlags{Name: "Living Room", Timeout: 2 * time.Second}
	cmd := newLineInCmd(flags)

	top := sonos.Topology{
		ByName: map[string]sonos.Member{
			"Kitchen": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.11": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
		},
	}

	origTG := newTopologyGetter
	origClient := newSourceClient
	t.Cleanup(func() {
		newTopologyGetter = origTG
		newSourceClient = origClient
	})

	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}
	fake := &fakeSourceClient{}
	newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--from", "Kitchen"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.uri != "x-rincon-stream:RINCON_K1400" {
		t.Fatalf("unexpected uri: %q", fake.uri)
	}
}

func TestTVCmd(t *testing.T) {
	flags := &rootFlags{Name: "Living Room", Timeout: 2 * time.Second}
	cmd := newTVCmd(flags)

	top := sonos.Topology{
		ByName: map[string]sonos.Member{
			"Living Room": {Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400"},
		},
	}

	origTG := newTopologyGetter
	origClient := newSourceClient
	t.Cleanup(func() {
		newTopologyGetter = origTG
		newSourceClient = origClient
	})

	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}
	fake := &fakeSourceClient{}
	newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.uri != "x-sonos-htastream:RINCON_LR1400:spdif" {
		t.Fatalf("unexpected uri: %q", fake.uri)
	}
}
