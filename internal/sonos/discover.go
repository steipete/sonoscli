package sonos

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type DiscoverOptions struct {
	Timeout          time.Duration
	IncludeInvisible bool
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

	// Prefer the topology-based approach (query one speaker for the full list),
	// since not every speaker will reliably respond to SSDP M-SEARCH.
	out, err := discoverViaTopology(ctx, timeout, ssdpResults, opts.IncludeInvisible)
	if err == nil && len(out) > 0 {
		return out, nil
	}

	// SSDP sometimes fails or returns incomplete results on certain networks.
	// Fall back to finding any reachable Sonos speaker, then query topology.
	if anyIP, scanErr := scanAnySpeakerIP(ctx, timeout); scanErr == nil && anyIP != "" {
		out, topErr := discoverViaTopologyFromIP(ctx, timeout, anyIP, opts.IncludeInvisible)
		if topErr == nil && len(out) > 0 {
			return out, nil
		}
	}

	// Fallback: resolve each SSDP response directly.
	httpClient := defaultHTTPClient(timeout)
	byIP := map[string]Device{}
	for _, r := range ssdpResults {
		location := r.Location
		if location == "" {
			continue
		}
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

	return sortDevices(byIP), nil
}

func discoverViaTopologyFromIP(ctx context.Context, timeout time.Duration, ip string, includeInvisible bool) ([]Device, error) {
	c := NewClient(ip, timeout)
	top, err := c.GetTopology(ctx)
	if err != nil {
		return nil, err
	}

	byIP := map[string]Device{}
	for _, g := range top.Groups {
		for _, m := range g.Members {
			if !includeInvisible && !m.IsVisible {
				continue
			}
			name := strings.TrimSpace(m.Name)
			if name == "" {
				name = m.IP
			}
			byIP[m.IP] = Device{
				IP:       m.IP,
				Name:     name,
				UDN:      m.UUID,
				Location: m.Location,
			}
		}
	}

	return sortDevices(byIP), nil
}

func discoverViaTopology(ctx context.Context, timeout time.Duration, results []ssdpResult, includeInvisible bool) ([]Device, error) {
	candidates := make([]string, 0, len(results))
	for _, r := range results {
		if r.Location == "" {
			continue
		}
		ip, err := hostToIP(r.Location)
		if err != nil || ip == "" {
			continue
		}
		candidates = append(candidates, ip)
	}
	if len(candidates) == 0 {
		return nil, errors.New("no ssdp candidates")
	}

	for _, ip := range candidates {
		c := NewClient(ip, timeout)
		top, err := c.GetTopology(ctx)
		if err != nil {
			continue
		}

		byIP := map[string]Device{}
		for _, g := range top.Groups {
			for _, m := range g.Members {
				if !includeInvisible && !m.IsVisible {
					continue
				}
				name := strings.TrimSpace(m.Name)
				if name == "" {
					name = m.IP
				}
				byIP[m.IP] = Device{
					IP:       m.IP,
					Name:     name,
					UDN:      m.UUID,
					Location: m.Location,
				}
			}
		}

		return sortDevices(byIP), nil
	}

	return nil, errors.New("topology discovery failed")
}

func sortDevices(byIP map[string]Device) []Device {
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
	return out
}

func scanLocalSubnets(ctx context.Context, timeout time.Duration, includeInvisible bool) ([]Device, error) {
	addrs, err := localIPv4Addrs()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("no local IPv4 addresses found")
	}

	httpClient := defaultHTTPClient(timeout)

	byIP := map[string]Device{}
	var byIPMu sync.Mutex

	candidateIPs := make(chan string, 1024)
	workers := 128
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for ip := range candidateIPs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if !isPortOpen(ip, 1400, 350*time.Millisecond) {
					continue
				}

				location := fmt.Sprintf("http://%s:1400/xml/device_description.xml", ip)
				name, udn, _, err := fetchDeviceDescription(ctx, httpClient, location)
				if err != nil {
					continue
				}
				if name == "" {
					name = ip
				}
				if !includeInvisible {
					// Best-effort: we don't have visibility info without topology. Keep it.
				}

				byIPMu.Lock()
				byIP[ip] = Device{
					IP:       ip,
					Name:     name,
					UDN:      udn,
					Location: location,
				}
				byIPMu.Unlock()
			}
		}()
	}

	// Enumerate each /24 for each interface address.
	seen := map[string]struct{}{}
	for _, ip := range addrs {
		prefix := ipTo24Prefix(ip)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}

		for host := 1; host <= 254; host++ {
			candidateIPs <- fmt.Sprintf("%s.%d", prefix, host)
		}
	}

	close(candidateIPs)
	wg.Wait()

	return sortDevices(byIP), nil
}

func scanAnySpeakerIP(ctx context.Context, timeout time.Duration) (string, error) {
	addrs, err := localIPv4Addrs()
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", errors.New("no local IPv4 addresses found")
	}

	// Keep per-IP operations quick; we only need one match.
	httpClient := defaultHTTPClient(2 * time.Second)

	candidateIPs := make(chan string, 1024)
	found := make(chan string, 1)

	workers := 128
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for ip := range candidateIPs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if !isPortOpen(ip, 1400, 250*time.Millisecond) {
					continue
				}
				location := fmt.Sprintf("http://%s:1400/xml/device_description.xml", ip)
				_, _, _, err := fetchDeviceDescription(ctx, httpClient, location)
				if err != nil {
					continue
				}

				select {
				case found <- ip:
				default:
				}
				return
			}
		}()
	}

	seen := map[string]struct{}{}
	for _, ip := range addrs {
		prefix := ipTo24Prefix(ip)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}

		for host := 1; host <= 254; host++ {
			select {
			case candidateIPs <- fmt.Sprintf("%s.%d", prefix, host):
			case ip := <-found:
				close(candidateIPs)
				wg.Wait()
				return ip, nil
			case <-ctx.Done():
				close(candidateIPs)
				wg.Wait()
				return "", ctx.Err()
			}
		}
	}

	close(candidateIPs)
	wg.Wait()

	select {
	case ip := <-found:
		return ip, nil
	default:
		return "", errors.New("no sonos speakers found on local subnets")
	}
}

func localIPv4Addrs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, iface := range ifaces {
		// Skip down interfaces and loopback.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			ips = append(ips, ip4)
		}
	}
	return ips, nil
}

func ipTo24Prefix(ip net.IP) string {
	ip4 := ip.To4()
	if ip4 == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", ip4[0], ip4[1], ip4[2])
}

func isPortOpen(ip string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
