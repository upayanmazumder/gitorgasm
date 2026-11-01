package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// listCommits uses repos/list-commits; a 409 denotes an empty repository.
func (p *contributionPass) listCommits(ctx context.Context) error {
	q := url.Values{"per_page": {"100"}, "since": {sinceParam(p.window.CommitsSince)}, "until": {p.window.Until.UTC().Format(time.RFC3339)}}
	err := contributionPages(ctx, p.client, p.installationID, p.first(0, p.base+"/commits?"+q.Encode()), "repos/list-commits", func(commits []models.ContributionCommit, next string) (bool, error) {
		batch := make([]models.Contribution, 0, len(commits))
		for _, commit := range commits {
			if commit.Author == nil {
				continue // Unlinked git identity, not a GitHub user.
			}
			if commit.Commit.Committer == nil || commit.Commit.Committer.Date.IsZero() || commit.SHA == "" {
				return false, fmt.Errorf("github commits: missing committer date or SHA")
			}
			if err := p.add(&batch, p.window.CommitsSince, "commit", commit.SHA, commit.Author, commit.Commit.Committer.Date); err != nil {
				return false, err
			}
		}
		return false, p.emit(batch, p.cursorAfter(0, next, false))
	})
	if hasStatus(err, http.StatusConflict) {
		err = p.emit(nil, models.ContributionCursor{Phase: models.PhasePulls})
	}
	if err != nil {
		return err
	}
	return nil
}
