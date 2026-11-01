package github

import (
	"context"
	"net/http"
	"strings"
)

// nextLink extracts the rel="next" URL from an RFC 8288 Link header.
func nextLink(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		segs := strings.Split(part, ";")
		if len(segs) < 2 {
			continue
		}
		for _, s := range segs[1:] {
			if strings.TrimSpace(s) == `rel="next"` {
				return strings.Trim(strings.TrimSpace(segs[0]), "<>")
			}
		}
	}
	return ""
}

// GetInstallationPage decodes an installation-authenticated GET and returns its next link.
func (c *Client) GetInstallationPage(ctx context.Context, installationID int64, path, operation string, dst any) (string, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	hdr, err := c.do(ctx, operation, "installation", req, dst)
	if err != nil {
		return "", err
	}
	return nextLink(hdr.Get("Link")), nil
}
