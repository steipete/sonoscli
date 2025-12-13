package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/sonos"
)

type fakeSpotifyEnqueuer struct {
	lastRef  string
	lastOpts sonos.EnqueueOptions
	pos      int
	err      error
}

func (f *fakeSpotifyEnqueuer) EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error) {
	f.lastRef = input
	f.lastOpts = opts
	return f.pos, f.err
}

func (f *fakeSpotifyEnqueuer) CoordinatorIP() string { return "192.168.1.20" }

type fakeSMAPISearcher struct {
	res sonos.SMAPISearchResult
	err error
}

func (f *fakeSMAPISearcher) Search(ctx context.Context, category, term string, index, count int) (sonos.SMAPISearchResult, error) {
	if f.err != nil {
		return sonos.SMAPISearchResult{}, f.err
	}
	return f.res, nil
}

func TestPlaySpotifyEnqueuesFirstResult(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatJSON}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	enq := &fakeSpotifyEnqueuer{pos: 7}
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return enq, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{res: sonos.SMAPISearchResult{
			MediaMetadata: []sonos.SMAPIItem{
				{ID: "spotify:track:abc123", Title: "Some Track"},
			},
		}}, sonos.MusicServiceDescriptor{Name: "Spotify", ID: "2311", Auth: "DeviceLink"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enq.lastRef != "spotify:track:abc123" {
		t.Fatalf("expected ref, got %q", enq.lastRef)
	}
	if !enq.lastOpts.PlayNow {
		t.Fatalf("expected PlayNow=true")
	}
	if enq.lastOpts.Title != "Some Track" {
		t.Fatalf("expected title from result, got %q", enq.lastOpts.Title)
	}
}

func TestPlaySpotifyEnqueueOnly(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatPlain}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	enq := &fakeSpotifyEnqueuer{pos: 1}
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return enq, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{res: sonos.SMAPISearchResult{
			MediaMetadata: []sonos.SMAPIItem{
				{ID: "spotify:track:abc123", Title: "Some Track"},
			},
		}}, sonos.MusicServiceDescriptor{Name: "Spotify", ID: "2311", Auth: "DeviceLink"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"--enqueue", "gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enq.lastOpts.PlayNow {
		t.Fatalf("expected PlayNow=false")
	}
}

func TestPlaySpotifyErrorsOnNoResults(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatPlain}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return &fakeSpotifyEnqueuer{}, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{res: sonos.SMAPISearchResult{}}, sonos.MusicServiceDescriptor{Name: "Spotify"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaySpotifyErrorsOnSMAPIError(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatPlain}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return &fakeSpotifyEnqueuer{}, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{err: errors.New("auth required")}, sonos.MusicServiceDescriptor{Name: "Spotify"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaySpotifyErrorsOnIndexOutOfRange(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatPlain}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	enq := &fakeSpotifyEnqueuer{pos: 1}
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return enq, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{res: sonos.SMAPISearchResult{
			MediaMetadata: []sonos.SMAPIItem{
				{ID: "spotify:track:abc123", Title: "Some Track"},
			},
		}}, sonos.MusicServiceDescriptor{Name: "Spotify"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"--index", "1", "gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if enq.lastRef != "" {
		t.Fatalf("expected no enqueue call, got ref=%q", enq.lastRef)
	}
}

func TestPlaySpotifyErrorsOnNonSpotifyResult(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatPlain}

	origEnq := newSpotifyEnqueuer
	origSmapi := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSmapi
	})

	enq := &fakeSpotifyEnqueuer{pos: 1}
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return enq, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return &fakeSMAPISearcher{res: sonos.SMAPISearchResult{
			MediaMetadata: []sonos.SMAPIItem{
				{ID: "notspotify:thing", Title: "Not Spotify"},
			},
		}}, sonos.MusicServiceDescriptor{Name: "Spotify"}, &sonos.Client{IP: "192.168.1.10"}, nil
	}

	cmd := newPlaySpotifyCmd(flags)
	cmd.SetArgs([]string{"gareth emery"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if enq.lastRef != "" {
		t.Fatalf("expected no enqueue call, got ref=%q", enq.lastRef)
	}
}
