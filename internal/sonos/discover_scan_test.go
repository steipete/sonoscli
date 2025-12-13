package sonos

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeAddr struct{}

func (a fakeAddr) Network() string { return "fake" }
func (a fakeAddr) String() string  { return "fake" }

func TestLocalIPv4AddrsFiltersInterfacesAndAddresses(t *testing.T) {
	origIfaces := netInterfacesFunc
	origAddrs := ifaceAddrsFunc
	t.Cleanup(func() {
		netInterfacesFunc = origIfaces
		ifaceAddrsFunc = origAddrs
	})

	netInterfacesFunc = func() ([]net.Interface, error) {
		return []net.Interface{
			{Index: 1, Flags: 0},                             // down
			{Index: 2, Flags: net.FlagUp | net.FlagLoopback}, // loopback
			{Index: 3, Flags: net.FlagUp},                    // ok
			{Index: 4, Flags: net.FlagUp},                    // ok but Addrs errors
		}, nil
	}
	ifaceAddrsFunc = func(iface net.Interface) ([]net.Addr, error) {
		switch iface.Index {
		case 3:
			return []net.Addr{
				&net.IPNet{IP: net.IPv4(10, 0, 0, 5), Mask: net.CIDRMask(24, 32)},
				&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
				fakeAddr{},
			}, nil
		case 4:
			return nil, errors.New("boom")
		default:
			return nil, nil
		}
	}

	ips, err := localIPv4Addrs()
	if err != nil {
		t.Fatalf("localIPv4Addrs: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.IPv4(10, 0, 0, 5)) {
		t.Fatalf("unexpected ips: %#v", ips)
	}
}

func TestScanAnySpeakerIPFindsSpeaker(t *testing.T) {
	origLocal := localIPv4AddrsFunc
	origPort := isPortOpenFunc
	origFetch := fetchDeviceDescriptionFunc
	t.Cleanup(func() {
		localIPv4AddrsFunc = origLocal
		isPortOpenFunc = origPort
		fetchDeviceDescriptionFunc = origFetch
	})

	localIPv4AddrsFunc = func() ([]net.IP, error) {
		return []net.IP{net.IPv4(192, 168, 50, 10)}, nil
	}

	wantIP := "192.168.50.123"
	isPortOpenFunc = func(ip string, port int, timeout time.Duration) bool {
		return ip == wantIP
	}

	fetchDeviceDescriptionFunc = func(ctx context.Context, _ *http.Client, location string) (string, string, string, error) {
		if strings.Contains(location, wantIP) {
			return "Office", "uuid:RINCON_OFFICE1400", wantIP, nil
		}
		return "", "", "", errors.New("not a sonos speaker")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	got, err := scanAnySpeakerIP(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("scanAnySpeakerIP: %v", err)
	}
	if got != wantIP {
		t.Fatalf("expected %q, got %q", wantIP, got)
	}
}

func TestScanAnySpeakerIPNoLocalAddrsErrors(t *testing.T) {
	origLocal := localIPv4AddrsFunc
	t.Cleanup(func() { localIPv4AddrsFunc = origLocal })

	localIPv4AddrsFunc = func() ([]net.IP, error) { return nil, nil }

	_, err := scanAnySpeakerIP(context.Background(), 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error")
	}
}
