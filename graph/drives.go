package graph

import (
	"context"
	"fmt"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// MyDrives lists the drives (spaces) visible to the calling user via
// GET /graph/v1.0/me/drives.
func (c *Client) MyDrives(ctx context.Context) ([]libregraph.Drive, error) {
	authed, err := c.AuthContext(ctx)
	if err != nil {
		return nil, err
	}
	drives, _, err := c.api.MeDrivesApi.ListMyDrives(authed).Execute()
	if err != nil {
		return nil, fmt.Errorf("graph: GET /me/drives: %w", err)
	}
	return drives.GetValue(), nil
}

// Drive fetches a single drive (space) by id via
// GET /graph/v1.0/drives/{id}.
func (c *Client) Drive(ctx context.Context, driveID string) (*libregraph.Drive, error) {
	authed, err := c.AuthContext(ctx)
	if err != nil {
		return nil, err
	}
	drive, _, err := c.api.DrivesApi.GetDrive(authed, driveID).Execute()
	if err != nil {
		return nil, fmt.Errorf("graph: GET /drives/%s: %w", driveID, err)
	}
	return drive, nil
}

// DriveByName resolves a drive (space) by its display name, matched
// case-insensitively against the calling user's drives. Adapters use
// this so deployments can configure a human-readable space name
// instead of an opaque drive id.
func (c *Client) DriveByName(ctx context.Context, name string) (*libregraph.Drive, error) {
	drives, err := c.MyDrives(ctx)
	if err != nil {
		return nil, err
	}
	for i := range drives {
		if strings.EqualFold(drives[i].GetName(), name) {
			return &drives[i], nil
		}
	}
	return nil, fmt.Errorf("graph: no drive named %q", name)
}
