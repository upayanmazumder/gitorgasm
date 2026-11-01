package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// listPulls uses pulls/list and pulls/list-reviews, crediting merges to the author.
func (p *contributionPass) listPulls(ctx context.Context) error {
	q := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}}
	err := contributionPages(ctx, p.client, p.installationID, p.first(1, p.base+"/pulls?"+q.Encode()), "pulls/list", func(pulls []models.ContributionPull, next string) (bool, error) {
		batch := make([]models.Contribution, 0, len(pulls)*2)
		stop := false
		for _, pull := range pulls {
			if pull.ID <= 0 || pull.Number <= 0 || pull.UpdatedAt.IsZero() || pull.CreatedAt.IsZero() {
				return false, fmt.Errorf("github pulls: missing ID, number, or timestamp")
			}
			if pull.UpdatedAt.Before(p.window.ActivitySince) {
				stop = true // Sorted by update time, so every later pull is older.
				break
			}
			source := strconv.FormatInt(pull.ID, 10)
			if err := p.add(&batch, p.window.ActivitySince, "pr_open", source, pull.User, pull.CreatedAt); err != nil {
				return false, err
			}
			if pull.MergedAt != nil {
				if err := p.add(&batch, p.window.ActivitySince, "pr_merge", source, pull.User, *pull.MergedAt); err != nil {
					return false, err
				}
			}
			reviews := p.base + "/pulls/" + strconv.FormatInt(pull.Number, 10) + "/reviews?per_page=100"
			if err := contributionPages(ctx, p.client, p.installationID, reviews, "pulls/list-reviews", func(page []models.ContributionReview, _ string) (bool, error) {
				for _, review := range page {
					if review.SubmittedAt == nil || review.User == nil { // Pending reviews and deleted users.
						continue
					}
					if review.ID <= 0 {
						return false, fmt.Errorf("github reviews: missing ID")
					}
					if err := p.add(&batch, p.window.ActivitySince, "review", strconv.FormatInt(review.ID, 10), review.User, *review.SubmittedAt); err != nil {
						return false, err
					}
				}
				return false, nil
			}); err != nil {
				return false, err
			}
		}
		return stop, p.emit(batch, p.cursorAfter(1, next, stop))
	})
	if err != nil {
		return err
	}
	return nil
}
