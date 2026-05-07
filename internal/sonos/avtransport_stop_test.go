package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStopOrNoopTreats701AsSuccess(t *testing.T) {
	var calls []string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		action := req.Header.Get("SOAPACTION")
		calls = append(calls, action)
		if strings.Contains(action, "#Stop") {
			body := soapFaultWithUPnPCode("701")
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
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

	if err := c.StopOrNoop(context.Background()); err != nil {
		t.Fatalf("StopOrNoop: %v", err)
	}
	if len(calls) != 1 || !strings.Contains(calls[0], "#Stop") {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}
