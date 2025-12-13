package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/sonos"
)

type fakeTopologyGetter struct {
	top sonos.Topology
	err error
}

func (f *fakeTopologyGetter) GetTopology(ctx context.Context) (sonos.Topology, error) {
	return f.top, f.err
}

type fakeGroupingClient struct {
	joinedUUID string
	joinCalls  int
	leaveCalls int
	joinErr    error
	leaveErr   error
}

func (f *fakeGroupingClient) JoinGroup(ctx context.Context, coordinatorUUID string) error {
	f.joinCalls++
	f.joinedUUID = coordinatorUUID
	return f.joinErr
}

func (f *fakeGroupingClient) LeaveGroup(ctx context.Context) error {
	f.leaveCalls++
	return f.leaveErr
}

func TestGroupJoinByName(t *testing.T) {
	flags := &rootFlags{Name: "Bedroom", Timeout: 2 * time.Second}
	cmd := newGroupJoinCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Living Room",
					IP:            "192.168.1.10",
					UUID:          "RINCON_LR1400",
					IsCoordinator: true,
				},
				Members: []sonos.Member{
					{Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400", IsCoordinator: true},
					{Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
				},
			},
			{
				ID: "G2",
				Coordinator: sonos.Member{
					Name:          "Bedroom",
					IP:            "192.168.1.20",
					UUID:          "RINCON_BED1400",
					IsCoordinator: true,
				},
				Members: []sonos.Member{
					{Name: "Bedroom", IP: "192.168.1.20", UUID: "RINCON_BED1400", IsCoordinator: true},
				},
			},
		},
		ByName: map[string]sonos.Member{
			"Living Room": {Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400"},
			"Kitchen":     {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
			"Bedroom":     {Name: "Bedroom", IP: "192.168.1.20", UUID: "RINCON_BED1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400"},
			"192.168.1.11": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
			"192.168.1.20": {Name: "Bedroom", IP: "192.168.1.20", UUID: "RINCON_BED1400"},
		},
	}

	origTG := newTopologyGetter
	origGC := newGroupingClient
	t.Cleanup(func() {
		newTopologyGetter = origTG
		newGroupingClient = origGC
	})

	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}
	fakeClient := &fakeGroupingClient{}
	newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
		if ip != "192.168.1.20" {
			t.Fatalf("unexpected joiner ip: %s", ip)
		}
		return fakeClient
	}

	cmd.SetArgs([]string{"--to", "Kitchen"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fakeClient.joinCalls != 1 {
		t.Fatalf("expected 1 join call, got %d", fakeClient.joinCalls)
	}
	if fakeClient.joinedUUID != "RINCON_LR1400" {
		t.Fatalf("unexpected coordinator uuid: %q", fakeClient.joinedUUID)
	}
}

func TestGroupUnjoinByIP(t *testing.T) {
	flags := &rootFlags{IP: "192.168.1.11", Timeout: 2 * time.Second}
	cmd := newGroupUnjoinCmd(flags)

	top := sonos.Topology{
		ByIP: map[string]sonos.Member{
			"192.168.1.11": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
		},
	}

	origTG := newTopologyGetter
	origGC := newGroupingClient
	t.Cleanup(func() {
		newTopologyGetter = origTG
		newGroupingClient = origGC
	})

	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}
	fakeClient := &fakeGroupingClient{}
	newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
		if ip != "192.168.1.11" {
			t.Fatalf("unexpected ip: %s", ip)
		}
		return fakeClient
	}

	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fakeClient.leaveCalls != 1 {
		t.Fatalf("expected 1 leave call, got %d", fakeClient.leaveCalls)
	}
}

func TestGroupJoinRequiresTo(t *testing.T) {
	flags := &rootFlags{Name: "Bedroom", Timeout: 2 * time.Second}
	cmd := newGroupJoinCmd(flags)
	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGroupStatusJSON(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, JSON: true}
	cmd := newGroupStatusCmd(flags)

	origTG := newTopologyGetter
	t.Cleanup(func() { newTopologyGetter = origTG })

	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: sonos.Topology{Groups: []sonos.Group{{ID: "G1"}}}}, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "\"id\": \"G1\"") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

type captureWriter struct {
	b []byte
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func (w *captureWriter) String() string {
	return string(w.b)
}

type discardWriter struct{}

func newDiscardWriter() *discardWriter { return &discardWriter{} }

func (w *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
