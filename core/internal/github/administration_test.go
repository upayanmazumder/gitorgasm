package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

func TestAdministrationRequestsUseUserCredentialsAndDocumentedBodies(t *testing.T) {
	tests := []struct {
		name, method, path, body, response string
		call                               func(*Client) error
	}{
		{"own membership", "GET", "/user/memberships/orgs/acme", "", `{"state":"active","role":"admin","organization":{"id":1,"login":"acme"}}`, func(c *Client) error {
			_, err := c.OrganizationMembership(context.Background(), "exact-session", "acme")
			return err
		}},
		{"user", "GET", "/users/mona_shortcode", "", `{"id":2,"login":"mona_shortcode","type":"User"}`, func(c *Client) error {
			u, err := c.UserByLogin(context.Background(), "exact-session", "mona_shortcode")
			if err == nil && u.Type != "User" {
				t.Error("lost user type")
			}
			return err
		}},
		{"team", "GET", "/orgs/acme/teams/core", "", `{"id":3,"name":"Core","slug":"core"}`, func(c *Client) error {
			_, err := c.OrganizationTeam(context.Background(), "exact-session", "acme", "core")
			return err
		}},
		{"membership", "GET", "/orgs/acme/memberships/mona", "", `{"state":"pending","role":"member"}`, func(c *Client) error {
			_, err := c.MemberMembership(context.Background(), "exact-session", "acme", "mona")
			return err
		}},
		{"team membership", "GET", "/orgs/acme/teams/core/memberships/mona", "", `{"state":"active","role":"maintainer"}`, func(c *Client) error {
			m, err := c.TeamMembership(context.Background(), "exact-session", "acme", "core", "mona")
			if err == nil && m.Role != "maintainer" {
				t.Error("lost maintainer role")
			}
			return err
		}},
		{"invite", "POST", "/orgs/acme/invitations", `{"invitee_id":2,"role":"direct_member"}`, `{"id":9,"email":"private@example.com"}`, func(c *Client) error {
			return c.InviteOrganizationMember(context.Background(), "exact-session", "acme", 2)
		}},
		{"set role", "PUT", "/orgs/acme/memberships/mona", `{"role":"admin"}`, `{"state":"active","role":"admin"}`, func(c *Client) error {
			_, err := c.SetOrganizationRole(context.Background(), "exact-session", "acme", "mona", "admin")
			return err
		}},
		{"add team", "PUT", "/orgs/acme/teams/core/memberships/mona", `{"role":"member"}`, `{"state":"pending","role":"member"}`, func(c *Client) error {
			_, err := c.AddTeamMember(context.Background(), "exact-session", "acme", "core", "mona")
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tt.method || r.URL.Path != tt.path || r.Header.Get("Authorization") != "Bearer exact-session" || r.Header.Get("X-GitHub-Api-Version") != APIVersion {
					t.Errorf("unexpected request: %s %s headers %v", r.Method, r.URL.Path, r.Header)
				}
				if tt.body != "" {
					var got, want map[string]any
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Error(err)
					}
					_ = json.Unmarshal([]byte(tt.body), &want)
					gb, _ := json.Marshal(got)
					wb, _ := json.Marshal(want)
					if string(gb) != string(wb) || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("body=%s want=%s", gb, wb)
					}
				}
				if tt.method == "POST" {
					w.WriteHeader(http.StatusCreated)
				}
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()
			c := New(&config.GitHub{APIBaseURL: srv.URL, RequestTimeout: time.Second})
			if err := tt.call(c); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("made %d requests", calls)
			}
		})
	}
}

func TestAdministrationDirectoriesAreBoundedAndPublicSearchIsUnauthenticated(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("per_page") != "30" {
			t.Errorf("unbounded or wrong page: %s", r.URL.RawQuery)
		}
		w.Header().Set("Link", `<https://evil.example/steal>; rel="next"`)
		switch r.URL.Path {
		case "/search/users":
			if r.Header.Get("Authorization") != "" || r.URL.Query().Get("q") != "octo cat type:user" {
				t.Errorf("search leaked auth or changed query: %v %s", r.Header, r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"incomplete_results":true,"items":[{"id":1,"login":"octocat","type":"User","email":"private@example.com"}]}`))
		case "/orgs/acme/members":
			if r.Header.Get("Authorization") != "Bearer exact-session" || r.URL.Query().Get("role") != "admin" {
				t.Error("wrong member credential/filter")
			}
			_, _ = w.Write([]byte(`[{"id":1,"login":"octocat","type":"User"}]`))
		case "/orgs/acme/teams":
			if r.Header.Get("Authorization") != "Bearer exact-session" {
				t.Error("wrong team credential")
			}
			_, _ = w.Write([]byte(`[{"id":1,"name":"Core","slug":"core","description":"private"}]`))
		default:
			t.Errorf("followed pagination URL: %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New(&config.GitHub{APIBaseURL: srv.URL, RequestTimeout: time.Second})
	users, next, incomplete, err := c.SearchUsersPage(context.Background(), "octo cat type:user", 2)
	if err != nil || !next || !incomplete || len(users) != 1 {
		t.Fatalf("search result=%v %v %v %v", users, next, incomplete, err)
	}
	members, next, err := c.OrganizationMembersPage(context.Background(), "exact-session", "acme", "admin", 2)
	if err != nil || !next || len(members) != 1 {
		t.Fatalf("members=%v next=%v err=%v", members, next, err)
	}
	teams, next, err := c.OrganizationTeamsPage(context.Background(), "exact-session", "acme", 2)
	if err != nil || !next || len(teams) != 1 {
		t.Fatalf("teams=%v next=%v err=%v", teams, next, err)
	}
	if calls != 3 {
		t.Fatalf("eagerly followed %d requests", calls)
	}
	encoded, _ := json.Marshal(teams)
	if strings.Contains(string(encoded), "private") {
		t.Fatal("exposed private team details")
	}
}

func TestAdministrationClientRejectsUnsafeArgumentsBeforeCallingGitHub(t *testing.T) {
	c := New(&config.GitHub{APIBaseURL: "http://127.0.0.1:1", RequestTimeout: time.Second})
	ctx := context.Background()
	for _, call := range []func() error{
		func() error { _, err := c.MemberMembership(ctx, "token", "acme", "../owner"); return err },
		func() error { _, err := c.SetOrganizationRole(ctx, "token", "acme", "user", "maintainer"); return err },
		func() error { return c.InviteOrganizationMember(ctx, "token", "acme", 0) },
		func() error { _, _, err := c.OrganizationTeamsPage(ctx, "token", "acme", 0); return err },
		func() error { _, _, _, err := c.SearchUsersPage(ctx, "q", 10001); return err },
		func() error { _, err := c.UserByLogin(ctx, "", "mona"); return err },
	} {
		var api *APIError
		err := call()
		if !errors.As(err, &api) || api.Kind != KindInvalid || api.Status != 0 {
			t.Errorf("argument reached transport: %v", err)
		}
	}
}
