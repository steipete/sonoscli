package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestCompleteSMAPIAuth_NoWaitAttemptsOnce(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := completeSMAPIAuth(context.Background(), 0, func(context.Context) (sonos.SMAPITokenPair, error) {
		calls++
		return sonos.SMAPITokenPair{}, errors.New("smapi fault: SOAP-ENV:Server: NOT_LINKED_RETRY")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestCompleteSMAPIAuth_WaitRetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := completeSMAPIAuth(context.Background(), 50*time.Millisecond, func(context.Context) (sonos.SMAPITokenPair, error) {
		calls++
		if calls < 3 {
			return sonos.SMAPITokenPair{}, errors.New("smapi fault: SOAP-ENV:Server: NOT_LINKED_RETRY")
		}
		return sonos.SMAPITokenPair{AuthToken: "t", PrivateKey: "k"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AuthToken != "t" || got.PrivateKey != "k" {
		t.Fatalf("unexpected token pair: %#v", got)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestCompleteSMAPIAuth_WaitStopsOnNonPendingError(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := completeSMAPIAuth(context.Background(), 50*time.Millisecond, func(context.Context) (sonos.SMAPITokenPair, error) {
		calls++
		return sonos.SMAPITokenPair{}, errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestIsSMAPIInvalidLinkCode(t *testing.T) {
	t.Parallel()

	if isSMAPIInvalidLinkCode(nil) {
		t.Fatalf("expected false for nil")
	}
	if !isSMAPIInvalidLinkCode(errors.New("smapi fault: SOAP-ENV:Server: Invalid linkCode details")) {
		t.Fatalf("expected true for invalid linkCode error")
	}
	if isSMAPIInvalidLinkCode(errors.New("smapi fault: SOAP-ENV:Server: NOT_LINKED_RETRY")) {
		t.Fatalf("expected false for not-linked error")
	}
}
