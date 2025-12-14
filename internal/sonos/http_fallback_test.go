package sonos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

type timeoutRoundTripper struct{}

func (timeoutRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

type errorRoundTripper struct {
	err error
}

func (e errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestFetchDeviceDescription_CurlFallbackOnTimeout(t *testing.T) {
	orig := curlRoundTripFunc
	t.Cleanup(func() { curlRoundTripFunc = orig })

	called := false
	curlRoundTripFunc = func(_ context.Context, req *http.Request, _ time.Duration) (*http.Response, error) {
		called = true
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		xml := `<?xml version="1.0" encoding="utf-8"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <manufacturer>Sonos</manufacturer>
    <roomName>Office</roomName>
    <UDN>uuid:RINCON_123</UDN>
  </device>
</root>`
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(stringsReader(xml)),
			Request:    req,
		}, nil
	}

	hc := &http.Client{Transport: timeoutRoundTripper{}, Timeout: 200 * time.Millisecond}
	name, udn, ip, err := fetchDeviceDescription(context.Background(), hc, "http://192.168.0.21:1400/xml/device_description.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected curl fallback to be used")
	}
	if name != "Office" {
		t.Fatalf("name: got %q, want %q", name, "Office")
	}
	if udn != "RINCON_123" {
		t.Fatalf("udn: got %q, want %q", udn, "RINCON_123")
	}
	if ip != "192.168.0.21" {
		t.Fatalf("ip: got %q, want %q", ip, "192.168.0.21")
	}
}

func TestSoapCall_CurlFallbackOnTimeout(t *testing.T) {
	orig := curlRoundTripFunc
	t.Cleanup(func() { curlRoundTripFunc = orig })

	curlRoundTripFunc = func(_ context.Context, req *http.Request, _ time.Duration) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if got := req.Header.Get("SOAPACTION"); got == "" {
			t.Fatalf("expected SOAPACTION header")
		}
		if got := req.Header.Get("Content-Type"); got == "" {
			t.Fatalf("expected Content-Type header")
		}
		b, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if !bytes.Contains(b, []byte("<u:GetZoneGroupState")) {
			t.Fatalf("expected soap body to include action, got %q", string(b))
		}

		resp := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1">
      <ZoneGroupState>ZGS</ZoneGroupState>
    </u:GetZoneGroupStateResponse>
  </s:Body>
</s:Envelope>`
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(stringsReader(resp)),
			Request:    req,
		}, nil
	}

	hc := &http.Client{Transport: timeoutRoundTripper{}, Timeout: 200 * time.Millisecond}
	out, err := soapCall(context.Background(), hc, "http://192.168.0.21:1400/ZoneGroupTopology/Control", urnZoneGroupTopology, "GetZoneGroupState", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out["ZoneGroupState"]; got != "ZGS" {
		t.Fatalf("ZoneGroupState: got %q, want %q", got, "ZGS")
	}
}

func TestDoRequest_NoFallbackForPublicIP(t *testing.T) {
	orig := curlRoundTripFunc
	t.Cleanup(func() { curlRoundTripFunc = orig })

	curlRoundTripFunc = func(context.Context, *http.Request, time.Duration) (*http.Response, error) {
		t.Fatalf("curl fallback should not be used for public IP")
		return nil, nil
	}

	hc := &http.Client{Transport: timeoutRoundTripper{}, Timeout: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://8.8.8.8/", nil)
	_, err := doRequest(context.Background(), hc, req)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDoRequest_NoFallbackForNonTimeoutError(t *testing.T) {
	orig := curlRoundTripFunc
	t.Cleanup(func() { curlRoundTripFunc = orig })

	curlRoundTripFunc = func(context.Context, *http.Request, time.Duration) (*http.Response, error) {
		t.Fatalf("curl fallback should not be used for non-timeout errors")
		return nil, nil
	}

	hc := &http.Client{Transport: errorRoundTripper{err: errors.New("connection refused")}, Timeout: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://192.168.0.21:1400/", nil)
	_, err := doRequest(context.Background(), hc, req)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseCurlResponse_ParsesHeadersAndBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://192.168.0.21:1400/", nil)
	raw := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nX-Test: 1\r\n\r\nhello")
	resp, err := parseCurlResponse(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode: got %d, want %d", resp.StatusCode, 200)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/plain" {
		t.Fatalf("Content-Type: got %q, want %q", got, "text/plain")
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(b) != "hello" {
		t.Fatalf("body: got %q, want %q", string(b), "hello")
	}
}

func TestParseCurlResponse_SkipsInterim100(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://192.168.0.21:1400/", nil)
	raw := []byte("HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nok")
	resp, err := parseCurlResponse(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode: got %d, want %d", resp.StatusCode, 200)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(b) != "ok" {
		t.Fatalf("body: got %q, want %q", string(b), "ok")
	}
}

func stringsReader(s string) io.Reader {
	return bytes.NewReader([]byte(s))
}
