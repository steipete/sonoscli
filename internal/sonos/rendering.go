package sonos

import (
	"context"
	"strconv"
)

func (c *Client) GetVolume(ctx context.Context) (int, error) {
	resp, err := c.soapCall(ctx, controlRenderingControl, urnRenderingControl, "GetVolume", map[string]string{
		"InstanceID": "0",
		"Channel":    "Master",
	})
	if err != nil {
		return 0, err
	}
	v, _ := strconv.Atoi(resp["CurrentVolume"])
	return v, nil
}

func (c *Client) SetVolume(ctx context.Context, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	_, err := c.soapCall(ctx, controlRenderingControl, urnRenderingControl, "SetVolume", map[string]string{
		"InstanceID":    "0",
		"Channel":       "Master",
		"DesiredVolume": strconv.Itoa(volume),
	})
	return err
}

func (c *Client) GetMute(ctx context.Context) (bool, error) {
	resp, err := c.soapCall(ctx, controlRenderingControl, urnRenderingControl, "GetMute", map[string]string{
		"InstanceID": "0",
		"Channel":    "Master",
	})
	if err != nil {
		return false, err
	}
	return resp["CurrentMute"] == "1", nil
}

func (c *Client) SetMute(ctx context.Context, mute bool) error {
	v := "0"
	if mute {
		v = "1"
	}
	_, err := c.soapCall(ctx, controlRenderingControl, urnRenderingControl, "SetMute", map[string]string{
		"InstanceID":  "0",
		"Channel":     "Master",
		"DesiredMute": v,
	})
	return err
}
