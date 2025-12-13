package sonos

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	IP   string
	Port int
	HTTP *http.Client
}

func NewClient(ip string, timeout time.Duration) *Client {
	return &Client{
		IP:   ip,
		Port: 1400,
		HTTP: defaultHTTPClient(timeout),
	}
}

func (c *Client) baseURL() string {
	port := c.Port
	if port == 0 {
		port = 1400
	}
	return fmt.Sprintf("http://%s:%d", c.IP, port)
}

func (c *Client) soapCall(ctx context.Context, controlPath, serviceURN, action string, args map[string]string) (map[string]string, error) {
	return soapCall(ctx, c.HTTP, c.baseURL()+controlPath, serviceURN, action, args)
}
