package sonos

import (
	"context"
	"errors"
)

func JoinURI(coordinatorUUID string) (string, error) {
	if coordinatorUUID == "" {
		return "", errors.New("coordinator UUID is required")
	}
	return "x-rincon:" + coordinatorUUID, nil
}

// JoinGroup makes this speaker join the group coordinated by coordinatorUUID.
// This is typically done by sending AVTransport.SetAVTransportURI to the joining speaker with:
//
//	CurrentURI = x-rincon:<COORDINATOR_UUID>
func (c *Client) JoinGroup(ctx context.Context, coordinatorUUID string) error {
	uri, err := JoinURI(coordinatorUUID)
	if err != nil {
		return err
	}
	return c.SetAVTransportURI(ctx, uri, "")
}

// LeaveGroup ungroups this speaker, making it the coordinator of a standalone group.
func (c *Client) LeaveGroup(ctx context.Context) error {
	return c.BecomeCoordinatorOfStandaloneGroup(ctx)
}
