package sonos

import (
	"context"
	"net/url"
	"sort"
	"time"
)

type DiscoverOptions struct {
	Timeout time.Duration
}

func Discover(ctx context.Context, opts DiscoverOptions) ([]Device, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ssdpResults, err := ssdpDiscover(ctx, timeout)
	if err != nil {
		return nil, err
	}

	httpClient := defaultHTTPClient(timeout)

	byIP := map[string]Device{}
	for _, r := range ssdpResults {
		location := r.Location
		if location == "" {
			continue
		}
		// Ensure the location URL is well-formed.
		if _, err := url.Parse(location); err != nil {
			continue
		}

		name, udn, ip, err := fetchDeviceDescription(ctx, httpClient, location)
		if err != nil {
			continue
		}
		if ip == "" {
			continue
		}
		if name == "" {
			name = ip
		}
		byIP[ip] = Device{
			IP:       ip,
			Name:     name,
			UDN:      udn,
			Location: location,
		}
	}

	out := make([]Device, 0, len(byIP))
	for _, d := range byIP {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].IP < out[j].IP
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
