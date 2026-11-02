package administration

// OutcomeMessage maps persisted outcome codes to fixed product copy. Neither the
// database nor public receipts ever store or echo an upstream error message.
// Unknown codes return empty text and are rejected by mutation helpers.
func OutcomeMessage(code string) string {
	switch code {
	case "role_changed":
		return "The organization role was changed."
	case "invitation_sent":
		return "An organization invitation was sent. Membership is pending acceptance."
	case "team_member_added":
		return "The user was added to the team."
	case "team_invitation_sent":
		return "A team invitation was sent. Membership is pending acceptance."
	case "already_member":
		return "The user is already an active organization member. No invitation was sent."
	case "already_invited":
		return "The user already has a pending organization invitation. No invitation was resent."
	case "already_team_member":
		return "The user already has an active or pending team membership. Their team role was not changed."
	case "role_unchanged":
		return "The user already has the requested organization role."
	case "active_member_required":
		return "Role changes require an active organization membership."
	case "self_demotion_forbidden":
		return "You cannot remove your own organization owner role with this action."
	case "user_not_found":
		return "The GitHub user could not be found."
	case "not_a_user":
		return "This account is not a GitHub user."
	case "team_not_found":
		return "The selected team could not be found."
	case "github_reauthentication_required":
		return "Sign in with GitHub again before submitting another operation."
	case "not_owner":
		return "Organization owner access is required."
	case "members_permission_required":
		return "The GitHub App requires Organization Members write permission."
	case "unsupported_account":
		return "Organization administration is not supported for this account."
	case "installation_suspended":
		return "The GitHub App installation is suspended. Remaining users were not attempted."
	case "github_rate_limited":
		return "GitHub rate limited this operation. Remaining users were not attempted."
	case "github_unavailable":
		return "GitHub is unavailable. Remaining users were not attempted."
	case "github_forbidden":
		return "GitHub did not allow this action."
	case "github_invalid":
		return "GitHub could not accept this action for the selected user."
	case "outcome_unknown":
		return "The outcome is unknown. Check GitHub before retrying; the action may have succeeded."
	case "membership_changed":
		return "GitHub returned a changed membership state. Check the person's membership on GitHub before retrying."
	case "interrupted":
		return "The operation was interrupted before this user was attempted."
	case "session_expired":
		return "Your session expired or was revoked. Sign in again before submitting another operation."
	case "organization_not_found":
		return "The organization is no longer accessible."
	default:
		return ""
	}
}
