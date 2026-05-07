package sonos

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Subscription struct {
	SID     string
	Timeout time.Duration
	URL     string
}

func parseSecondTimeout(h string) (time.Duration, bool) {
	if h == "" {
		return 0, false
	}
	if strings.EqualFold(h, "infinite") {
		return 0, true
	}
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(strings.ToLower(h), "second-")
	secs, err := strconv.Atoi(h)
	if err != nil {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

func (c *Client) Subscribe(ctx context.Context, eventPath string, callbackURL string, requestedTimeout time.Duration) (Subscription, error) {
	req, err := http.NewRequestWithContext(ctx, "SUBSCRIBE", c.baseURL()+eventPath, nil)
	if err != nil {
		return Subscription{}, err
	}
	req.Header.Set("CALLBACK", "<"+callbackURL+">")
	req.Header.Set("NT", "upnp:event")
	if requestedTimeout > 0 {
		req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", int(requestedTimeout.Seconds())))
	}

	resp, err := c.HTTP.Do(req) //nolint:gosec // Sonos event URL targets the selected local speaker.
	if err != nil {
		return Subscription{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Subscription{}, fmt.Errorf("subscribe failed: %s", resp.Status)
	}

	sid := strings.TrimSpace(resp.Header.Get("SID"))
	if sid == "" {
		sid = strings.TrimSpace(resp.Header.Get("Sid"))
	}
	if sid == "" {
		return Subscription{}, fmt.Errorf("subscribe response missing SID header")
	}

	to, _ := parseSecondTimeout(resp.Header.Get("TIMEOUT"))

	return Subscription{
		SID:     sid,
		Timeout: to,
		URL:     c.baseURL() + eventPath,
	}, nil
}

func (c *Client) Renew(ctx context.Context, sub Subscription, requestedTimeout time.Duration) (Subscription, error) {
	req, err := http.NewRequestWithContext(ctx, "SUBSCRIBE", sub.URL, nil)
	if err != nil {
		return Subscription{}, err
	}
	req.Header.Set("SID", sub.SID)
	if requestedTimeout > 0 {
		req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", int(requestedTimeout.Seconds())))
	}

	resp, err := c.HTTP.Do(req) //nolint:gosec // Sonos event URL targets an existing local subscription.
	if err != nil {
		return Subscription{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Subscription{}, fmt.Errorf("renew failed: %s", resp.Status)
	}

	to, _ := parseSecondTimeout(resp.Header.Get("TIMEOUT"))
	sub.Timeout = to
	return sub, nil
}

func (c *Client) Unsubscribe(ctx context.Context, sub Subscription) error {
	req, err := http.NewRequestWithContext(ctx, "UNSUBSCRIBE", sub.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("SID", sub.SID)
	resp, err := c.HTTP.Do(req) //nolint:gosec // Sonos event URL targets an existing local subscription.
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusPreconditionFailed {
		// Precondition Failed: speaker rebooted or already unsubscribed. Treat as success.
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unsubscribe failed: %s", resp.Status)
	}
	return nil
}

func (c *Client) SubscribeAVTransport(ctx context.Context, callbackURL string, requestedTimeout time.Duration) (Subscription, error) {
	return c.Subscribe(ctx, eventAVTransport, callbackURL, requestedTimeout)
}

func (c *Client) SubscribeRenderingControl(ctx context.Context, callbackURL string, requestedTimeout time.Duration) (Subscription, error) {
	return c.Subscribe(ctx, eventRenderingControl, callbackURL, requestedTimeout)
}
