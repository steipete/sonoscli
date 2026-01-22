package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
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

func TestResolveMemberFuzzyUnique(t *testing.T) {
	top := sonos.Topology{
		ByName: map[string]sonos.Member{
			"Office":  {Name: "Office", IP: "192.168.1.10"},
			"Kitchen": {Name: "Kitchen", IP: "192.168.1.11"},
		},
	}

	mem, err := resolveMember(top, "Off", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mem.Name != "Office" {
		t.Fatalf("unexpected member: %+v", mem)
	}
}

func TestResolveMemberFuzzyAmbiguous(t *testing.T) {
	top := sonos.Topology{
		ByName: map[string]sonos.Member{
			"Office":      {Name: "Office", IP: "192.168.1.10"},
			"Home Office": {Name: "Home Office", IP: "192.168.1.12"},
		},
	}

	_, err := resolveMember(top, "off", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous") {
		t.Fatalf("unexpected error: %s", msg)
	}
	if !strings.Contains(msg, "Home Office") || !strings.Contains(msg, "Office") {
		t.Fatalf("missing suggestions: %s", msg)
	}
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

type recordingGroupingClient struct {
	ip          string
	joinedUUIDs *[]string
	leaveIPs    *[]string
}

func (r *recordingGroupingClient) JoinGroup(ctx context.Context, coordinatorUUID string) error {
	*r.joinedUUIDs = append(*r.joinedUUIDs, r.ip+"->"+coordinatorUUID)
	return nil
}

func (r *recordingGroupingClient) LeaveGroup(ctx context.Context) error {
	*r.leaveIPs = append(*r.leaveIPs, r.ip)
	return nil
}

func TestGroupPartyJoinsAllNonDestinationMembers(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second}
	cmd := newGroupPartyCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Bar",
					IP:            "192.168.1.10",
					UUID:          "RINCON_BAR1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400", IsCoordinator: true, IsVisible: true},
					{Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400", IsVisible: true},
				},
			},
			{
				ID: "G2",
				Coordinator: sonos.Member{
					Name:          "Office",
					IP:            "192.168.1.20",
					UUID:          "RINCON_OFF1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400", IsCoordinator: true, IsVisible: true},
					{Name: "Invisible", IP: "192.168.1.21", UUID: "RINCON_INV1400", IsVisible: false},
				},
			},
		},
		ByName: map[string]sonos.Member{
			"Bar":     {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"Kitchen": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
			"Office":  {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"192.168.1.11": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400"},
			"192.168.1.20": {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
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

	var joined []string
	var left []string
	newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
		return &recordingGroupingClient{ip: ip, joinedUUIDs: &joined, leaveIPs: &left}
	}

	cmd.SetArgs([]string{"--to", "Bar"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(joined) != 1 {
		t.Fatalf("expected 1 join call, got %d: %#v", len(joined), joined)
	}
	if joined[0] != "192.168.1.20->RINCON_BAR1400" {
		t.Fatalf("unexpected join operation: %s", joined[0])
	}
}

func TestGroupDissolveLeavesAllMembersCoordinatorLast(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newGroupDissolveCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Bar",
					IP:            "192.168.1.10",
					UUID:          "RINCON_BAR1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400", IsCoordinator: true, IsVisible: true},
					{Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400", IsVisible: true},
				},
			},
		},
		ByName: map[string]sonos.Member{
			"Bar":    {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"Office": {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"192.168.1.20": {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
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

	var joined []string
	var left []string
	newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
		return &recordingGroupingClient{ip: ip, joinedUUIDs: &joined, leaveIPs: &left}
	}

	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(left) != 2 {
		t.Fatalf("expected 2 leave calls, got %d: %#v", len(left), left)
	}
	if left[0] != "192.168.1.20" || left[1] != "192.168.1.10" {
		t.Fatalf("expected coordinator last; got: %#v", left)
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
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatJSON}
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

func TestGroupStatusHidesInvisibleByDefault(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatPlain}
	cmd := newGroupStatusCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Office",
					IP:            "192.168.1.10",
					UUID:          "RINCON_OFF1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Office", IP: "192.168.1.10", UUID: "RINCON_OFF1400", IsCoordinator: true, IsVisible: true},
					{Name: "Invisible", IP: "192.168.1.11", UUID: "RINCON_INV1400", IsVisible: false},
				},
			},
		},
	}

	origTG := newTopologyGetter
	t.Cleanup(func() { newTopologyGetter = origTG })
	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "Invisible") {
		t.Fatalf("expected invisible member to be hidden, got: %s", out.String())
	}
}

func TestGroupStatusAllShowsInvisible(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatTSV}
	cmd := newGroupStatusCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Office",
					IP:            "192.168.1.10",
					UUID:          "RINCON_OFF1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Office", IP: "192.168.1.10", UUID: "RINCON_OFF1400", IsCoordinator: true, IsVisible: true},
					{Name: "Invisible", IP: "192.168.1.11", UUID: "RINCON_INV1400", IsVisible: false},
				},
			},
		},
	}

	origTG := newTopologyGetter
	t.Cleanup(func() { newTopologyGetter = origTG })
	newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
		return &fakeTopologyGetter{top: top}, nil
	}

	cmd.SetArgs([]string{"--all"})
	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "\tInvisible\t") {
		t.Fatalf("expected invisible member in TSV output, got: %s", out.String())
	}
}

func TestGroupSoloLeavesOthersThenTarget(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newGroupSoloCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Bar",
					IP:            "192.168.1.10",
					UUID:          "RINCON_BAR1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400", IsCoordinator: true, IsVisible: true},
					{Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400", IsVisible: true},
					{Name: "Invisible", IP: "192.168.1.21", UUID: "RINCON_INV1400", IsVisible: false},
				},
			},
		},
		ByName: map[string]sonos.Member{
			"Bar":    {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"Office": {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Bar", IP: "192.168.1.10", UUID: "RINCON_BAR1400"},
			"192.168.1.20": {Name: "Office", IP: "192.168.1.20", UUID: "RINCON_OFF1400"},
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

	var left []string
	newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
		return &recordingGroupingClient{ip: ip, joinedUUIDs: new([]string), leaveIPs: &left}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have left Bar then Office (target last). Invisible is ignored.
	if len(left) != 2 || left[0] != "192.168.1.10" || left[1] != "192.168.1.20" {
		t.Fatalf("unexpected leave order: %#v", left)
	}
	if !strings.Contains(out.String(), "\"target\"") {
		t.Fatalf("expected json output, got: %s", out.String())
	}
}
