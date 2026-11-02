package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

const administrationPageSize = 30

var administrationSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

func invalidAdministrationArgument(operation string) error {
	return &APIError{Operation: operation, Kind: KindInvalid, Message: "invalid administration request"}
}

func validAdministrationSegments(segments ...string) bool {
	for _, segment := range segments {
		if !administrationSegment.MatchString(segment) {
			return false
		}
	}
	return true
}

// administrationRequest never acquires or substitutes installation credentials.
func (c *Client) administrationRequest(ctx context.Context, operation, method, path, token string, body any, out any) (http.Header, error) {
	if token == "" {
		return nil, invalidAdministrationArgument(operation)
	}
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, invalidAdministrationArgument(operation)
		}
	}
	req, err := c.newRequest(ctx, method, path, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(ctx, operation, "user", req, out)
}

// OrganizationMembership calls orgs/get-membership-for-authenticated-user.
func (c *Client) OrganizationMembership(ctx context.Context, token, org string) (*models.Membership, error) {
	const op = "orgs/get-membership-for-authenticated-user"
	if !validAdministrationSegments(org) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Membership
	_, err := c.administrationRequest(ctx, op, http.MethodGet, "/user/memberships/orgs/"+org, token, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UserByLogin calls users/get-by-username, permitting authorized managed-user lookup.
func (c *Client) UserByLogin(ctx context.Context, token, login string) (*models.User, error) {
	const op = "users/get-by-username"
	if !validAdministrationSegments(login) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.User
	_, err := c.administrationRequest(ctx, op, http.MethodGet, "/users/"+login, token, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func administrationPage(page int) (url.Values, bool) {
	if page < 1 || page > 10000 {
		return nil, false
	}
	return url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(administrationPageSize)}}, true
}

// SearchUsersPage calls search/users without authentication, as required by GitHub.
// q is the handler-validated public user query, including its fixed type:user restriction.
// It fetches exactly one page and never follows upstream pagination URLs.
func (c *Client) SearchUsersPage(ctx context.Context, q string, page int) ([]models.DirectoryUser, bool, bool, error) {
	const op = "search/users"
	query, ok := administrationPage(page)
	if !ok || q == "" || len(q) > 128 {
		return nil, false, false, invalidAdministrationArgument(op)
	}
	query.Set("q", q)
	req, err := c.newRequest(ctx, http.MethodGet, "/search/users?"+query.Encode(), nil)
	if err != nil {
		return nil, false, false, err
	}
	var out struct {
		Items             []models.DirectoryUser `json:"items"`
		IncompleteResults bool                   `json:"incomplete_results"`
	}
	hdr, err := c.do(ctx, op, "public", req, &out)
	if err != nil {
		return nil, false, false, err
	}
	if out.Items == nil {
		out.Items = []models.DirectoryUser{}
	}
	return out.Items, nextLink(hdr.Get("Link")) != "", out.IncompleteResults, nil
}

// OrganizationMembersPage calls orgs/list-members with the documented role filter.
func (c *Client) OrganizationMembersPage(ctx context.Context, token, org, role string, page int) ([]models.DirectoryUser, bool, error) {
	const op = "orgs/list-members"
	query, ok := administrationPage(page)
	if !ok || !validAdministrationSegments(org) || (role != "all" && role != "member" && role != "admin") {
		return nil, false, invalidAdministrationArgument(op)
	}
	query.Set("role", role)
	out := []models.DirectoryUser{}
	hdr, err := c.administrationRequest(ctx, op, http.MethodGet, "/orgs/"+org+"/members?"+query.Encode(), token, nil, &out)
	if err != nil {
		return nil, false, err
	}
	return out, nextLink(hdr.Get("Link")) != "", nil
}

// OrganizationTeamsPage calls teams/list and fetches exactly one bounded page.
func (c *Client) OrganizationTeamsPage(ctx context.Context, token, org string, page int) ([]models.Team, bool, error) {
	const op = "teams/list"
	query, ok := administrationPage(page)
	if !ok || !validAdministrationSegments(org) {
		return nil, false, invalidAdministrationArgument(op)
	}
	out := []models.Team{}
	hdr, err := c.administrationRequest(ctx, op, http.MethodGet, "/orgs/"+org+"/teams?"+query.Encode(), token, nil, &out)
	if err != nil {
		return nil, false, err
	}
	return out, nextLink(hdr.Get("Link")) != "", nil
}

// OrganizationTeam calls teams/get-by-name.
func (c *Client) OrganizationTeam(ctx context.Context, token, org, slug string) (*models.Team, error) {
	const op = "teams/get-by-name"
	if !validAdministrationSegments(org, slug) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Team
	_, err := c.administrationRequest(ctx, op, http.MethodGet, "/orgs/"+org+"/teams/"+slug, token, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// MemberMembership calls orgs/get-membership-for-user.
func (c *Client) MemberMembership(ctx context.Context, token, org, login string) (*models.Membership, error) {
	const op = "orgs/get-membership-for-user"
	if !validAdministrationSegments(org, login) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Membership
	_, err := c.administrationRequest(ctx, op, http.MethodGet, "/orgs/"+org+"/memberships/"+login, token, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// TeamMembership calls teams/get-membership-for-user-in-org.
func (c *Client) TeamMembership(ctx context.Context, token, org, slug, login string) (*models.Membership, error) {
	const op = "teams/get-membership-for-user-in-org"
	if !validAdministrationSegments(org, slug, login) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Membership
	_, err := c.administrationRequest(ctx, op, http.MethodGet, "/orgs/"+org+"/teams/"+slug+"/memberships/"+login, token, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// InviteOrganizationMember calls orgs/create-invitation. This sends notifications.
func (c *Client) InviteOrganizationMember(ctx context.Context, token, org string, userID int64) error {
	const op = "orgs/create-invitation"
	if !validAdministrationSegments(org) || userID <= 0 {
		return invalidAdministrationArgument(op)
	}
	body := struct {
		InviteeID int64  `json:"invitee_id"`
		Role      string `json:"role"`
	}{userID, "direct_member"}
	_, err := c.administrationRequest(ctx, op, http.MethodPost, "/orgs/"+org+"/invitations", token, body, nil)
	return err
}

// SetOrganizationRole calls orgs/set-membership-for-user. admin grants full ownership.
func (c *Client) SetOrganizationRole(ctx context.Context, token, org, login, role string) (*models.Membership, error) {
	const op = "orgs/set-membership-for-user"
	if !validAdministrationSegments(org, login) || (role != "member" && role != "admin") {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Membership
	body := struct {
		Role string `json:"role"`
	}{role}
	_, err := c.administrationRequest(ctx, op, http.MethodPut, "/orgs/"+org+"/memberships/"+login, token, body, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// AddTeamMember calls teams/add-or-update-membership-for-user-in-org with member only.
// Callers must preserve existing memberships before invoking this notification-producing write.
func (c *Client) AddTeamMember(ctx context.Context, token, org, slug, login string) (*models.Membership, error) {
	const op = "teams/add-or-update-membership-for-user-in-org"
	if !validAdministrationSegments(org, slug, login) {
		return nil, invalidAdministrationArgument(op)
	}
	var out models.Membership
	body := struct {
		Role string `json:"role"`
	}{"member"}
	_, err := c.administrationRequest(ctx, op, http.MethodPut, "/orgs/"+org+"/teams/"+slug+"/memberships/"+login, token, body, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
