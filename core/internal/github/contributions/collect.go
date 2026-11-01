package contributions

import (
	"context"
	"fmt"
	"net/url"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// Collect emits repository events and the resumable cursor after each top-level page.
func Collect(ctx context.Context, client *github.Client, installationID, repoID int64, owner, repo string, w models.ContributionWindow, from models.ContributionCursor, emit func([]models.Contribution, models.ContributionCursor) error) error {
	if installationID <= 0 || repoID <= 0 || owner == "" || repo == "" || emit == nil ||
		w.CommitsSince.IsZero() || w.ActivitySince.IsZero() || !w.CommitsSince.Before(w.Until) || !w.ActivitySince.Before(w.Until) {
		return fmt.Errorf("github contributions: invalid repository, window, or emitter")
	}
	p := contributionPass{
		client: client, installationID: installationID, repoID: repoID,
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
