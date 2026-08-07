package graph

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// GetDriveItem looks up a driveItem by its OpenCloud resource id via
// GET /graph/v1.0/drives/{driveID}/items/{resourceID}.
//
// Resource ids follow reva's composite format
// `<storageID>$<spaceID>!<opaqueID>`. The driveID path segment is the
// `<storageID>$<spaceID>` prefix, while the item segment takes the
// full composite id; SplitResourceID derives the former from the
// latter.
//
// The request is issued by hand: the generated client only exposes the
// v1beta1 driveItem endpoint, which rejects the composite driveID with
// "invalid driveID or itemID". Once a released libre-graph-api-go
// carries the v1.0 operation (GetDriveItemV1), this switches to the
// generated call without an API change.
func (c *Client) GetDriveItem(ctx context.Context, resourceID string) (*libregraph.DriveItem, error) {
	driveID, _, ok := SplitResourceID(resourceID)
	if !ok {
		return nil, fmt.Errorf("graph: malformed driveItem id: %q", resourceID)
	}
	path := "/v1.0/drives/" + url.PathEscape(driveID) + "/items/" + url.PathEscape(resourceID)
	var item libregraph.DriveItem
	if err := c.getJSON(ctx, path, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// SplitResourceID splits a composite resource id
// (`<storageID>$<spaceID>!<opaqueID>`) into the drive id
// (`<storageID>$<spaceID>`) and the opaque item part. ok is false when
// either side of the `!` separator is missing.
func SplitResourceID(id string) (driveID, itemID string, ok bool) {
	driveID, itemID, found := strings.Cut(id, "!")
	if !found || driveID == "" || itemID == "" {
		return "", "", false
	}
	return driveID, itemID, true
}
