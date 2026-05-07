package sonos

import (
	"context"
	"errors"
	"fmt"
)

type QueueItem struct {
	Position int      `json:"position"` // 1-based
	Item     DIDLItem `json:"item"`
}

type QueuePage struct {
	Items          []QueueItem `json:"items"`
	NumberReturned int         `json:"numberReturned"`
	TotalMatches   int         `json:"totalMatches"`
	UpdateID       int         `json:"updateID"`
}

func (c *Client) ListQueue(ctx context.Context, start, count int) (QueuePage, error) {
	if start < 0 {
		start = 0
	}
	if count <= 0 {
		count = 100
	}
	br, err := c.Browse(ctx, "Q:0", start, count)
	if err != nil {
		return QueuePage{}, err
	}
	didlItems, err := ParseDIDLItems(br.Result)
	if err != nil {
		return QueuePage{}, err
	}
	items := make([]QueueItem, 0, len(didlItems))
	for i, it := range didlItems {
		items = append(items, QueueItem{
			Position: start + i + 1,
			Item:     it,
		})
	}
	return QueuePage{
		Items:          items,
		NumberReturned: br.NumberReturned,
		TotalMatches:   br.TotalMatches,
		UpdateID:       br.UpdateID,
	}, nil
}

func (c *Client) ClearQueue(ctx context.Context) error {
	return c.RemoveAllTracksFromQueue(ctx)
}

func (c *Client) RemoveQueuePosition(ctx context.Context, position int) error {
	if position <= 0 {
		return fmt.Errorf("position must be >= 1")
	}
	return c.RemoveTrackFromQueue(ctx, position)
}

func (c *Client) PlayQueuePosition(ctx context.Context, position int) error {
	if position <= 0 {
		return fmt.Errorf("position must be >= 1")
	}
	return c.playFromQueueTrack(ctx, position)
}

// PlayFromQueueTrack binds the speaker's AVTransport to its queue and starts
// playback at the given 1-based track number.
func (c *Client) PlayFromQueueTrack(ctx context.Context, oneBasedTrackNumber int) error {
	return c.playFromQueueTrack(ctx, oneBasedTrackNumber)
}

func (c *Client) playFromQueueTrack(ctx context.Context, oneBasedTrackNumber int) error {
	dd, err := c.GetDeviceDescription(ctx)
	if err != nil {
		return err
	}
	if dd.UDN == "" {
		return errors.New("missing device UDN")
	}
	queueURI := "x-rincon-queue:" + dd.UDN + "#0"
	if err := c.SetAVTransportURI(ctx, queueURI, ""); err != nil {
		return err
	}
	if err := c.SeekTrackNumber(ctx, oneBasedTrackNumber); err != nil {
		return err
	}
	return c.Play(ctx)
}
