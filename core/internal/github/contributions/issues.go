package contributions

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// listIssues uses issues/list-for-repo and issues/list-events; deleted issues may return 410.
func (p *contributionPass) listIssues(ctx context.Context) error {
	q := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}, "since": {sinceParam(p.window.ActivitySince)}}
	return contributionPages(ctx, p.client, p.installationID, p.first(2, p.base+"/issues?"+q.Encode()), "issues/list-for-repo", func(issues []models.ContributionIssue, next string) (bool, error) {
		batch := make([]models.Contribution, 0, len(issues))
		for _, issue := range issues {
			if issue.ID <= 0 || issue.Number <= 0 || issue.CreatedAt.IsZero() || issue.UpdatedAt.IsZero() {
				return false, fmt.Errorf("github issues: missing ID, number, or timestamp")
			}
			if issue.PullRequest != nil || issue.UpdatedAt.Before(p.window.ActivitySince) {
				continue // The issues endpoint also returns pull requests.
			}
			if err := p.add(&batch, p.window.ActivitySince, "issue_open", strconv.FormatInt(issue.ID, 10), issue.User, issue.CreatedAt); err != nil {
				return false, err
			}
			// Event IDs, unlike closed_at, keep every close after a reopen.
			events := p.base + "/issues/" + strconv.FormatInt(issue.Number, 10) + "/events?per_page=100"
			err := contributionPages(ctx, p.client, p.installationID, events, "issues/list-events", func(page []models.ContributionIssueEvent, _ string) (bool, error) {
				for _, event := range page {
					if event.Event != "closed" || event.Actor == nil {
						continue
					}
					if event.ID <= 0 || event.CreatedAt.IsZero() {
						return false, fmt.Errorf("github issue events: missing ID or timestamp")
					}
					if err := p.add(&batch, p.window.ActivitySince, "issue_close", strconv.FormatInt(event.ID, 10), event.Actor, event.CreatedAt); err != nil {
						return false, err
					}
				}
				return false, nil
			})
			if err != nil && !hasStatus(err, http.StatusGone) {
				return false, err
			}
		}
		return false, p.emit(batch, p.cursorAfter(2, next, false))
	})
}
