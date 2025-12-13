package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func soapFaultWithUPnPCode(code string) string {
	return `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body><s:Fault>` +
		`<faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring>` +
		`<detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0">` +
		`<errorCode>` + code + `</errorCode><errorDescription>err</errorDescription>` +
		`</UPnPError></detail>` +
		`</s:Fault></s:Body></s:Envelope>`
}

func okSOAPResponse(action string) string {
	return `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body><u:` + action + `Response xmlns:u="` + urnAVTransport + `"></u:` + action + `Response></s:Body></s:Envelope>`
}

func TestPreviousOrRestartFallsBackToSeekOn711(t *testing.T) {
	var calls []string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		action := req.Header.Get("SOAPACTION")
		calls = append(calls, action)
		switch {
		case strings.Contains(action, "#Previous"):
			body := soapFaultWithUPnPCode("711")
			return &http.Response{
				StatusCode: 500,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		case strings.Contains(action, "#Seek"):
			// Ensure this is the restart seek.
			raw, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(raw), "<Unit>REL_TIME</Unit>") || !strings.Contains(string(raw), "<Target>0:00:00</Target>") {
				t.Fatalf("unexpected seek body: %s", string(raw))
			}
			body := okSOAPResponse("Seek")
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		default:
			t.Fatalf("unexpected action: %s", action)
			return nil, nil
		}
	})
	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{
			Transport: rt,
			Timeout:   2 * time.Second,
		},
	}

	if err := c.PreviousOrRestart(context.Background()); err != nil {
		t.Fatalf("PreviousOrRestart: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (Previous, Seek), got %d: %#v", len(calls), calls)
	}
}

func TestPreviousOrRestartReturnsErrorForOtherCodes(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		action := req.Header.Get("SOAPACTION")
		if strings.Contains(action, "#Previous") {
			body := soapFaultWithUPnPCode("710")
			return &http.Response{
				StatusCode: 500,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}
		t.Fatalf("unexpected action: %s", action)
		return nil, nil
	})
	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{
			Transport: rt,
			Timeout:   2 * time.Second,
		},
	}

	if err := c.PreviousOrRestart(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
