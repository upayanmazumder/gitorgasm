package administration

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

type Actor struct {
	GitHubID int64  `json:"github_id"`
	Login    string `json:"login"`
}

type Access struct {
	Organization   string  `json:"organization"`
	Actor          Actor   `json:"actor"`
	CanManage      bool    `json:"can_manage"`
	Reason         *string `json:"reason"`
	MaxBatchSize   int     `json:"max_batch_size"`
	Token          string  `json:"-"`
	InstallationID int64   `json:"-"`
	AccountID      int64   `json:"-"`
	UserID         int64   `json:"-"`
}

// AccessError contains only fixed product copy, never an upstream error or token.
type AccessError struct {
	Status  int
	Code    string
	Message string
}

func (e *AccessError) Error() string { return e.Message }

func accessError(status int, code, message string) error {
	return &AccessError{Status: status, Code: code, Message: message}
}

func organizationNotFound() error {
	return accessError(http.StatusNotFound, "organization_not_found", "Organization not found.")
}

func reauthenticationRequired() error {
	return accessError(http.StatusUnauthorized, "github_reauthentication_required", "Sign in again to reconnect your GitHub account.")
}

func internalAccessError() error {
	return accessError(http.StatusInternalServerError, "internal_error", "Unable to authorize organization administration.")
}

// githubAccessError intentionally discards all raw upstream error text.
func githubAccessError(err error) error {
	var api *github.APIError
	if !errors.As(err, &api) {
		return accessError(http.StatusServiceUnavailable, "github_unavailable", "GitHub is temporarily unavailable. Try again later.")
	}
	switch api.Kind {
	case github.KindUnauthorized:
		return reauthenticationRequired()
	case github.KindRateLimited:
		return accessError(http.StatusTooManyRequests, "github_rate_limited", "GitHub has temporarily limited requests. Try again later.")
	case github.KindForbidden:
		return accessError(http.StatusForbidden, "github_forbidden", "GitHub denied access to organization administration.")
	case github.KindNotFound:
		return organizationNotFound()
	default:
		return accessError(http.StatusServiceUnavailable, "github_unavailable", "GitHub is temporarily unavailable. Try again later.")
	}
}

type Authorizer struct {
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Sealer *secure.Sealer
	Now    func() time.Time
}

// Check resolves tenant access first, then revalidates the exact actor session,
// live installation identity/permissions and live UAT owner membership on every call.
func (a *Authorizer) Check(ctx context.Context, userID int64, sessionHash []byte, organization string) (*Access, error) {
	if a.Pool == nil {
		return nil, internalAccessError()
	}
	tenant, err := tenancy.AccessibleAdministrationInstallation(ctx, a.Pool, userID, organization)
	if errors.Is(err, store.ErrNotFound) {
		return nil, organizationNotFound()
	}
	if err != nil {
		return nil, internalAccessError()
	}
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	credential, err := identity.LookupSessionCredential(ctx, a.Pool, userID, sessionHash, now)
	if errors.Is(err, store.ErrNotFound) {
		return nil, accessError(http.StatusUnauthorized, "session_expired", "Your session has expired. Sign in again.")
	}
	if err != nil {
		return nil, internalAccessError()
	}
	if credential.TokenExpiresAt != nil && !credential.TokenExpiresAt.After(now) {
		return nil, reauthenticationRequired()
	}
	if a.GitHub == nil {
		return nil, accessError(http.StatusServiceUnavailable, "github_not_configured", "The GitHub App is not configured on this deployment.")
	}
	if a.Sealer == nil {
		return nil, internalAccessError()
	}
	if credential.KeyVersion != secure.KeyVersion || len(credential.TokenSealed) == 0 {
		return nil, reauthenticationRequired()
	}
	plaintext, err := a.Sealer.Open(credential.TokenSealed, []byte(strconv.FormatInt(userID, 10)))
	if err != nil || len(plaintext) == 0 {
		return nil, reauthenticationRequired()
	}
	token := string(plaintext)
	clear(plaintext)
	inst, err := a.GitHub.GetInstallation(ctx, tenant.InstallationID)
	if err != nil {
		return nil, githubAccessError(err)
	}
	if inst.ID != tenant.InstallationID || inst.Account == nil || inst.Account.ID != tenant.AccountID || !strings.EqualFold(inst.Account.Name(), tenant.Organization) {
		return nil, organizationNotFound()
	}
	access := &Access{
		Organization: tenant.Organization, Actor: Actor{GitHubID: credential.GitHubID, Login: credential.Login}, MaxBatchSize: MaxTargets,
		InstallationID: tenant.InstallationID, AccountID: tenant.AccountID, UserID: userID,
	}
	deny := func(reason string) (*Access, error) { access.Reason = &reason; return access, nil }
	if tenant.AccountType != "Organization" || inst.Account.Kind() != "Organization" {
		return deny("unsupported_account")
	}
	if tenant.Suspended || inst.SuspendedAt != nil {
		return deny("installation_suspended")
	}
	if inst.Permissions["members"] != "write" {
		return deny("members_permission_required")
	}
	membership, err := a.GitHub.OrganizationMembership(ctx, token, tenant.Organization)
	if github.IsKind(err, github.KindNotFound) {
		return deny("not_owner")
	}
	if err != nil {
		return nil, githubAccessError(err)
	}
	if membership.Organization == nil || membership.Organization.ID != tenant.AccountID || !strings.EqualFold(membership.Organization.Login, tenant.Organization) {
		return nil, organizationNotFound()
	}
	if membership.User != nil && membership.User.ID != credential.GitHubID {
		return nil, reauthenticationRequired()
	}
	if membership.State != "active" || membership.Role != RoleAdmin {
		return deny("not_owner")
	}
	access.CanManage = true
	access.Token = token
	return access, nil
}
