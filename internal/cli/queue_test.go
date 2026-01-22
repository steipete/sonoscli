package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

type fakeQueueClient struct {
	page         sonos.QueuePage
	listCalls    int
	clearCalls   int
	removeCalls  int
	playCalls    int
	lastPosition int
	err          error
}

func (f *fakeQueueClient) ListQueue(ctx context.Context, start, count int) (sonos.QueuePage, error) {
	f.listCalls++
	return f.page, f.err
}

func (f *fakeQueueClient) ClearQueue(ctx context.Context) error {
	f.clearCalls++
	return f.err
}

func (f *fakeQueueClient) RemoveQueuePosition(ctx context.Context, position int) error {
	f.removeCalls++
	f.lastPosition = position
	return f.err
}

func (f *fakeQueueClient) PlayQueuePosition(ctx context.Context, position int) error {
	f.playCalls++
	f.lastPosition = position
	return f.err
}

func TestQueueListRequiresTarget(t *testing.T) {
	flags := &rootFlags{Timeout: 2 * time.Second}
	cmd := newQueueListCmd(flags)
	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestQueueListPrintsTable(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newQueueListCmd(flags)

	orig := newQueueClient
	t.Cleanup(func() { newQueueClient = orig })

	fc := &fakeQueueClient{
		page: sonos.QueuePage{
			Items: []sonos.QueueItem{
				{Position: 1, Item: sonos.DIDLItem{Title: "Song 1", URI: "x://1"}},
			},
		},
	}
	newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) { return fc, nil }

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "POS") || !strings.Contains(out.String(), "Song 1") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if fc.listCalls != 1 {
		t.Fatalf("expected list call")
	}
}

func TestQueuePlayCallsClient(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newQueuePlayCmd(flags)

	orig := newQueueClient
	t.Cleanup(func() { newQueueClient = orig })

	fc := &fakeQueueClient{}
	newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) { return fc, nil }

	cmd.SetArgs([]string{"3"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.playCalls != 1 || fc.lastPosition != 3 {
		t.Fatalf("unexpected calls: %+v", fc)
	}
}

func TestQueueRemoveCallsClient(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newQueueRemoveCmd(flags)

	orig := newQueueClient
	t.Cleanup(func() { newQueueClient = orig })

	fc := &fakeQueueClient{}
	newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) { return fc, nil }

	cmd.SetArgs([]string{"2"})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.removeCalls != 1 || fc.lastPosition != 2 {
		t.Fatalf("unexpected calls: %+v", fc)
	}
}

func TestQueueClearCallsClient(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newQueueClearCmd(flags)

	orig := newQueueClient
	t.Cleanup(func() { newQueueClient = orig })

	fc := &fakeQueueClient{}
	newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) { return fc, nil }

	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.clearCalls != 1 {
		t.Fatalf("expected clear call")
	}
}

func TestQueueClientErrorPropagates(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newQueueClearCmd(flags)

	orig := newQueueClient
	t.Cleanup(func() { newQueueClient = orig })

	fc := &fakeQueueClient{err: errors.New("boom")}
	newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) { return fc, nil }

	cmd.SetArgs([]string{})
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}
