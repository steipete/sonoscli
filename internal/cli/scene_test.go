package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/scenes"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type fakeSceneStore struct {
	scenes map[string]scenes.Scene
	put    scenes.Scene

	listCalls   int
	deleteCalls int
	deletedName string
}

func (f *fakeSceneStore) List() ([]scenes.SceneMeta, error) {
	f.listCalls++
	out := make([]scenes.SceneMeta, 0, len(f.scenes))
	for _, s := range f.scenes {
		out = append(out, scenes.SceneMeta{Name: s.Name, CreatedAt: s.CreatedAt})
	}
	return out, nil
}

func (f *fakeSceneStore) Get(name string) (scenes.Scene, bool, error) {
	sc, ok := f.scenes[name]
	return sc, ok, nil
}

func (f *fakeSceneStore) Put(scene scenes.Scene) error {
	if f.scenes == nil {
		f.scenes = map[string]scenes.Scene{}
	}
	f.put = scene
	f.scenes[scene.Name] = scene
	return nil
}

func (f *fakeSceneStore) Delete(name string) error {
	f.deleteCalls++
	f.deletedName = name
	delete(f.scenes, name)
	return nil
}

type fakeSceneTopologyGetter struct {
	top sonos.Topology
	err error
}

func (f *fakeSceneTopologyGetter) GetTopology(ctx context.Context) (sonos.Topology, error) {
	return f.top, f.err
}

type fakeSceneSpeaker struct {
	ip string

	volume int
	mute   bool

	leaveCalls int
	joinCalls  int
	joinUUID   string

	setVolCalls int
	setVolValue int

	setMuteCalls int
	setMuteValue bool
}

func (f *fakeSceneSpeaker) LeaveGroup(ctx context.Context) error {
	f.leaveCalls++
	return nil
}

func (f *fakeSceneSpeaker) JoinGroup(ctx context.Context, coordinatorUUID string) error {
	f.joinCalls++
	f.joinUUID = coordinatorUUID
	return nil
}

func (f *fakeSceneSpeaker) GetVolume(ctx context.Context) (int, error) { return f.volume, nil }
func (f *fakeSceneSpeaker) SetVolume(ctx context.Context, volume int) error {
	f.setVolCalls++
	f.setVolValue = volume
	return nil
}
func (f *fakeSceneSpeaker) GetMute(ctx context.Context) (bool, error) { return f.mute, nil }
func (f *fakeSceneSpeaker) SetMute(ctx context.Context, mute bool) error {
	f.setMuteCalls++
	f.setMuteValue = mute
	return nil
}

func TestSceneSaveCapturesGroupsAndDeviceAudio(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second}
	cmd := newSceneCmd(flags)

	top := sonos.Topology{
		Groups: []sonos.Group{
			{
				ID: "G1",
				Coordinator: sonos.Member{
					Name:          "Living Room",
					IP:            "192.168.1.10",
					UUID:          "RINCON_LR1400",
					IsCoordinator: true,
					IsVisible:     true,
				},
				Members: []sonos.Member{
					{Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400", IsCoordinator: true, IsVisible: true},
					{Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400", IsVisible: true},
					{Name: "Sub", IP: "192.168.1.12", UUID: "RINCON_SUB", IsVisible: false},
				},
			},
		},
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "Living Room", IP: "192.168.1.10", UUID: "RINCON_LR1400", IsVisible: true},
			"192.168.1.11": {Name: "Kitchen", IP: "192.168.1.11", UUID: "RINCON_K1400", IsVisible: true},
			"192.168.1.12": {Name: "Sub", IP: "192.168.1.12", UUID: "RINCON_SUB", IsVisible: false},
		},
	}

	store := &fakeSceneStore{}
	speakers := map[string]*fakeSceneSpeaker{
		"192.168.1.10": {ip: "192.168.1.10", volume: 10, mute: false},
		"192.168.1.11": {ip: "192.168.1.11", volume: 20, mute: true},
		"192.168.1.12": {ip: "192.168.1.12", volume: 1, mute: false},
	}

	origStore := newSceneStore
	origTG := newSceneTopologyGetter
	origClient := newSceneSpeakerClient
	t.Cleanup(func() {
		newSceneStore = origStore
		newSceneTopologyGetter = origTG
		newSceneSpeakerClient = origClient
	})

	newSceneStore = func() (scenes.Store, error) { return store, nil }
	newSceneTopologyGetter = func(ctx context.Context, timeout time.Duration) (sceneTopologyGetter, error) {
		return &fakeSceneTopologyGetter{top: top}, nil
	}
	newSceneSpeakerClient = func(ip string, timeout time.Duration) sceneSpeakerClient {
		s, ok := speakers[ip]
		if !ok {
			return &fakeSceneSpeaker{ip: ip}
		}
		return s
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"save", "Morning"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.put.Name != "Morning" {
		t.Fatalf("scene name: %q", store.put.Name)
	}
	if len(store.put.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(store.put.Groups))
	}
	// Invisible/bonded devices should not be included in scenes.
	if len(store.put.Devices) != 2 {
		t.Fatalf("expected 2 visible devices, got %d", len(store.put.Devices))
	}
}

func TestSceneApplyUngroupsJoinsAndRestoresAudio(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second}
	cmd := newSceneCmd(flags)

	scene := scenes.Scene{
		Name: "Work",
		Groups: []scenes.SceneGroup{
			{
				CoordinatorUUID: "RINCON_A1400",
				CoordinatorName: "A",
				MemberUUIDs:     []string{"RINCON_A1400", "RINCON_B1400", "RINCON_C_INVISIBLE"},
			},
		},
		Devices: []scenes.SceneDevice{
			{UUID: "RINCON_A1400", Name: "A", IP: "192.168.1.10", Volume: 5, Mute: false},
			{UUID: "RINCON_B1400", Name: "B", IP: "192.168.1.11", Volume: 15, Mute: true},
			{UUID: "RINCON_C_INVISIBLE", Name: "C", IP: "192.168.1.12", Volume: 1, Mute: false},
		},
	}

	top := sonos.Topology{
		ByIP: map[string]sonos.Member{
			"192.168.1.10": {Name: "A", IP: "192.168.1.10", UUID: "RINCON_A1400", IsVisible: true},
			"192.168.1.11": {Name: "B", IP: "192.168.1.11", UUID: "RINCON_B1400", IsVisible: true},
			"192.168.1.12": {Name: "C", IP: "192.168.1.12", UUID: "RINCON_C_INVISIBLE", IsVisible: false},
		},
	}

	store := &fakeSceneStore{scenes: map[string]scenes.Scene{"Work": scene}}
	speakers := map[string]*fakeSceneSpeaker{
		"192.168.1.10": {ip: "192.168.1.10"},
		"192.168.1.11": {ip: "192.168.1.11"},
		"192.168.1.12": {ip: "192.168.1.12"},
	}

	origStore := newSceneStore
	origTG := newSceneTopologyGetter
	origClient := newSceneSpeakerClient
	t.Cleanup(func() {
		newSceneStore = origStore
		newSceneTopologyGetter = origTG
		newSceneSpeakerClient = origClient
	})

	newSceneStore = func() (scenes.Store, error) { return store, nil }
	newSceneTopologyGetter = func(ctx context.Context, timeout time.Duration) (sceneTopologyGetter, error) {
		return &fakeSceneTopologyGetter{top: top}, nil
	}
	newSceneSpeakerClient = func(ip string, timeout time.Duration) sceneSpeakerClient {
		s := speakers[ip]
		if s == nil {
			return &fakeSceneSpeaker{ip: ip}
		}
		return s
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"apply", "Work"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both devices should be ungrouped once.
	if speakers["192.168.1.10"].leaveCalls != 1 {
		t.Fatalf("expected A leaveCalls=1, got %d", speakers["192.168.1.10"].leaveCalls)
	}
	if speakers["192.168.1.11"].leaveCalls != 1 {
		t.Fatalf("expected B leaveCalls=1, got %d", speakers["192.168.1.11"].leaveCalls)
	}

	// B should join coordinator A.
	if speakers["192.168.1.11"].joinCalls != 1 || speakers["192.168.1.11"].joinUUID != "RINCON_A1400" {
		t.Fatalf("expected B join to RINCON_A1400, got calls=%d uuid=%q", speakers["192.168.1.11"].joinCalls, speakers["192.168.1.11"].joinUUID)
	}

	// Audio restored.
	if speakers["192.168.1.10"].setVolCalls != 1 || speakers["192.168.1.10"].setVolValue != 5 {
		t.Fatalf("expected A setVol=5 once, got calls=%d val=%d", speakers["192.168.1.10"].setVolCalls, speakers["192.168.1.10"].setVolValue)
	}
	if speakers["192.168.1.11"].setMuteCalls != 1 || speakers["192.168.1.11"].setMuteValue != true {
		t.Fatalf("expected B setMute=true once, got calls=%d val=%v", speakers["192.168.1.11"].setMuteCalls, speakers["192.168.1.11"].setMuteValue)
	}
	// Invisible device should be ignored.
	if s := speakers["192.168.1.12"]; s != nil && (s.leaveCalls != 0 || s.joinCalls != 0 || s.setVolCalls != 0 || s.setMuteCalls != 0) {
		t.Fatalf("expected invisible device untouched, got %+v", s)
	}
}

func TestSceneApplyMissingScene(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second}
	cmd := newSceneCmd(flags)

	store := &fakeSceneStore{scenes: map[string]scenes.Scene{}}

	origStore := newSceneStore
	origTG := newSceneTopologyGetter
	t.Cleanup(func() {
		newSceneStore = origStore
		newSceneTopologyGetter = origTG
	})

	newSceneStore = func() (scenes.Store, error) { return store, nil }
	newSceneTopologyGetter = func(ctx context.Context, timeout time.Duration) (sceneTopologyGetter, error) {
		return nil, errors.New("should not be called")
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"apply", "Nope"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSceneListPlain(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatPlain}
	cmd := newSceneCmd(flags)

	store := &fakeSceneStore{scenes: map[string]scenes.Scene{
		"Morning": {Name: "Morning", CreatedAt: time.Date(2025, 12, 13, 0, 0, 0, 0, time.UTC)},
	}}

	origStore := newSceneStore
	t.Cleanup(func() { newSceneStore = origStore })
	newSceneStore = func() (scenes.Store, error) { return store, nil }

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.listCalls != 1 {
		t.Fatalf("expected listCalls=1, got %d", store.listCalls)
	}
	if !strings.Contains(out.String(), "Morning") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestSceneDeleteJSON(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newSceneCmd(flags)

	store := &fakeSceneStore{scenes: map[string]scenes.Scene{
		"Work": {Name: "Work"},
	}}

	origStore := newSceneStore
	t.Cleanup(func() { newSceneStore = origStore })
	newSceneStore = func() (scenes.Store, error) { return store, nil }

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"delete", "Work"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.deleteCalls != 1 || store.deletedName != "Work" {
		t.Fatalf("expected delete Work once, got calls=%d name=%q", store.deleteCalls, store.deletedName)
	}
	if !strings.Contains(out.String(), "\"action\": \"scene.delete\"") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
