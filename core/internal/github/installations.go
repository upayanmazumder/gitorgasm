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
		var page struct {
			Repositories []models.Repository `json:"repositories"`
		}
		following, err := c.GetInstallationPage(ctx, id, next, "apps/list-repos-accessible-to-installation", &page)
		if err != nil {
			return err
		}
		if err := fn(page.Repositories); err != nil {
			return err
		}
		next = following
	}
	return nil
}
