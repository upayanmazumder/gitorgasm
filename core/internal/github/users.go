package github

import (
	"context"
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// GetAuthenticatedUser calls GET /user (users/get-authenticated) with a user access token.
func (c *Client) GetAuthenticatedUser(ctx context.Context, userToken string) (*models.User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	var u models.User
	if _, err := c.do(ctx, "users/get-authenticated", "user", req, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUserInstallations calls GET /user/installations (apps/list-installations-for-authenticated-user), the tenant authorization source.
func (c *Client) ListUserInstallations(ctx context.Context, userToken string) ([]models.Installation, error) {
	var all []models.Installation
	next := "/user/installations?per_page=100"
	for next != "" {
		req, err := c.newRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+userToken)
		var page struct {
			Installations []models.Installation `json:"installations"`
		}
		hdr, err := c.do(ctx, "apps/list-installations-for-authenticated-user", "user", req, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Installations...)
		next = nextLink(hdr.Get("Link"))
	}
	return all, nil
}
