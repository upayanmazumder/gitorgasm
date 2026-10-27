package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Contribution records an attributed event without storing GitHub content.
type Contribution struct {
	EventID      string
	RepositoryID int64
	UserID       int64
	Login        string
	Kind         string
	OccurredAt   time.Time
	IsBot        bool
}

type contributionUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type contributionCommit struct {
	SHA    string            `json:"sha"`
	Author *contributionUser `json:"author"`
	Commit struct {
		Committer *struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type contributionPull struct {
	ID        int64             `json:"id"`
	Number    int64             `json:"number"`
	User      *contributionUser `json:"user"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	MergedAt  *time.Time        `json:"merged_at"`
}

type contributionReview struct {
	ID          int64             `json:"id"`
	User        *contributionUser `json:"user"`
	SubmittedAt *time.Time        `json:"submitted_at"`
}

type contributionIssue struct {
	ID          int64             `json:"id"`
	Number      int64             `json:"number"`
	User        *contributionUser `json:"user"`
	PullRequest *json.RawMessage  `json:"pull_request"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type contributionIssueEvent struct {
	ID        int64             `json:"id"`
	Event     string            `json:"event"`
	Actor     *contributionUser `json:"actor"`
	CreatedAt time.Time         `json:"created_at"`
}

// contributionPages rejects incomplete or cyclic pagination.
func contributionPages[T any](ctx context.Context, c *Client, installationID int64, path, operation string, visit func([]T) error) error {
	seen := make(map[string]struct{})
	for path != "" {
		if _, ok := seen[path]; ok {
			return fmt.Errorf("github %s: pagination loop at %s", operation, path)
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
			return fmt.Errorf("github %s: null page at %s", operation, path)
		}
		if err := visit(page); err != nil {
			return err
		}
		path = nextLink(hdr.Get("Link"))
	}
	return nil
}

// ListContributions emits events in [since, until) and requires discarding pages on failure.
func (c *Client) ListContributions(ctx context.Context, installationID, repoID int64, owner, repo string, since, until time.Time, emit func([]Contribution) error) error {
	if installationID <= 0 || repoID <= 0 || owner == "" || repo == "" || since.IsZero() || !since.Before(until) || emit == nil {
		return fmt.Errorf("github contributions: invalid repository, window, or emitter")
	}
	base := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	seen := make(map[string]struct{})
	within := func(at time.Time) bool { return !at.IsZero() && !at.Before(since) && at.Before(until) }
	add := func(batch *[]Contribution, kind, source string, user *contributionUser, at time.Time) error {
		if user == nil || !within(at) {
			return nil
		}
		if user.ID <= 0 || user.Login == "" || source == "" {
			return fmt.Errorf("github contributions: invalid %s identity", kind)
		}
		id := strconv.FormatInt(repoID, 10) + ":" + kind + ":" + source
		if _, exists := seen[id]; exists {
			return nil
		}
		seen[id] = struct{}{}
		*batch = append(*batch, Contribution{EventID: id, RepositoryID: repoID, UserID: user.ID, Login: user.Login, Kind: kind, OccurredAt: at, IsBot: user.Type == "Bot"})
		return nil
	}
	flush := func(batch []Contribution) error {
		if len(batch) != 0 {
			return emit(batch)
		}
		return nil
	}

	// GitHub uses a strict, second-precision since boundary for commits.
	querySince := since.UTC().Truncate(time.Second).Add(-time.Second).Format(time.RFC3339)
	commitsQuery := url.Values{"per_page": {"100"}, "since": {querySince}, "until": {until.UTC().Format(time.RFC3339)}}
	if err := contributionPages[contributionCommit](ctx, c, installationID, base+"/commits?"+commitsQuery.Encode(), "repos/list-commits", func(commits []contributionCommit) error {
		batch := make([]Contribution, 0, len(commits))
		for _, commit := range commits {
			if commit.Author == nil {
				continue // Unlinked git identity, not a GitHub user.
			}
			if commit.Commit.Committer == nil || commit.Commit.Committer.Date.IsZero() || commit.SHA == "" {
				return fmt.Errorf("github commits: missing committer date or SHA")
			}
			if err := add(&batch, "commit", commit.SHA, commit.Author, commit.Commit.Committer.Date); err != nil {
				return err
			}
		}
		return flush(batch)
	}); err != nil {
		return err
	}

	pullsQuery := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}}
	if err := contributionPages[contributionPull](ctx, c, installationID, base+"/pulls?"+pullsQuery.Encode(), "pulls/list", func(pulls []contributionPull) error {
		batch := make([]Contribution, 0, len(pulls)*2)
		for _, pull := range pulls {
			if pull.ID <= 0 || pull.Number <= 0 || pull.UpdatedAt.IsZero() || pull.CreatedAt.IsZero() {
				return fmt.Errorf("github pulls: missing ID, number, or timestamp")
			}
			if pull.UpdatedAt.Before(since) {
				continue // Older updates cannot contain new opens, merges, or reviews.
			}
			if err := add(&batch, "pr_open", strconv.FormatInt(pull.ID, 10), pull.User, pull.CreatedAt); err != nil {
				return err
			}
			if pull.MergedAt != nil {
				if err := add(&batch, "pr_merge", strconv.FormatInt(pull.ID, 10), pull.User, *pull.MergedAt); err != nil {
					return err
				}
			}
			reviewPath := base + "/pulls/" + strconv.FormatInt(pull.Number, 10) + "/reviews?per_page=100"
			if err := contributionPages[contributionReview](ctx, c, installationID, reviewPath, "pulls/list-reviews", func(reviews []contributionReview) error {
				reviewBatch := make([]Contribution, 0, len(reviews))
				for _, review := range reviews {
					if review.SubmittedAt == nil || review.User == nil { // PENDING reviews and deleted users are not contributions.
						continue
					}
					if review.ID <= 0 {
						return fmt.Errorf("github reviews: missing ID")
					}
					if err := add(&reviewBatch, "review", strconv.FormatInt(review.ID, 10), review.User, *review.SubmittedAt); err != nil {
						return err
					}
				}
				return flush(reviewBatch)
			}); err != nil {
				return err
			}
		}
		return flush(batch)
	}); err != nil {
		return err
	}

	issuesQuery := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}, "since": {querySince}}
	return contributionPages[contributionIssue](ctx, c, installationID, base+"/issues?"+issuesQuery.Encode(), "issues/list-for-repo", func(issues []contributionIssue) error {
		batch := make([]Contribution, 0, len(issues))
		for _, issue := range issues {
			if issue.ID <= 0 || issue.Number <= 0 || issue.CreatedAt.IsZero() || issue.UpdatedAt.IsZero() {
				return fmt.Errorf("github issues: missing ID, number, or timestamp")
			}
			if issue.PullRequest != nil || issue.UpdatedAt.Before(since) {
				continue // GitHub's issues endpoint also includes pull requests.
			}
			if err := add(&batch, "issue_open", strconv.FormatInt(issue.ID, 10), issue.User, issue.CreatedAt); err != nil {
				return err
			}
			// Event IDs, unlike closed_at, retain every close after a reopen.
			eventsPath := base + "/issues/" + strconv.FormatInt(issue.Number, 10) + "/events?per_page=100"
			if err := contributionPages[contributionIssueEvent](ctx, c, installationID, eventsPath, "issues/list-events-for-timeline", func(events []contributionIssueEvent) error {
				eventBatch := make([]Contribution, 0, len(events))
				for _, event := range events {
					if event.Event != "closed" || event.Actor == nil {
						continue
					}
					if event.ID <= 0 || event.CreatedAt.IsZero() {
						return fmt.Errorf("github issue events: missing ID or timestamp")
					}
					if err := add(&eventBatch, "issue_close", strconv.FormatInt(event.ID, 10), event.Actor, event.CreatedAt); err != nil {
						return err
					}
				}
				return flush(eventBatch)
			}); err != nil {
				return err
			}
		}
		return flush(batch)
	})
}
