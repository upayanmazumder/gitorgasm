package tenancy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// AdministrationInstallation is the tenant identity authorized by a current grant.
type AdministrationInstallation struct {
	Organization   string
	AccountID      int64
	InstallationID int64
	AccountType    string
	Suspended      bool
}

// AccessibleAdministrationInstallation resolves tenant access before any upstream call.
func AccessibleAdministrationInstallation(ctx context.Context, q store.DBTX, userID int64, login string) (*AdministrationInstallation, error) {
	var out AdministrationInstallation
	err := q.QueryRow(ctx, `
		SELECT a.login,a.github_id,i.github_id,a.type,i.suspended_at IS NOT NULL
		  FROM accounts a
		  JOIN installations i ON i.account_id=a.github_id AND i.deleted_at IS NULL
		  JOIN user_installations ui ON ui.installation_id=i.github_id AND ui.user_id=$1
		 WHERE lower(a.login)=lower($2)
		 ORDER BY a.updated_at DESC
		 LIMIT 1`, userID, login).
		Scan(&out.Organization, &out.AccountID, &out.InstallationID, &out.AccountType, &out.Suspended)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
