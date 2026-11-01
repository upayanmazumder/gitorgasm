package httpapi_test

import (
	"context"
	"testing"
)

type page struct {
	Data []struct {
		Login           string `json:"login"`
		Name            string `json:"name"`
		RepositoryCount int    `json:"repository_count"`
	} `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func TestOrganizationsAreTenantScoped(t *testing.T) {
	h := newHarness(t)
	alice := h.seedTenant(1, "acme", 100, 10, "api", "web", "infra")
	bob := h.seedTenant(2, "globex", 200, 20, "secret")

	if rec := h.do("GET", "/v1/organizations", "", nil, nil); rec.Code != 401 {
		t.Fatalf("anonymous = %d, want 401", rec.Code)
	}

	rec := h.do("GET", "/v1/organizations", alice, nil, nil)
	got := decode[page](t, rec)
	if rec.Code != 200 || len(got.Data) != 1 || got.Data[0].Login != "acme" || got.Data[0].RepositoryCount != 3 {
		t.Fatalf("alice orgs = %d %s", rec.Code, rec.Body)
	}

	// Alice must not see Globex, and the response must not reveal that it exists.
	rec = h.do("GET", "/v1/organizations/globex/repositories", alice, nil, nil)
	if rec.Code != 404 {
		t.Fatalf("cross-tenant read = %d, want 404", rec.Code)
	}
	rec = h.do("GET", "/v1/organizations/globex/repositories", bob, nil, nil)
	if rec.Code != 200 || len(decode[page](t, rec).Data) != 1 {
		t.Fatalf("bob own org = %d %s", rec.Code, rec.Body)
	}

	// Revoking the grant (as a re-login with fewer installations does) removes access at once.
	h.exec(`DELETE FROM user_installations WHERE installation_id = 10`)
	if rec := h.do("GET", "/v1/organizations/acme/repositories", alice, nil, nil); rec.Code != 404 {
		t.Fatalf("after revocation = %d, want 404", rec.Code)
	}
	// A deleted installation hides the organization even if a grant row remains.
	h.exec(`UPDATE installations SET deleted_at = now() WHERE github_id = 20`)
	if rec := h.do("GET", "/v1/organizations", bob, nil, nil); len(decode[page](t, rec).Data) != 0 {
		t.Fatalf("deleted installation still listed: %s", rec.Body)
	}
}

func TestReusedLoginResolvesToNewestAccount(t *testing.T) {
	h := newHarness(t)
	// Account 100 was renamed on GitHub, and account 300 took its login before 100 resynced.
	h.seedTenant(1, "acme", 100, 10, "stale")
	h.exec(`UPDATE accounts SET updated_at = now() - interval '1 day' WHERE github_id = 100`)
	session := h.seedTenant(1, "ACME", 300, 30, "fresh")
	h.exec(`INSERT INTO user_installations (user_id, installation_id) SELECT id, 10 FROM users WHERE github_id = 1`)

	rec := h.do("GET", "/v1/organizations/acme/repositories", session, nil, nil)
	got := decode[page](t, rec)
	if rec.Code != 200 || len(got.Data) != 1 || got.Data[0].Name != "fresh" {
		t.Fatalf("acme repositories = %d %s, want the newest account's repositories", rec.Code, rec.Body)
	}
}

func TestRepositoryPagination(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10, "Charlie", "alpha", "bravo", "delta", "echo")
	h.exec(`UPDATE repositories SET deleted_at = now() WHERE name = 'delta'`)

	var names []string
	cursor := ""
	for range 5 {
		path := "/v1/organizations/ACME/repositories?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		rec := h.do("GET", path, s, nil, nil)
		if rec.Code != 200 {
			t.Fatalf("page = %d %s", rec.Code, rec.Body)
		}
		p := decode[page](t, rec)
		for _, r := range p.Data {
			names = append(names, r.Name)
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	want := []string{"alpha", "bravo", "Charlie", "echo"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}

	for _, bad := range []string{"limit=0", "limit=101", "limit=x", "cursor=!!!", "cursor=e30"} {
		if rec := h.do("GET", "/v1/organizations/acme/repositories?"+bad, s, nil, nil); rec.Code != 400 {
			t.Errorf("%s = %d, want 400", bad, rec.Code)
		}
	}
}

func TestSyncRequestChecksOriginAndDeduplicates(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10)

	if rec := h.do("POST", "/v1/organizations/acme/sync", s, nil, map[string]string{"Origin": "https://evil.example"}); rec.Code != 403 {
		t.Fatalf("cross-origin = %d, want 403", rec.Code)
	}
	origin := map[string]string{"Origin": appOrigin}
	first := decode[map[string]any](t, h.do("POST", "/v1/organizations/acme/sync", s, nil, origin))
	second := decode[map[string]any](t, h.do("POST", "/v1/organizations/acme/sync", s, nil, origin))
	if first["duplicate"] != false || second["duplicate"] != true || first["job_id"] != second["job_id"] {
		t.Fatalf("first=%v second=%v", first, second)
	}
	var audits int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'installation.sync_requested'`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audit rows = %d (%v), want 2", audits, err)
	}
	if rec := h.do("POST", "/v1/organizations/globex/sync", s, nil, origin); rec.Code != 404 {
		t.Fatalf("sync of foreign org = %d, want 404", rec.Code)
	}
}
