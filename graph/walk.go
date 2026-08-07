package graph

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// childrenResponse is the collection envelope of the children route.
type childrenResponse struct {
	Value []libregraph.DriveItem `json:"value"`
}

// Children lists the direct children of an item via
// GET /graph/v1.0/drives/{driveID}/items/{itemID}/children.
//
// For a space root, pass the drive id as itemID: an OpenCloud space's
// root item id equals its drive id, and there is no /root/children
// route; children are always fetched via /items/{itemID}/children.
//
// Issued by hand until a released libre-graph-api-go carries the
// GetDriveItemChildren operation.
func (c *Client) Children(ctx context.Context, driveID, itemID string) ([]libregraph.DriveItem, error) {
	path := "/v1.0/drives/" + url.PathEscape(driveID) + "/items/" + url.PathEscape(itemID) + "/children"
	var resp childrenResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// WalkFunc is called by WalkFiles for every file found. path is the
// walk-relative path including the file name (e.g. "photos/a.jpg"),
// parent is the containing folder item, or nil for files directly
// under the walk root. Returning an error aborts the walk.
type WalkFunc func(path string, item libregraph.DriveItem, parent *libregraph.DriveItem) error

// WalkFiles traverses the folder tree below rootItemID depth-first and
// calls visit for every file. Folders whose name starts with a dot
// (e.g. OpenCloud's internal ".space") are skipped. To walk a whole
// space, pass the drive id as rootItemID (see Children).
func (c *Client) WalkFiles(ctx context.Context, driveID, rootItemID string, visit WalkFunc) error {
	return c.walk(ctx, driveID, rootItemID, "", nil, visit)
}

func (c *Client) walk(ctx context.Context, driveID, itemID, prefix string, parent *libregraph.DriveItem, visit WalkFunc) error {
	children, err := c.Children(ctx, driveID, itemID)
	if err != nil {
		return fmt.Errorf("graph: list children of %q: %w", prefix, err)
	}
	for i := range children {
		item := children[i]
		name := item.GetName()
		itemPath := prefix + name
		switch {
		case item.Folder != nil:
			if strings.HasPrefix(name, ".") {
				continue
			}
			if err := c.walk(ctx, driveID, item.GetId(), itemPath+"/", &item, visit); err != nil {
				return err
			}
		case item.File != nil:
			if err := visit(itemPath, item, parent); err != nil {
				return err
			}
		}
	}
	return nil
}
