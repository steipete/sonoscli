package sonos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

func (c *Client) Play(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Play", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	return err
}

func (c *Client) Pause(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Pause", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	return err
}

func (c *Client) Stop(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Stop", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	return err
}

// StopOrNoop attempts to stop playback. Some sources (e.g. TV input via
// x-sonos-htastream) reject Stop with UPnP error 701 (Transition not available).
// In that case, this is treated as a successful no-op.
func (c *Client) StopOrNoop(ctx context.Context) error {
	if err := c.Stop(ctx); err != nil {
		var upnpErr *UPnPError
		if errors.As(err, &upnpErr) && upnpErr.Code == "701" {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) Next(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Next", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	return err
}

func (c *Client) Previous(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Previous", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	return err
}

// PreviousOrRestart attempts to go to the previous track. If the device
// rejects the transition (common for some streaming sources), it falls back
// to restarting the current track by seeking to 0:00:00.
func (c *Client) PreviousOrRestart(ctx context.Context) error {
	if err := c.Previous(ctx); err != nil {
		var upnpErr *UPnPError
		if errors.As(err, &upnpErr) {
			// Observed on some sources (e.g. Spotify): Previous returns a UPnP error
			// instead of restarting the current track like the Sonos controller does.
			// 701 = Transition not available
			// 711 = Illegal seek target (some devices misuse this for Previous)
			if upnpErr.Code == "701" || upnpErr.Code == "711" {
				return c.SeekRelTime(ctx, "0:00:00")
			}
		}
		return err
	}
	return nil
}

func (c *Client) SeekRelTime(ctx context.Context, hhmmss string) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Seek", map[string]string{
		"InstanceID": "0",
		"Unit":       "REL_TIME",
		"Target":     hhmmss,
	})
	return err
}

func (c *Client) SeekTrackNumber(ctx context.Context, oneBasedTrackNumber int) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "Seek", map[string]string{
		"InstanceID": "0",
		"Unit":       "TRACK_NR",
		"Target":     strconv.Itoa(oneBasedTrackNumber),
	})
	return err
}

func (c *Client) SetAVTransportURI(ctx context.Context, uri, meta string) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "SetAVTransportURI", map[string]string{
		"InstanceID":         "0",
		"CurrentURI":         uri,
		"CurrentURIMetaData": meta,
	})
	return err
}

func (c *Client) BecomeCoordinatorOfStandaloneGroup(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "BecomeCoordinatorOfStandaloneGroup", map[string]string{
		"InstanceID": "0",
	})
	return err
}

func (c *Client) RemoveAllTracksFromQueue(ctx context.Context) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "RemoveAllTracksFromQueue", map[string]string{
		"InstanceID": "0",
	})
	return err
}

func (c *Client) RemoveTrackFromQueue(ctx context.Context, oneBasedTrackNumber int) error {
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "RemoveTrackFromQueue", map[string]string{
		"InstanceID": "0",
		"ObjectID":   "Q:0/" + strconv.Itoa(oneBasedTrackNumber),
		"UpdateID":   "0",
	})
	return err
}

func (c *Client) AddURIToQueue(ctx context.Context, enqueuedURI, enqueuedMeta string, desiredFirstTrackNumber int, enqueueAsNext bool) (firstTrackNumber int, err error) {
	asNext := "0"
	if enqueueAsNext {
		asNext = "1"
	}
	resp, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "AddURIToQueue", map[string]string{
		"InstanceID":                      "0",
		"EnqueuedURI":                     enqueuedURI,
		"EnqueuedURIMetaData":             enqueuedMeta,
		"DesiredFirstTrackNumberEnqueued": strconv.Itoa(desiredFirstTrackNumber),
		"EnqueueAsNext":                   asNext,
	})
	if err != nil {
		return 0, err
	}
	v := resp["FirstTrackNumberEnqueued"]
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

type PositionInfo struct {
	Track         string
	TrackURI      string
	TrackMeta     string
	TrackDuration string
	RelTime       string
}

func (c *Client) GetPositionInfo(ctx context.Context) (PositionInfo, error) {
	resp, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "GetPositionInfo", map[string]string{
		"InstanceID": "0",
	})
	if err != nil {
		return PositionInfo{}, err
	}
	return PositionInfo{
		Track:         resp["Track"],
		TrackURI:      resp["TrackURI"],
		TrackMeta:     resp["TrackMetaData"],
		TrackDuration: resp["TrackDuration"],
		RelTime:       resp["RelTime"],
	}, nil
}

type TransportInfo struct {
	State  string
	Status string
	Speed  string
}

func (c *Client) GetTransportInfo(ctx context.Context) (TransportInfo, error) {
	resp, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "GetTransportInfo", map[string]string{
		"InstanceID": "0",
	})
	if err != nil {
		return TransportInfo{}, err
	}
	return TransportInfo{
		State:  resp["CurrentTransportState"],
		Status: resp["CurrentTransportStatus"],
		Speed:  resp["CurrentSpeed"],
	}, nil
}

// PlayMode represents the playback mode (shuffle/repeat settings).
// Valid values: NORMAL, SHUFFLE, SHUFFLE_NOREPEAT, REPEAT_ALL, REPEAT_ONE
type PlayMode string

const (
	PlayModeNormal          PlayMode = "NORMAL"
	PlayModeShuffle         PlayMode = "SHUFFLE"
	PlayModeShuffleNoRepeat PlayMode = "SHUFFLE_NOREPEAT"
	PlayModeRepeatAll       PlayMode = "REPEAT_ALL"
	PlayModeRepeatOne       PlayMode = "REPEAT_ONE"
)

// TransportSettings holds the current play mode and rec quality mode.
type TransportSettings struct {
	PlayMode       PlayMode
	RecQualityMode string
}

// GetTransportSettings returns the current play mode (shuffle/repeat) settings.
func (c *Client) GetTransportSettings(ctx context.Context) (TransportSettings, error) {
	resp, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "GetTransportSettings", map[string]string{
		"InstanceID": "0",
	})
	if err != nil {
		return TransportSettings{}, err
	}
	return TransportSettings{
		PlayMode:       PlayMode(resp["PlayMode"]),
		RecQualityMode: resp["RecQualityMode"],
	}, nil
}

// SetPlayMode sets the playback mode (shuffle/repeat).
// Valid modes: NORMAL, SHUFFLE, SHUFFLE_NOREPEAT, REPEAT_ALL, REPEAT_ONE
func (c *Client) SetPlayMode(ctx context.Context, mode PlayMode) error {
	// Validate mode before sending to device
	switch mode {
	case PlayModeNormal, PlayModeShuffle, PlayModeShuffleNoRepeat, PlayModeRepeatAll, PlayModeRepeatOne:
		// valid
	default:
		return fmt.Errorf("invalid play mode: %q", mode)
	}
	_, err := c.soapCall(ctx, controlAVTransport, urnAVTransport, "SetPlayMode", map[string]string{
		"InstanceID":  "0",
		"NewPlayMode": string(mode),
	})
	return err
}
