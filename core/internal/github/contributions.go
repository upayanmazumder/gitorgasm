package github

import (
	"context"
	"fmt"
	"net/url"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// ListContributions emits attributed events of one repository, one top-level page at a time.
// emit receives the cursor to resume from after that page; batches may repeat events after a
// resume, so storage must be idempotent on EventID. A 404 means the repository is gone.
//
// Operations (github/rest-api-description v2.1.0): repos/list-commits (409 on an empty
// repository), pulls/list (no since parameter, so pages stop at the first older update),
// pulls/list-reviews, issues/list-for-repo, and issues/list-events (410 for a deleted issue).
// PR merges credit the PR author. Issue closes credit the actor of each "closed" event.
func (c *Client) ListContributions(ctx context.Context, installationID, repoID int64, owner, repo string, w models.ContributionWindow, from models.ContributionCursor, emit func([]models.Contribution, models.ContributionCursor) error) error {
	if installationID <= 0 || repoID <= 0 || owner == "" || repo == "" || emit == nil ||
		w.CommitsSince.IsZero() || w.ActivitySince.IsZero() || !w.CommitsSince.Before(w.Until) || !w.ActivitySince.Before(w.Until) {
		return fmt.Errorf("github contributions: invalid repository, window, or emitter")
	}
	p := contributionPass{
		client: c, installationID: installationID, repoID: repoID,
		base:   "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo),
		window: w, from: from, start: contributionPhase(from.Phase), emit: emit,
	}
	if p.start == 3 {
		return nil
	}
	if p.start <= 0 {
		if err := p.listCommits(ctx); err != nil {
			return err
		}
	}
	if p.start <= 1 {
		if err := p.listPulls(ctx); err != nil {
			return err
		}
	}
	return p.listIssues(ctx)
}
