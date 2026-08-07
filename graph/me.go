package graph

import (
	"context"
	"fmt"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// GetMe resolves the calling user via GET /graph/v1.0/me. Useful for
// protocol handlers that report account details and for validating
// credentials at startup or in health checks.
func (c *Client) GetMe(ctx context.Context) (*libregraph.User, error) {
	authed, err := c.AuthContext(ctx)
	if err != nil {
		return nil, err
	}
	user, _, err := c.api.MeUserApi.GetOwnUser(authed).Execute()
	if err != nil {
		return nil, fmt.Errorf("graph: GET /me: %w", err)
	}
	return user, nil
}
