package sonos

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	IP   string
	HTTP *http.Client
}

func NewClient(ip string, timeout time.Duration) *Client {
	return &Client{
		IP:   ip,
		HTTP: defaultHTTPClient(timeout),
	}
}

func (c *Client) baseURL() string {
	return fmt.Sprintf("http://%s:1400", c.IP)
}

func (c *Client) soapCall(ctx context.Context, controlPath, serviceURN, action string, args map[string]string) (map[string]string, error) {
	return soapCall(ctx, c.HTTP, c.baseURL()+controlPath, serviceURN, action, args)
}
