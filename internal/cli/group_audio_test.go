package cli

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeGroupAudioClient struct {
	groupVolume int
	groupMute   bool

	getVolCalls int
	setVolCalls int
	setVolValue int

	getMuteCalls int
	setMuteCalls int
	setMuteValue bool
}

func (f *fakeGroupAudioClient) GetGroupVolume(ctx context.Context) (int, error) {
	f.getVolCalls++
	return f.groupVolume, nil
}

func (f *fakeGroupAudioClient) SetGroupVolume(ctx context.Context, volume int) error {
	f.setVolCalls++
	f.setVolValue = volume
	return nil
}

func (f *fakeGroupAudioClient) GetGroupMute(ctx context.Context) (bool, error) {
	f.getMuteCalls++
	return f.groupMute, nil
}

func (f *fakeGroupAudioClient) SetGroupMute(ctx context.Context, mute bool) error {
	f.setMuteCalls++
	f.setMuteValue = mute
	return nil
}

func TestGroupVolumeGetPlain(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newGroupVolumeCmd(flags)

	fake := &fakeGroupAudioClient{groupVolume: 42}
	orig := newGroupAudioClient
	t.Cleanup(func() { newGroupAudioClient = orig })
	newGroupAudioClient = func(ctx context.Context, flags *rootFlags) (groupAudioClient, error) {
		return fake, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"get"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "42" {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if fake.getVolCalls != 1 {
		t.Fatalf("expected 1 get call, got %d", fake.getVolCalls)
	}
}

func TestGroupVolumeGetJSON(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newGroupVolumeCmd(flags)

	fake := &fakeGroupAudioClient{groupVolume: 7}
	orig := newGroupAudioClient
	t.Cleanup(func() { newGroupAudioClient = orig })
	newGroupAudioClient = func(ctx context.Context, flags *rootFlags) (groupAudioClient, error) {
		return fake, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"get"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "\"volume\": 7") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestGroupVolumeSetCallsClient(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newGroupVolumeCmd(flags)

	fake := &fakeGroupAudioClient{}
	orig := newGroupAudioClient
	t.Cleanup(func() { newGroupAudioClient = orig })
	newGroupAudioClient = func(ctx context.Context, flags *rootFlags) (groupAudioClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"set", "33"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.setVolCalls != 1 || fake.setVolValue != 33 {
		t.Fatalf("expected set volume 33 once, got calls=%d val=%d", fake.setVolCalls, fake.setVolValue)
	}
}

func TestGroupMuteToggle(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newGroupMuteCmd(flags)

	fake := &fakeGroupAudioClient{groupMute: true}
	orig := newGroupAudioClient
	t.Cleanup(func() { newGroupAudioClient = orig })
	newGroupAudioClient = func(ctx context.Context, flags *rootFlags) (groupAudioClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"toggle"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.getMuteCalls != 1 {
		t.Fatalf("expected 1 get mute call, got %d", fake.getMuteCalls)
	}
	if fake.setMuteCalls != 1 || fake.setMuteValue != false {
		t.Fatalf("expected set mute to false once, got calls=%d val=%v", fake.setMuteCalls, fake.setMuteValue)
	}
}
