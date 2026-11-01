package github

import (
	"context"
	"fmt"
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

// contributionPages rejects null or cyclic pages; visit may stop before the next page.
func contributionPages[T any](ctx context.Context, c *Client, installationID int64, path, operation string, visit func(page []T, next string) (stop bool, err error)) error {
	seen := make(map[string]struct{})
	for path != "" {
		if _, ok := seen[path]; ok {
			return fmt.Errorf("github %s: pagination loop", operation)
		}
		seen[path] = struct{}{}
		token, err := c.installationToken(ctx, installationID)
		if err != nil {
			return err
		}
		req, err := c.newRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		var page []T
		hdr, err := c.do(ctx, operation, "installation", req, &page)
		if err != nil {
			return err
		}
		if page == nil {
			return fmt.Errorf("github %s: null page", operation)
		}
		next := nextLink(hdr.Get("Link"))
		stop, err := visit(page, next)
		if err != nil || stop {
			return err
		}
		path = next
	}
	return nil
}
