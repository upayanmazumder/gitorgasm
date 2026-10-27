package github

import (
	"context"
	"encoding/json"
	"errors"
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

// ContributionWindow bounds one repository pass. Commits use a longer lookback than
// pull request and issue activity because a push can carry old committer dates.
type ContributionWindow struct {
	CommitsSince  time.Time
	ActivitySince time.Time
	Until         time.Time
}

// Contribution phases in the order a repository pass runs them.
const (
	PhaseCommits = "commits"
	PhasePulls   = "pulls"
	PhaseIssues  = "issues"
	PhaseDone    = "done"
)

// ContributionCursor is the resume point of a repository pass: a phase and the next
// top-level page URL. An empty Next starts the phase from its first page.
type ContributionCursor struct {
	Phase string
	Next  string
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

// contributionPages rejects incomplete or cyclic pagination. visit receives the next page
// URL ("" on the last page) and may stop early.
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

func hasStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

// ListContributions emits attributed events of one repository, one top-level page at a time.
// emit receives the cursor to resume from after that page; batches may repeat events after a
// resume, so storage must be idempotent on EventID. A 404 means the repository is gone.
//
// Operations (github/rest-api-description v2.1.0): repos/list-commits (409 on an empty
// repository), pulls/list (no since parameter, so pages stop at the first older update),
// pulls/list-reviews, issues/list-for-repo, and issues/list-events (410 for a deleted issue).
// PR merges credit the PR author. Issue closes credit the actor of each "closed" event.
func (c *Client) ListContributions(ctx context.Context, installationID, repoID int64, owner, repo string, w ContributionWindow, from ContributionCursor, emit func([]Contribution, ContributionCursor) error) error {
	if installationID <= 0 || repoID <= 0 || owner == "" || repo == "" || emit == nil ||
		w.CommitsSince.IsZero() || w.ActivitySince.IsZero() || !w.CommitsSince.Before(w.Until) || !w.ActivitySince.Before(w.Until) {
		return fmt.Errorf("github contributions: invalid repository, window, or emitter")
	}
	base := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	add := func(batch *[]Contribution, since time.Time, kind, source string, user *contributionUser, at time.Time) error {
		if user == nil || at.IsZero() || at.Before(since) || !at.Before(w.Until) {
			return nil
		}
		if user.ID <= 0 || user.Login == "" || source == "" {
			return fmt.Errorf("github contributions: invalid %s identity", kind)
		}
		*batch = append(*batch, Contribution{
			EventID: strconv.FormatInt(repoID, 10) + ":" + kind + ":" + source, RepositoryID: repoID,
			UserID: user.ID, Login: user.Login, Kind: kind, OccurredAt: at, IsBot: user.Type == "Bot",
		})
		return nil
	}
	// GitHub compares since strictly and with second precision.
	sinceParam := func(at time.Time) string {
		return at.UTC().Truncate(time.Second).Add(-time.Second).Format(time.RFC3339)
	}
	phase := func(name string) int {
		switch name {
		case "", PhaseCommits:
			return 0
		case PhasePulls:
			return 1
		case PhaseIssues:
			return 2
		default:
			return 3
		}
	}
	order := []string{PhaseCommits, PhasePulls, PhaseIssues, PhaseDone}
	start := phase(from.Phase)
	if start == 3 {
		return nil
	}
	cursorAfter := func(i int, next string, stop bool) ContributionCursor {
		if next == "" || stop {
			return ContributionCursor{Phase: order[i+1]}
		}
		return ContributionCursor{Phase: order[i], Next: next}
	}
	first := func(i int, initial string) string {
		if i == start && from.Next != "" {
			return from.Next
		}
		return initial
	}

	if start <= 0 {
		q := url.Values{"per_page": {"100"}, "since": {sinceParam(w.CommitsSince)}, "until": {w.Until.UTC().Format(time.RFC3339)}}
		err := contributionPages(ctx, c, installationID, first(0, base+"/commits?"+q.Encode()), "repos/list-commits", func(commits []contributionCommit, next string) (bool, error) {
			batch := make([]Contribution, 0, len(commits))
			for _, commit := range commits {
				if commit.Author == nil {
					continue // Unlinked git identity, not a GitHub user.
				}
				if commit.Commit.Committer == nil || commit.Commit.Committer.Date.IsZero() || commit.SHA == "" {
					return false, fmt.Errorf("github commits: missing committer date or SHA")
				}
				if err := add(&batch, w.CommitsSince, "commit", commit.SHA, commit.Author, commit.Commit.Committer.Date); err != nil {
					return false, err
				}
			}
			return false, emit(batch, cursorAfter(0, next, false))
		})
		if hasStatus(err, http.StatusConflict) {
			err = emit(nil, ContributionCursor{Phase: PhasePulls})
		}
		if err != nil {
			return err
		}
	}

	if start <= 1 {
		q := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}}
		err := contributionPages(ctx, c, installationID, first(1, base+"/pulls?"+q.Encode()), "pulls/list", func(pulls []contributionPull, next string) (bool, error) {
			batch := make([]Contribution, 0, len(pulls)*2)
			stop := false
			for _, pull := range pulls {
				if pull.ID <= 0 || pull.Number <= 0 || pull.UpdatedAt.IsZero() || pull.CreatedAt.IsZero() {
					return false, fmt.Errorf("github pulls: missing ID, number, or timestamp")
				}
				if pull.UpdatedAt.Before(w.ActivitySince) {
					stop = true // Sorted by update time, so every later pull is older.
					break
				}
				source := strconv.FormatInt(pull.ID, 10)
				if err := add(&batch, w.ActivitySince, "pr_open", source, pull.User, pull.CreatedAt); err != nil {
					return false, err
				}
				if pull.MergedAt != nil {
					if err := add(&batch, w.ActivitySince, "pr_merge", source, pull.User, *pull.MergedAt); err != nil {
						return false, err
					}
				}
				reviews := base + "/pulls/" + strconv.FormatInt(pull.Number, 10) + "/reviews?per_page=100"
				if err := contributionPages(ctx, c, installationID, reviews, "pulls/list-reviews", func(page []contributionReview, _ string) (bool, error) {
					for _, review := range page {
						if review.SubmittedAt == nil || review.User == nil { // Pending reviews and deleted users.
							continue
						}
						if review.ID <= 0 {
							return false, fmt.Errorf("github reviews: missing ID")
						}
						if err := add(&batch, w.ActivitySince, "review", strconv.FormatInt(review.ID, 10), review.User, *review.SubmittedAt); err != nil {
							return false, err
						}
					}
					return false, nil
				}); err != nil {
					return false, err
				}
			}
			return stop, emit(batch, cursorAfter(1, next, stop))
		})
		if err != nil {
			return err
		}
	}

	q := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}, "since": {sinceParam(w.ActivitySince)}}
	return contributionPages(ctx, c, installationID, first(2, base+"/issues?"+q.Encode()), "issues/list-for-repo", func(issues []contributionIssue, next string) (bool, error) {
		batch := make([]Contribution, 0, len(issues))
		for _, issue := range issues {
			if issue.ID <= 0 || issue.Number <= 0 || issue.CreatedAt.IsZero() || issue.UpdatedAt.IsZero() {
				return false, fmt.Errorf("github issues: missing ID, number, or timestamp")
			}
			if issue.PullRequest != nil || issue.UpdatedAt.Before(w.ActivitySince) {
				continue // The issues endpoint also returns pull requests.
			}
			if err := add(&batch, w.ActivitySince, "issue_open", strconv.FormatInt(issue.ID, 10), issue.User, issue.CreatedAt); err != nil {
				return false, err
			}
			// Event IDs, unlike closed_at, keep every close after a reopen.
			events := base + "/issues/" + strconv.FormatInt(issue.Number, 10) + "/events?per_page=100"
			err := contributionPages(ctx, c, installationID, events, "issues/list-events", func(page []contributionIssueEvent, _ string) (bool, error) {
				for _, event := range page {
					if event.Event != "closed" || event.Actor == nil {
						continue
					}
					if event.ID <= 0 || event.CreatedAt.IsZero() {
						return false, fmt.Errorf("github issue events: missing ID or timestamp")
					}
					if err := add(&batch, w.ActivitySince, "issue_close", strconv.FormatInt(event.ID, 10), event.Actor, event.CreatedAt); err != nil {
						return false, err
					}
				}
				return false, nil
			})
			if err != nil && !hasStatus(err, http.StatusGone) {
				return false, err
			}
		}
		return false, emit(batch, cursorAfter(2, next, false))
	})
}
