package administration

import (
	"context"
	"errors"

	"github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
)

type targetOutcome struct {
	status, code, remainingCode string
	stop                        bool
}

func terminal(status, code string) targetOutcome { return targetOutcome{status: status, code: code} }

// target performs read-only preconditions before persisting running immediately before the write.
func (w *Worker) target(ctx context.Context, q store.DBTX, op *adminstore.Execution, access *administration.Access, position int, login string) (targetOutcome, error) {
	user, err := w.GitHub.UserByLogin(ctx, access.Token, login)
	if err != nil {
		return precheckFailure(err, "user_not_found"), nil
	}
	if user == nil || user.ID <= 0 || user.Login == "" {
		return targetOutcome{code: "github_unavailable", stop: true}, nil
	}
	if user.Type != "User" {
		return terminal("failed", "not_a_user"), nil
	}
	// Use the resolved account login for all subsequent preconditions and writes.
	login = user.Login
	switch op.Kind {
	case administration.KindInvite:
		membership, err := w.GitHub.MemberMembership(ctx, access.Token, access.Organization, login)
		if err == nil {
			if membership != nil && membership.State == "active" {
				return terminal("unchanged", "already_member"), nil
			}
			if membership != nil && membership.State == "pending" {
				return terminal("unchanged", "already_invited"), nil
			}
			return targetOutcome{code: "github_unavailable", stop: true}, nil
		}
		if !github.IsKind(err, github.KindNotFound) {
			return precheckFailure(err, "user_not_found"), nil
		}
	case administration.KindTeamMembership:
		if op.TeamSlug == nil {
			return targetOutcome{}, errExecution
		}
		team, err := w.GitHub.OrganizationTeam(ctx, access.Token, access.Organization, *op.TeamSlug)
		if err != nil {
			outcome := precheckFailure(err, "team_not_found")
			outcome.stop = true // The team is a batch-wide precondition.
			return outcome, nil
		}
		if team == nil || team.ID <= 0 || team.Slug != *op.TeamSlug {
			return targetOutcome{code: "github_unavailable", stop: true}, nil
		}
		membership, err := w.GitHub.TeamMembership(ctx, access.Token, access.Organization, *op.TeamSlug, login)
		if err == nil {
			if membership != nil && (membership.State == "active" || membership.State == "pending") {
				// In particular, never PUT member over an existing maintainer.
				return terminal("unchanged", "already_team_member"), nil
			}
			return targetOutcome{code: "github_unavailable", stop: true}, nil
		}
		if !github.IsKind(err, github.KindNotFound) {
			return precheckFailure(err, "user_not_found"), nil
		}
	case administration.KindOrganizationRole:
		if op.Role == nil {
			return targetOutcome{}, errExecution
		}
		if *op.Role == administration.RoleMember && user.ID == access.Actor.GitHubID {
			return terminal("failed", "self_demotion_forbidden"), nil
		}
		membership, err := w.GitHub.MemberMembership(ctx, access.Token, access.Organization, login)
		if github.IsKind(err, github.KindNotFound) {
			return terminal("failed", "active_member_required"), nil
		}
		if err != nil {
			return precheckFailure(err, "active_member_required"), nil
		}
		if membership == nil || membership.State != "active" {
			// Reject known nonmembers; GitHub offers no conditional role PUT against removal races.
			return terminal("failed", "active_member_required"), nil
		}
		if *op.Role == administration.RoleMember && membership.User != nil && membership.User.ID == access.Actor.GitHubID {
			return terminal("failed", "self_demotion_forbidden"), nil
		}
		if membership.Role == *op.Role {
			return terminal("unchanged", "role_unchanged"), nil
		}
	default:
		return targetOutcome{}, errExecution
	}
	if err := ctx.Err(); err != nil {
		return targetOutcome{}, errExecution
	}
	if err := adminstore.StartTarget(ctx, q, op.ID, position); err != nil {
		return targetOutcome{}, errExecution
	}
	var membership *models.Membership
	switch op.Kind {
	case administration.KindInvite:
		err = w.GitHub.InviteOrganizationMember(ctx, access.Token, access.Organization, user.ID)
	case administration.KindTeamMembership:
		membership, err = w.GitHub.AddTeamMember(ctx, access.Token, access.Organization, *op.TeamSlug, login)
	case administration.KindOrganizationRole:
		membership, err = w.GitHub.SetOrganizationRole(ctx, access.Token, access.Organization, login, *op.Role)
	}
	if err != nil {
		return writeFailure(ctx, err), nil
	}
	switch op.Kind {
	case administration.KindInvite:
		return terminal("invited", "invitation_sent"), nil
	case administration.KindTeamMembership:
		if membership != nil && membership.State == "active" {
			return terminal("succeeded", "team_member_added"), nil
		}
		if membership != nil && membership.State == "pending" {
			return terminal("invited", "team_invitation_sent"), nil
		}
	case administration.KindOrganizationRole:
		if membership != nil && membership.State != "active" {
			// Membership can change between the active GET and PUT. Never report a role
			// change or attempt a rollback/retry when GitHub instead returns an invitation.
			return targetOutcome{status: "unknown", code: "membership_changed", remainingCode: "membership_changed", stop: true}, nil
		}
		if membership != nil && membership.State == "active" && membership.Role == *op.Role {
			return terminal("succeeded", "role_changed"), nil
		}
	}
	// A successful response with unexpected state is still an attempted, non-replayable write.
	return targetOutcome{status: "unknown", code: "outcome_unknown", remainingCode: "github_unavailable", stop: true}, nil
}

func precheckFailure(err error, missingCode string) targetOutcome {
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return targetOutcome{code: "github_unavailable", stop: true}
	}
	switch apiErr.Kind {
	case github.KindNotFound:
		return terminal("failed", missingCode)
	case github.KindUnauthorized:
		return targetOutcome{code: "github_reauthentication_required", stop: true}
	case github.KindForbidden:
		return targetOutcome{code: "github_forbidden", stop: true}
	case github.KindRateLimited:
		return targetOutcome{code: "github_rate_limited", stop: true}
	case github.KindInvalid:
		if apiErr.Status == 422 {
			return terminal("failed", "github_invalid")
		}
	}
	return targetOutcome{code: "github_unavailable", stop: true}
}

func writeFailure(ctx context.Context, err error) targetOutcome {
	remainingCode := "github_unavailable"
	if ctx.Err() != nil {
		remainingCode = "interrupted"
	}
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || apiErr.Kind == github.KindTransient || (apiErr.Status >= 200 && apiErr.Status < 300) {
		return targetOutcome{status: "unknown", code: "outcome_unknown", remainingCode: remainingCode, stop: true}
	}
	switch apiErr.Kind {
	case github.KindUnauthorized:
		return targetOutcome{status: "failed", code: "github_reauthentication_required", stop: true}
	case github.KindForbidden:
		return targetOutcome{status: "failed", code: "github_forbidden", stop: true}
	case github.KindRateLimited:
		return targetOutcome{status: "failed", code: "github_rate_limited", stop: true}
	case github.KindNotFound:
		return terminal("failed", "user_not_found")
	case github.KindInvalid:
		return targetOutcome{status: "failed", code: "github_invalid", stop: apiErr.Status != 422}
	default:
		return targetOutcome{status: "unknown", code: "outcome_unknown", remainingCode: remainingCode, stop: true}
	}
}
