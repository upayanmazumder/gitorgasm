package github

import (
	"context"
	"net/http"
	"strconv"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// GetInstallation calls GET /app/installations/{installation_id} (apps/get-installation).
func (c *Client) GetInstallation(ctx context.Context, id int64) (*models.Installation, error) {
	token, err := c.appJWT()
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/app/installations/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var inst models.Installation
	if _, err := c.do(ctx, "apps/get-installation", "app", req, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}

// ListInstallationRepositories pages GET /installation/repositories (apps/list-repos-accessible-to-installation).
func (c *Client) ListInstallationRepositories(ctx context.Context, id int64, fn func([]models.Repository) error) error {
	next := "/installation/repositories?per_page=100"
	for next != "" {
		token, err := c.installationToken(ctx, id)
		if err != nil {
			return err
		}
		req, err := c.newRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		var page struct {
			Repositories []models.Repository `json:"repositories"`
		}
		hdr, err := c.do(ctx, "apps/list-repos-accessible-to-installation", "installation", req, &page)
		if err != nil {
			return err
		}
		if err := fn(page.Repositories); err != nil {
			return err
		}
		next = nextLink(hdr.Get("Link"))
	}
	return nil
}
