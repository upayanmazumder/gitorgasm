package contributions

import (
	"context"
	"fmt"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

// contributionPages rejects null or cyclic pages; visit may stop before the next page.
func contributionPages[T any](ctx context.Context, c *github.Client, installationID int64, path, operation string, visit func(page []T, next string) (stop bool, err error)) error {
	seen := make(map[string]struct{})
	for path != "" {
		if _, ok := seen[path]; ok {
			return fmt.Errorf("github %s: pagination loop", operation)
		}
		seen[path] = struct{}{}
		var page []T
		next, err := c.GetInstallationPage(ctx, installationID, path, operation, &page)
		if err != nil {
			return err
		}
		if page == nil {
			return fmt.Errorf("github %s: null page", operation)
		}
		stop, err := visit(page, next)
		if err != nil || stop {
			return err
		}
		path = next
	}
	return nil
}
