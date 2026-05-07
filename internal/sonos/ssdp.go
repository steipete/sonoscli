package sonos

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"
)

type ssdpResult struct {
	Location string
	USN      string
	ST       string
	Server   string
}

type ssdpUDPConn interface {
	WriteToUDP(b []byte, addr *net.UDPAddr) (int, error)
	ReadFromUDP(b []byte) (int, *net.UDPAddr, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

var ssdpListenUDP = func(network string, laddr *net.UDPAddr) (ssdpUDPConn, error) {
	return net.ListenUDP(network, laddr)
}

var ssdpNow = time.Now

func ssdpDiscover(ctx context.Context, timeout time.Duration) ([]ssdpResult, error) {
	// SSDP M-SEARCH for Sonos ZonePlayer devices.
	payload := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		`MAN: "ssdp:discover"`,
		"MX: 1",
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1",
		"", "",
	}, "\r\n")

	conn, err := ssdpListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	dst := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}

	// UDP is unreliable, send multiple times.
	for i := 0; i < 3; i++ {
		if _, err := conn.WriteToUDP([]byte(payload), dst); err != nil {
			return nil, err
		}
	}
	slog.Debug("ssdp: sent M-SEARCH", "dst", dst.String())

	deadline := ssdpNow().Add(timeout)
	byLocation := map[string]ssdpResult{}

	buf := make([]byte, 64*1024)
Loop:
	for !ssdpNow().After(deadline) {
		select {
		case <-ctx.Done():
			// Treat DeadlineExceeded like a normal timeout so callers can fall back.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				break Loop
			}
			return nil, ctx.Err()
		default:
		}

		_ = conn.SetReadDeadline(ssdpNow().Add(200 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				continue
			}
			// Some platforms can return spurious read errors while sockets are closing.
			break
		}
		msg := buf[:n]
		res, ok := parseSSDPResponse(msg)
		if !ok || res.Location == "" {
			continue
		}
		slog.Debug("ssdp: response", "location", res.Location, "usn", res.USN, "server", res.Server)
		byLocation[res.Location] = res
	}

	out := make([]ssdpResult, 0, len(byLocation))
	for _, v := range byLocation {
		out = append(out, v)
	}
	return out, nil
}

func parseSSDPResponse(b []byte) (ssdpResult, bool) {
	// SSDP responses are HTTP-like with CRLF line endings.
	s := bufio.NewScanner(bytes.NewReader(b))
	s.Split(bufio.ScanLines)

	// First line should be "HTTP/1.1 200 OK"
	if !s.Scan() {
		return ssdpResult{}, false
	}
	first := strings.TrimSpace(s.Text())
	if !strings.HasPrefix(first, "HTTP/") {
		return ssdpResult{}, false
	}

	headers := map[string]string{}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	return ssdpResult{
		Location: headers["location"],
		USN:      headers["usn"],
		ST:       headers["st"],
		Server:   headers["server"],
	}, true
}

func hostToIP(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("location host missing: %q", location)
	}
	return host, nil
}
