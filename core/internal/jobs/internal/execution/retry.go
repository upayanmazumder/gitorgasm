package execution

import (
	"errors"

	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

// RetryDecision delays rate limits, stops on permission errors, and otherwise lets River back off.
func RetryDecision(err error) error {
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Kind {
	case github.KindRateLimited:
		return river.JobSnooze(apiErr.RetryAfter)
	case github.KindUnauthorized, github.KindForbidden, github.KindInvalid:
		return river.JobCancel(err)
	default:
		return err
	}
}
