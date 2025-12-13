package sonos

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Device struct {
	IP       string `json:"ip"`
	Name     string `json:"name"`
	UDN      string `json:"udn"`
	Location string `json:"location"`
}

type deviceDescription struct {
	Device struct {
		DeviceType   string `xml:"deviceType"`
		RoomName     string `xml:"roomName"`
		Manufacturer string `xml:"manufacturer"`
		UDN          string `xml:"UDN"`
	} `xml:"device"`
}

func fetchDeviceDescription(ctx context.Context, httpClient *http.Client, locationURL string) (name, udn, ip string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, locationURL, nil)
	if err != nil {
		return "", "", "", err
	}
	resp, err := doRequest(ctx, httpClient, req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", "", "", fmt.Errorf("device description: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", "", "", err
	}

	var dd deviceDescription
	if err := xml.Unmarshal(b, &dd); err != nil {
		return "", "", "", err
	}

	// Filter out non-Sonos UPnP devices that might respond to our SSDP search.
	deviceType := strings.TrimSpace(dd.Device.DeviceType)
	manufacturer := strings.TrimSpace(dd.Device.Manufacturer)
	if deviceType != "urn:schemas-upnp-org:device:ZonePlayer:1" && !strings.Contains(strings.ToLower(manufacturer), "sonos") {
		return "", "", "", fmt.Errorf("not a sonos ZonePlayer (deviceType=%q manufacturer=%q)", deviceType, manufacturer)
	}

	name = strings.TrimSpace(dd.Device.RoomName)
	udn = strings.TrimPrefix(strings.TrimSpace(dd.Device.UDN), "uuid:")

	ip, err = hostToIP(locationURL)
	if err != nil {
		return "", "", "", err
	}
	return name, udn, ip, nil
}

func defaultHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	proxyFromEnv := http.ProxyFromEnvironment
	tr := &http.Transport{
		// Avoid routing local Sonos traffic via HTTP proxy env vars.
		Proxy: func(req *http.Request) (*url.URL, error) {
			host := req.URL.Hostname()
			if ip := net.ParseIP(host); ip != nil {
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
					return nil, nil
				}
			}
			return proxyFromEnv(req)
		},

		DialContext:         dialer.DialContext,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}
}

func (c *Client) GetDeviceDescription(ctx context.Context) (Device, error) {
	location := c.baseURL() + "/xml/device_description.xml"
	name, udn, ip, err := fetchDeviceDescription(ctx, c.HTTP, location)
	if err != nil {
		return Device{}, err
	}
	return Device{
		IP:       ip,
		Name:     name,
		UDN:      udn,
		Location: location,
	}, nil
}
