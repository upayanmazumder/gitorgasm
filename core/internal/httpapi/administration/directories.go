package administration

import (
	"net/http"
	"strings"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
)

type directoryUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type directoryTeam struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type userPage struct {
	Data       []directoryUser `json:"data"`
	NextCursor *string         `json:"next_cursor"`
}

type searchPage struct {
	userPage
	IncompleteResults bool `json:"incomplete_results"`
}

func normalizeSearch(raw string) (string, bool) {
	if len(raw) > 64 {
		return "", false
	}
	for _, c := range raw {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-', c == ' ':
		default:
			return "", false
		}
	}
	q := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	return q, len(q) >= 2
}

func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r, "q", "cursor") {
		return
	}
	q, ok := normalizeSearch(r.URL.Query().Get("q"))
	if !ok {
		response.Error(w, r, http.StatusBadRequest, "invalid_query", "search must contain 2 to 64 ASCII letters, digits, spaces, underscores or hyphens")
		return
	}
	c, ok := directoryCursor(w, r, u.UserID, "users", q)
	if !ok || h.authorize(w, r, u, true) == nil {
		return
	}
	// GitHub public search must never receive an installation or user token.
	users, next, incomplete, err := h.GitHub.SearchUsersPage(r.Context(), q+" type:user", c.Page)
	if err != nil {
		h.githubError(w, r, err)
		return
	}
	data := make([]directoryUser, 0, len(users))
	for _, user := range users {
		if user.Type == "User" {
			data = append(data, directoryUser{ID: user.ID, Login: user.Login})
		}
	}
	response.JSON(w, http.StatusOK, searchPage{userPage{data, nextDirectoryCursor(c, next)}, incomplete})
}

func (h *Handler) Members(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r, "role", "cursor") {
		return
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "all"
	}
	if role != "all" && role != "member" && role != "admin" {
		response.Error(w, r, http.StatusBadRequest, "invalid_role", "role must be all, member or admin")
		return
	}
	c, ok := directoryCursor(w, r, u.UserID, "members", role)
	if !ok {
		return
	}
	access := h.authorize(w, r, u, true)
	if access == nil {
		return
	}
	users, next, err := h.GitHub.OrganizationMembersPage(r.Context(), access.Token, access.Organization, role, c.Page)
	if err != nil {
		h.githubError(w, r, err)
		return
	}
	data := make([]directoryUser, 0, len(users))
	for _, user := range users {
		data = append(data, directoryUser{ID: user.ID, Login: user.Login})
	}
	response.JSON(w, http.StatusOK, userPage{data, nextDirectoryCursor(c, next)})
}

func (h *Handler) Teams(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r, "cursor") {
		return
	}
	c, ok := directoryCursor(w, r, u.UserID, "teams", "")
	if !ok {
		return
	}
	access := h.authorize(w, r, u, true)
	if access == nil {
		return
	}
	teams, next, err := h.GitHub.OrganizationTeamsPage(r.Context(), access.Token, access.Organization, c.Page)
	if err != nil {
		h.githubError(w, r, err)
		return
	}
	data := make([]directoryTeam, 0, len(teams))
	for _, team := range teams {
		data = append(data, directoryTeam{ID: team.ID, Name: team.Name, Slug: team.Slug})
	}
	response.JSON(w, http.StatusOK, struct {
		Data       []directoryTeam `json:"data"`
		NextCursor *string         `json:"next_cursor"`
	}{data, nextDirectoryCursor(c, next)})
}
