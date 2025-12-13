package sonos

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DiscoverOptions struct {
	Timeout          time.Duration
	IncludeInvisible bool
}

var (
	ssdpDiscoverFunc              = ssdpDiscover
	scanAnySpeakerIPFunc          = scanAnySpeakerIP
	discoverViaTopologyFunc       = discoverViaTopology
	discoverViaTopologyFromIPFunc = discoverViaTopologyFromIP
	fetchDeviceDescriptionFunc    = fetchDeviceDescription
	newClientForDiscover          = NewClient
	localIPv4AddrsFunc            = localIPv4Addrs
	isPortOpenFunc                = isPortOpen
	netInterfacesFunc             = net.Interfaces
	ifaceAddrsFunc                = func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }
)

func Discover(ctx context.Context, opts DiscoverOptions) ([]Device, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	slog.Debug("discover: start", "timeout", timeout.String(), "includeInvisible", opts.IncludeInvisible)

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ssdpTimeout := 1500 * time.Millisecond
	if timeout <= 2*time.Second {
		ssdpTimeout = timeout / 2
	}
	if ssdpTimeout <= 0 || ssdpTimeout > timeout {
		ssdpTimeout = timeout
	}

	ssdpCtx, cancelSSDP := context.WithTimeout(opCtx, ssdpTimeout)
	ssdpResults, err := ssdpDiscoverFunc(ssdpCtx, ssdpTimeout)
	cancelSSDP()
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	slog.Debug("discover: ssdp finished", "timeout", ssdpTimeout.String(), "results", len(ssdpResults), "err", errString(err))

	// Prefer the topology-based approach (query one speaker for the full list),
	// since not every speaker will reliably respond to SSDP M-SEARCH.
	out, err := discoverViaTopologyFunc(opCtx, timeout, ssdpResults, opts.IncludeInvisible)
	if err == nil && len(out) > 0 {
		slog.Debug("discover: topology via ssdp candidates succeeded", "devices", len(out))
		return out, nil
	}
	slog.Debug("discover: topology via ssdp candidates failed", "err", errString(err))

	// SSDP sometimes fails or returns incomplete results on certain networks.
	// Fall back to finding any reachable Sonos speaker, then query topology.
	if anyIP, scanErr := scanAnySpeakerIPFunc(opCtx, timeout); scanErr == nil && anyIP != "" {
		slog.Debug("discover: subnet scan found a speaker", "ip", anyIP)
		out, topErr := discoverViaTopologyFromIPFunc(opCtx, timeout, anyIP, opts.IncludeInvisible)
		if topErr == nil && len(out) > 0 {
			slog.Debug("discover: topology via scanned speaker succeeded", "devices", len(out))
			return out, nil
		}
		slog.Debug("discover: topology via scanned speaker failed", "ip", anyIP, "err", errString(topErr))
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

		name, udn, ip, err := fetchDeviceDescriptionFunc(opCtx, httpClient, location)
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
	c := newClientForDiscover(ip, timeout)
	top, err := c.GetTopology(ctx)
	if err != nil {
		slog.Debug("discover: GetTopology failed", "ip", ip, "err", errString(err))
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

	slog.Debug("discover: querying topology candidates", "candidates", len(candidates))

	// Query multiple candidates and keep the best (largest) result, to account
	// for devices that may return incomplete topology snapshots.
	deadline := time.Now().Add(timeout)
	bestByIP := map[string]Device{}
	for _, ip := range candidates {
		if time.Now().After(deadline) {
			break
		}
		c := newClientForDiscover(ip, timeout)
		top, err := c.GetTopology(ctx)
		if err != nil {
			slog.Debug("discover: topology candidate failed", "ip", ip, "err", errString(err))
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

		bestByIP = preferDeviceSet(bestByIP, byIP)
	}

	if len(bestByIP) > 0 {
		slog.Debug("discover: topology candidates result", "devices", len(bestByIP))
		return sortDevices(bestByIP), nil
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

func preferDeviceSet(best, candidate map[string]Device) map[string]Device {
	if len(candidate) == 0 {
		return best
	}
	if len(candidate) > len(best) {
		return cloneDeviceMap(candidate)
	}
	if len(candidate) == len(best) {
		out := cloneDeviceMap(best)
		for k, v := range candidate {
			if _, ok := out[k]; !ok {
				out[k] = v
			}
		}
		return out
	}
	return best
}

func cloneDeviceMap(in map[string]Device) map[string]Device {
	out := make(map[string]Device, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func scanAnySpeakerIP(ctx context.Context, timeout time.Duration) (string, error) {
	addrs, err := localIPv4AddrsFunc()
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", errors.New("no local IPv4 addresses found")
	}
	slog.Debug("discover: subnet scan start", "interfaces", len(addrs))

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

				if !isPortOpenFunc(ip, 1400, 250*time.Millisecond) {
					continue
				}
				location := fmt.Sprintf("http://%s:1400/xml/device_description.xml", ip)
				_, _, _, err := fetchDeviceDescriptionFunc(ctx, httpClient, location)
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
		slog.Debug("discover: subnet scan found speaker", "ip", ip)
		return ip, nil
	default:
		return "", errors.New("no sonos speakers found on local subnets")
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func localIPv4Addrs() ([]net.IP, error) {
	ifaces, err := netInterfacesFunc()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, iface := range ifaces {
		// Skip down interfaces and loopback.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifaceAddrsFunc(iface)
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
	conn, err := d.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
