package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/sonos"
)

type fakeSpotifyEnqueuer struct {
	input string
	opts  sonos.EnqueueOptions
	err   error
}

func (f *fakeSpotifyEnqueuer) EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error) {
	f.input = input
	f.opts = opts
	return 7, f.err
}

func (f *fakeSpotifyEnqueuer) CoordinatorIP() string { return "192.0.2.7" }

type fakeSMAPISearcher struct {
	result sonos.SMAPISearchResult
	err    error
}

func (f fakeSMAPISearcher) Search(ctx context.Context, category, term string, index, count int) (sonos.SMAPISearchResult, error) {
	return f.result, f.err
}

func TestPlaySpotifyCmdSuccessJSON(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newPlaySpotifyCmd(flags)

	enq := &fakeSpotifyEnqueuer{}
	origEnq := newSpotifyEnqueuer
	origSearch := newSMAPISearcher
	t.Cleanup(func() {
		newSpotifyEnqueuer = origEnq
		newSMAPISearcher = origSearch
	})
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return enq, nil
	}
	newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
		return fakeSMAPISearcher{result: sonos.SMAPISearchResult{
			MediaMetadata: []sonos.SMAPIItem{{ID: "spotify:track:abc", Title: "Track"}},
		}}, sonos.MusicServiceDescriptor{ID: "2311", Name: serviceName, Auth: sonos.MusicServiceAuthDeviceLink}, &sonos.Client{IP: "192.0.2.9"}, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--enqueue", "--title", "Override", "query"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enq.input != "spotify:track:abc" {
		t.Fatalf("input = %q", enq.input)
	}
	if enq.opts.Title != "Override" || enq.opts.PlayNow {
		t.Fatalf("opts = %+v", enq.opts)
	}
}

func TestPlaySpotifyCmdResultErrors(t *testing.T) {
	tests := []struct {
		name   string
		result sonos.SMAPISearchResult
		args   []string
		want   string
	}{
		{name: "no results", result: sonos.SMAPISearchResult{}, args: []string{"query"}, want: "no results"},
		{name: "out of range", result: sonos.SMAPISearchResult{MediaMetadata: []sonos.SMAPIItem{{ID: "spotify:track:abc", Title: "Track"}}}, args: []string{"--index", "2", "query"}, want: "out of range"},
		{name: "not spotify", result: sonos.SMAPISearchResult{MediaMetadata: []sonos.SMAPIItem{{ID: "track:abc", Title: "Track"}}}, args: []string{"query"}, want: "not a playable Spotify ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
			cmd := newPlaySpotifyCmd(flags)

			origEnq := newSpotifyEnqueuer
			origSearch := newSMAPISearcher
			t.Cleanup(func() {
				newSpotifyEnqueuer = origEnq
				newSMAPISearcher = origSearch
			})
			newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
				return &fakeSpotifyEnqueuer{}, nil
			}
			newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
				return fakeSMAPISearcher{result: tt.result}, sonos.MusicServiceDescriptor{ID: "2311", Name: "Spotify"}, &sonos.Client{IP: "192.0.2.9"}, nil
			}

			cmd.SetOut(newDiscardWriter())
			cmd.SetErr(newDiscardWriter())
			cmd.SetArgs(tt.args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestPlaySpotifyCmdDependencyErrors(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlaySpotifyCmd(flags)

	origEnq := newSpotifyEnqueuer
	t.Cleanup(func() { newSpotifyEnqueuer = origEnq })
	newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
		return nil, errors.New("enqueue setup")
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"query"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "enqueue setup") {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestRealSpotifyEnqueuer_EnqueueSpotify(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/xml/device_description.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <manufacturer>Sonos, Inc.</manufacturer>
    <roomName>Office</roomName>
    <UDN>uuid:RINCON_OFFICE1400</UDN>
  </device>
</root>`))
	})
	mux.HandleFunc("/MediaRenderer/AVTransport/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "AVTransport:1#AddURIToQueue"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:AddURIToQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <FirstTrackNumberEnqueued>1</FirstTrackNumberEnqueued>
    </u:AddURIToQueueResponse>
  </s:Body>
</s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SetAVTransportURIResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Seek"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SeekResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Play"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`))
		default:
			t.Fatalf("unexpected SOAPACTION: %q", action)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	c := &sonos.Client{
		IP:   u.Hostname(),
		Port: port,
		HTTP: srv.Client(),
	}

	enq := realSpotifyEnqueuer{c: c}
	if got := enq.CoordinatorIP(); got != c.IP {
		t.Fatalf("CoordinatorIP: %q", got)
	}

	pos, err := enq.EnqueueSpotify(context.Background(), "spotify:track:abc", sonos.EnqueueOptions{
		Title:   "X",
		PlayNow: true,
	})
	if err != nil {
		t.Fatalf("EnqueueSpotify: %v", err)
	}
	if pos != 1 {
		t.Fatalf("expected pos=1, got %d", pos)
	}

	// Ensure wrapper passes through context cancellation cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.HTTP.Timeout = 10 * time.Second
	_, _ = enq.EnqueueSpotify(ctx, "spotify:track:abc", sonos.EnqueueOptions{Title: "X"})
}
