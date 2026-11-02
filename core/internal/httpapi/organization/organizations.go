package organization

import (
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

type listResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func (s *Handler) List(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	orgs, err := tenancy.ListOrganizations(r.Context(), s.Pool, u.UserID)
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, listResponse[tenancy.Organization]{Data: orgs})
}
