package sonos

import (
	"context"
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
