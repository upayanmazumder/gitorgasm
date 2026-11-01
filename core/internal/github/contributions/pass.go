package contributions

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

type contributionPass struct {
	client         *github.Client
	installationID int64
	repoID         int64
	base           string
	window         models.ContributionWindow
	from           models.ContributionCursor
	start          int
	emit           func([]models.Contribution, models.ContributionCursor) error
}

func (p *contributionPass) add(batch *[]models.Contribution, since time.Time, kind, source string, user *models.ContributionUser, at time.Time) error {
	if user == nil || at.IsZero() || at.Before(since) || !at.Before(p.window.Until) {
		return nil
	}
	if user.ID <= 0 || user.Login == "" || source == "" {
		return fmt.Errorf("github contributions: invalid %s identity", kind)
	}
	*batch = append(*batch, models.Contribution{
		EventID: strconv.FormatInt(p.repoID, 10) + ":" + kind + ":" + source, RepositoryID: p.repoID,
		UserID: user.ID, Login: user.Login, Kind: kind, OccurredAt: at, IsBot: user.Type == "Bot",
	})
	return nil
}

// GitHub compares since strictly and with second precision.
func sinceParam(at time.Time) string {
	return at.UTC().Truncate(time.Second).Add(-time.Second).Format(time.RFC3339)
}

func contributionPhase(name string) int {
	switch name {
	case "", models.PhaseCommits:
		return 0
	case models.PhasePulls:
		return 1
	case models.PhaseIssues:
		return 2
	default:
		return 3
	}
}

func (p *contributionPass) cursorAfter(i int, next string, stop bool) models.ContributionCursor {
	order := [...]string{models.PhaseCommits, models.PhasePulls, models.PhaseIssues, models.PhaseDone}
	if next == "" || stop {
		return models.ContributionCursor{Phase: order[i+1]}
	}
	return models.ContributionCursor{Phase: order[i], Next: next}
}

func (p *contributionPass) first(i int, initial string) string {
	if i == p.start && p.from.Next != "" {
		return p.from.Next
	}
	return initial
}

func hasStatus(err error, status int) bool {
	var apiErr *github.APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}
