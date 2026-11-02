package access

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// ResolveOrganization hides unknown and forbidden organizations to prevent tenant enumeration.
func ResolveOrganization(pool *pgxpool.Pool, log *slog.Logger, w http.ResponseWriter, r *http.Request, u *identity.SessionUser) (int64, bool) {
	id, err := tenancy.AccessibleInstallation(r.Context(), pool, u.UserID, r.PathValue("login"))
	if errors.Is(err, store.ErrNotFound) {
		response.Error(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		return 0, false
	}
	if err != nil {
		response.InternalError(log, w, r, err)
		return 0, false
	}
	return id, true
}
