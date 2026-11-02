package integration_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type capabilities struct {
	Data struct {
		Organization string `json:"organization"`
		Actor        struct {
			GitHubID int64  `json:"github_id"`
			Login    string `json:"login"`
		} `json:"actor"`
		CanManage    bool    `json:"can_manage"`
		Reason       *string `json:"reason"`
		MaxBatchSize int     `json:"max_batch_size"`
	} `json:"data"`
}

func TestAdministrationCapabilitiesUseLiveOwnerAndSessionToken(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice-1")
	aliceSecond := h.addSession(h.userID(1), "token-alice-2")
	bob := h.seedActor(2, "bob", "token-bob")
	outsider := h.seedTenant(3, "globex", 200, 20)
	gh.set(func(f *adminGitHub) {
		f.roles["token-alice-1"], f.roles["token-alice-2"], f.roles["token-bob"] = "admin", "admin", "member"
	})

	rec := h.do("GET", "/v1/organizations/ACME/administration", alice, nil, nil)
	got := decode[capabilities](t, rec)
	if rec.Code != http.StatusOK || !got.Data.CanManage || got.Data.Reason != nil || got.Data.Organization != "acme" ||
		got.Data.Actor.GitHubID != 1 || got.Data.Actor.Login != "alice" || got.Data.MaxBatchSize != 50 {
		t.Fatalf("owner capabilities = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "token-alice") {
		t.Fatal("capabilities exposed a GitHub token")
	}
	// Each session opens only its own sealed token, never another session of the same user.
	h.do("GET", "/v1/organizations/acme/administration", aliceSecond, nil, nil)
	_, tokens, _, _ := gh.snapshot()
	if len(tokens) != 2 || tokens[0] != "Bearer token-alice-1" || tokens[1] != "Bearer token-alice-2" {
		t.Fatalf("user tokens sent = %q", tokens)
	}

	reasons := []struct {
		name, want string
		session    string
		change     func(*adminGitHub)
	}{
		{"member", "not_owner", bob, func(*adminGitHub) {}},
		{"members read only", "members_permission_required", alice, func(f *adminGitHub) { f.members = "read" }},
		{"suspended", "installation_suspended", alice, func(f *adminGitHub) { f.members, f.suspended = "write", true }},
	}
	for _, tc := range reasons {
		gh.set(tc.change)
		rec := h.do("GET", "/v1/organizations/acme/administration", tc.session, nil, nil)
		got := decode[capabilities](t, rec)
		if rec.Code != http.StatusOK || got.Data.CanManage || got.Data.Reason == nil || *got.Data.Reason != tc.want {
			t.Fatalf("%s capabilities = %d %s, want reason %s", tc.name, rec.Code, rec.Body, tc.want)
		}
	}
	gh.set(func(f *adminGitHub) { f.suspended = false })

	for _, path := range []string{"teams", "members", "users?q=octo", "operations", "operations/" + strings.Repeat("a", 43)} {
		rec := h.do("GET", "/v1/organizations/acme/administration/"+path, bob, nil, nil)
		if rec.Code != http.StatusForbidden || decode[adminError](t, rec).Error.Code != "not_owner" {
			t.Fatalf("non-owner %s = %d %s", path, rec.Code, rec.Body)
		}
	}
	rec = h.do("POST", "/v1/organizations/acme/administration/operations", bob, []byte(`{"kind":"invite","logins":["octo"]}`), adminHeaders("6f9619ff-8b86-d011-b42d-00c04fc964ff"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner submit = %d %s", rec.Code, rec.Body)
	}

	// Tenant isolation comes before any GitHub request.
	before, _, _, _ := gh.snapshot()
	for _, tc := range []struct{ session, org string }{{alice, "globex"}, {outsider, "acme"}, {alice, "unknown"}} {
		for _, path := range []string{"", "/teams", "/members", "/users?q=octo", "/operations"} {
			rec := h.do("GET", "/v1/organizations/"+tc.org+"/administration"+path, tc.session, nil, nil)
			if rec.Code != http.StatusNotFound || decode[adminError](t, rec).Error.Code != "organization_not_found" {
				t.Fatalf("%s%s = %d %s, want 404", tc.org, path, rec.Code, rec.Body)
			}
		}
	}
	if after, _, _, _ := gh.snapshot(); len(after) != len(before) {
		t.Fatalf("inaccessible organizations caused GitHub calls: %q", after[len(before):])
	}

	// A revoked or expired user token fails closed and asks for a new sign-in.
	gh.set(func(f *adminGitHub) { f.roles["token-alice-1"] = "revoked" })
	rec = h.do("GET", "/v1/organizations/acme/administration", alice, nil, nil)
	if rec.Code != http.StatusUnauthorized || decode[adminError](t, rec).Error.Code != "github_reauthentication_required" || strings.Contains(rec.Body.String(), "private-detail") {
		t.Fatalf("revoked token = %d %s", rec.Code, rec.Body)
	}
	gh.set(func(f *adminGitHub) { f.roles["token-alice-1"] = "admin" })
	h.exec(`UPDATE sessions SET github_token_expires_at = now() - interval '1 second' WHERE user_id = $1`, h.userID(1))
	before, _, _, _ = gh.snapshot()
	rec = h.do("GET", "/v1/organizations/acme/administration", alice, nil, nil)
	if rec.Code != http.StatusUnauthorized || decode[adminError](t, rec).Error.Code != "github_reauthentication_required" {
		t.Fatalf("expired token = %d %s", rec.Code, rec.Body)
	}
	if after, _, _, _ := gh.snapshot(); len(after) != len(before) {
		t.Fatal("an expired token was sent to GitHub")
	}
	h.exec(`UPDATE sessions SET expires_at = now() - interval '1 second' WHERE user_id = $1`, h.userID(1))
	if rec := h.do("GET", "/v1/organizations/acme/administration", alice, nil, nil); rec.Code != http.StatusUnauthorized || decode[adminError](t, rec).Error.Code != "unauthenticated" {
		t.Fatalf("expired session = %d %s", rec.Code, rec.Body)
	}
}

func TestAdministrationOperationRequestsAreStrictAndIdempotent(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	carol := h.seedActor(3, "carol", "token-carol")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"], f.roles["token-carol"] = "admin", "admin" })
	const path = "/v1/organizations/acme/administration/operations"
	const key = "6f9619ff-8b86-d011-b42d-00c04fc964ff"
	body := []byte(`{"kind":"organization_role","role":"admin","logins":["@Octo-Cat"," octo-cat","hubot"]}`)
	ctx := context.Background()
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := h.pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if rec := h.do("POST", path, alice, body, map[string]string{"Origin": "https://evil.example", "Idempotency-Key": key}); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin submit = %d", rec.Code)
	}
	for _, k := range []string{"", "not-a-uuid"} {
		if rec := h.do("POST", path, alice, body, adminHeaders(k)); rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_idempotency_key" {
			t.Fatalf("key %q = %d %s", k, rec.Code, rec.Body)
		}
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = fmt.Sprintf("user%d", i)
	}
	manyJSON, _ := json.Marshal(many)
	invalid := []struct {
		body   string
		status int
		code   string
	}{
		{`{"kind":"invite","logins":["octo"],"extra":true}`, http.StatusBadRequest, "invalid_request"},
		{`{"kind":"invite","logins":["octo"],"role":"admin"}`, http.StatusBadRequest, "invalid_request"},
		{`{"kind":"organization_role","logins":["octo"]}`, http.StatusBadRequest, "invalid_request"},
		{`{"kind":"invite","logins":["../orgs/acme"]}`, http.StatusBadRequest, "invalid_login"},
		{`{"kind":"invite","logins":` + string(manyJSON) + `}`, http.StatusBadRequest, "too_many_logins"},
		{`{"kind":"invite","logins":["` + strings.Repeat("a", 20<<10) + `"]}`, http.StatusRequestEntityTooLarge, "request_too_large"},
		{`{"kind":"team_membership","team_slug":"missing","logins":["octo"]}`, http.StatusUnprocessableEntity, "team_not_found"},
	}
	for _, tc := range invalid {
		rec := h.do("POST", path, alice, []byte(tc.body), adminHeaders(key))
		if rec.Code != tc.status || decode[adminError](t, rec).Error.Code != tc.code {
			t.Fatalf("body %.60s = %d %s, want %d %s", tc.body, rec.Code, rec.Body, tc.status, tc.code)
		}
	}
	if n := count(`SELECT count(*) FROM administration_operations`); n != 0 {
		t.Fatalf("rejected requests stored %d operations", n)
	}

	rec := h.do("POST", path, alice, body, adminHeaders(strings.ToUpper(key)))
	op := decode[struct{ Data adminOperation }](t, rec).Data
	if rec.Code != http.StatusAccepted || rec.Header().Get("Location") != path+"/"+op.ID {
		t.Fatalf("submit = %d %s location %q", rec.Code, rec.Body, rec.Header().Get("Location"))
	}
	if op.Kind != "organization_role" || op.Status != "queued" || op.Role == nil || *op.Role != "admin" || op.TeamSlug != nil ||
		op.TargetCount != 2 || len(op.Results) != 2 || op.Results[0].Login != "Octo-Cat" || op.Results[1].Login != "hubot" ||
		op.Results[0].Status != "queued" || op.Results[0].Code != nil || op.CompletedAt != nil {
		t.Fatalf("receipt = %+v", op)
	}
	var args string
	if err := h.pool.QueryRow(ctx, `SELECT args::text FROM river_job WHERE kind = 'administer_organization'`).Scan(&args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(args, op.ID) || strings.Contains(args, "token") || strings.Contains(strings.ToLower(args), "octo") {
		t.Fatalf("job args = %s, want only the operation id", args)
	}

	// The same key and request return the same receipt without another job.
	rec = h.do("POST", path, alice, []byte(`{"logins":["octo-cat","HUBOT"],"role":"admin","kind":"organization_role"}`), adminHeaders(key))
	if rec.Code != http.StatusAccepted || decode[struct{ Data adminOperation }](t, rec).Data.ID != op.ID {
		t.Fatalf("replay = %d %s", rec.Code, rec.Body)
	}
	rec = h.do("POST", path, alice, []byte(`{"kind":"organization_role","role":"member","logins":["octo-cat","hubot"]}`), adminHeaders(key))
	if rec.Code != http.StatusConflict || decode[adminError](t, rec).Error.Code != "idempotency_conflict" {
		t.Fatalf("changed payload = %d %s", rec.Code, rec.Body)
	}
	if n := count(`SELECT count(*) FROM river_job WHERE kind = 'administer_organization'`); n != 1 {
		t.Fatalf("jobs = %d, want 1", n)
	}

	// Keys are scoped to the actor. Another owner cannot read the first receipt.
	rec = h.do("POST", path, carol, body, adminHeaders(key))
	other := decode[struct{ Data adminOperation }](t, rec).Data
	if rec.Code != http.StatusAccepted || other.ID == op.ID {
		t.Fatalf("other actor submit = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", path+"/"+op.ID, carol, nil, nil); rec.Code != http.StatusNotFound || decode[adminError](t, rec).Error.Code != "operation_not_found" {
		t.Fatalf("other actor read = %d %s", rec.Code, rec.Body)
	}
	h.seedTenant(4, "globex", 200, 20)
	globex := h.addSession(h.userID(4), "token-globex")
	gh.set(func(f *adminGitHub) { f.roles["token-globex"] = "admin" })
	if rec := h.do("GET", "/v1/organizations/globex/administration/operations/"+op.ID, globex, nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-organization read = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", path+"/"+op.ID, alice, nil, nil); rec.Code != http.StatusOK || decode[struct{ Data adminOperation }](t, rec).Data.ID != op.ID {
		t.Fatalf("owner read = %d %s", rec.Code, rec.Body)
	}
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "member" })
	if rec := h.do("GET", path+"/"+op.ID, alice, nil, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("former owner read = %d %s", rec.Code, rec.Body)
	}
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })

	// A finished receipt replays with 200.
	h.exec(`UPDATE administration_targets SET status = 'succeeded', code = 'role_changed' WHERE operation_id = $1`, op.ID)
	h.exec(`UPDATE administration_operations SET status = 'completed', started_at = now(), completed_at = now() WHERE id = $1`, op.ID)
	rec = h.do("POST", path, alice, body, adminHeaders(key))
	done := decode[struct{ Data adminOperation }](t, rec).Data
	if rec.Code != http.StatusOK || done.Status != "completed" || done.CompletedAt == nil || done.Results[0].Status != "succeeded" ||
		done.Results[0].Message == nil || *done.Results[0].Message == "" {
		t.Fatalf("terminal replay = %d %s", rec.Code, rec.Body)
	}
}

func TestAdministrationReceiptsListOnlyTheActorsRecentOperations(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	carol := h.seedActor(3, "carol", "token-carol")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"], f.roles["token-carol"] = "admin", "admin" })
	const path = "/v1/organizations/acme/administration/operations"
	for i := range 12 {
		key := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		if rec := h.do("POST", path, alice, []byte(`{"kind":"invite","logins":["octo"]}`), adminHeaders(key)); rec.Code != http.StatusAccepted {
			t.Fatalf("submit %d = %d %s", i, rec.Code, rec.Body)
		}
	}
	if rec := h.do("POST", path, carol, []byte(`{"kind":"invite","logins":["octo"]}`), adminHeaders("00000000-0000-4000-8000-000000000099")); rec.Code != http.StatusAccepted {
		t.Fatalf("carol submit = %d", rec.Code)
	}
	type list struct {
		Data       []adminOperation `json:"data"`
		NextCursor *string          `json:"next_cursor"`
	}
	first := decode[list](t, h.do("GET", path, alice, nil, nil))
	if len(first.Data) != 10 || first.NextCursor == nil || first.Data[0].Results != nil {
		t.Fatalf("first page = %d items, cursor %v", len(first.Data), first.NextCursor)
	}
	if rec := h.do("GET", path+"?cursor="+*first.NextCursor, carol, nil, nil); rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_cursor" {
		t.Fatalf("cursor of another actor = %d %s", rec.Code, rec.Body)
	}
	second := decode[list](t, h.do("GET", path+"?cursor="+*first.NextCursor, alice, nil, nil))
	if len(second.Data) != 2 || second.NextCursor != nil {
		t.Fatalf("second page = %d items, cursor %v", len(second.Data), second.NextCursor)
	}
	seen := map[string]bool{}
	for _, op := range append(first.Data, second.Data...) {
		if seen[op.ID] {
			t.Fatalf("operation %s repeated across pages", op.ID)
		}
		seen[op.ID] = true
	}
	if carolList := decode[list](t, h.do("GET", path, carol, nil, nil)); len(carolList.Data) != 1 || seen[carolList.Data[0].ID] {
		t.Fatalf("carol sees %d operations", len(carolList.Data))
	}
	if rec := h.do("GET", path+"?cursor=e30", alice, nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed cursor = %d", rec.Code)
	}
}

func TestAdministrationReceiptsShowInterruptedJobs(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	const path = "/v1/organizations/acme/administration/operations"
	submit := func(i int) string {
		t.Helper()
		rec := h.do("POST", path, alice, []byte(`{"kind":"invite","logins":["octo","hubot"]}`), adminHeaders(fmt.Sprintf("00000000-0000-4000-8000-%012d", i)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("submit = %d %s", rec.Code, rec.Body)
		}
		return decode[struct{ Data adminOperation }](t, rec).Data.ID
	}
	read := func(id string) adminOperation {
		t.Helper()
		rec := h.do("GET", path+"/"+id, alice, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("read = %d %s", rec.Code, rec.Body)
		}
		return decode[struct{ Data adminOperation }](t, rec).Data
	}
	statuses := func(op adminOperation) string {
		parts := make([]string, len(op.Results))
		for i, r := range op.Results {
			code := ""
			if r.Code != nil {
				code = *r.Code
			}
			parts[i] = r.Status + ":" + code
		}
		return op.Status + " " + strings.Join(parts, ",")
	}

	missing := submit(1)
	h.exec(`DELETE FROM river_job WHERE args->>'operation_id' = $1`, missing)
	if got := statuses(read(missing)); got != "interrupted not_attempted:interrupted,not_attempted:interrupted" {
		t.Fatalf("missing job = %s", got)
	}

	discarded := submit(2)
	h.exec(`UPDATE administration_operations SET status = 'running', started_at = now() WHERE id = $1`, discarded)
	h.exec(`UPDATE administration_targets SET status = 'running' WHERE operation_id = $1 AND position = 0`, discarded)
	h.exec(`UPDATE river_job SET state = 'discarded', attempt = 1, attempted_at = now(), finalized_at = now() WHERE args->>'operation_id' = $1`, discarded)
	op := read(discarded)
	if got := statuses(op); got != "interrupted unknown:outcome_unknown,not_attempted:interrupted" || op.CompletedAt == nil {
		t.Fatalf("discarded job = %s, completed_at %v", got, op.CompletedAt)
	}

	stuck := submit(3)
	h.exec(`UPDATE administration_operations SET status = 'running', started_at = now() WHERE id = $1`, stuck)
	h.exec(`UPDATE river_job SET state = 'running', attempt = 1, attempted_at = now() WHERE args->>'operation_id' = $1`, stuck)
	if got := statuses(read(stuck)); got != "running queued:,queued:" {
		t.Fatalf("live running job = %s", got)
	}
	h.exec(`UPDATE river_job SET attempted_at = now() - interval '31 minutes' WHERE args->>'operation_id' = $1`, stuck)
	if got := statuses(read(stuck)); got != "interrupted not_attempted:interrupted,not_attempted:interrupted" {
		t.Fatalf("stuck job = %s", got)
	}

	// Reads derive the view. They never change stored state.
	var stored int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM administration_operations WHERE status IN ('queued', 'running')`).Scan(&stored); err != nil || stored != 3 {
		t.Fatalf("stored active operations = %d (%v), want 3", stored, err)
	}
}

func TestAdministrationDirectoriesArePagedAndScoped(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	const base = "/v1/organizations/acme/administration/"
	type userPage struct {
		Data              []map[string]any `json:"data"`
		NextCursor        *string          `json:"next_cursor"`
		IncompleteResults bool             `json:"incomplete_results"`
	}

	rec := h.do("GET", base+"users?q=Octo%20Cat", alice, nil, nil)
	users := decode[userPage](t, rec)
	if rec.Code != http.StatusOK || len(users.Data) != 1 || users.Data[0]["login"] != "found1" || len(users.Data[0]) != 2 || !users.IncompleteResults || users.NextCursor == nil {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "private@example.com") {
		t.Fatal("search exposed an email address")
	}
	if rec := h.do("GET", base+"users?q=other&cursor="+*users.NextCursor, alice, nil, nil); rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_cursor" {
		t.Fatalf("cursor reused for another query = %d %s", rec.Code, rec.Body)
	}
	users = decode[userPage](t, h.do("GET", base+"users?q=octo+cat&cursor="+*users.NextCursor, alice, nil, nil))
	if len(users.Data) != 1 || users.Data[0]["login"] != "found2" || users.NextCursor != nil {
		t.Fatalf("second search page = %+v", users)
	}
	before, _, _, _ := gh.snapshot()
	for _, q := range []string{"", "octo%20type%3Aorg", "a%22b", strings.Repeat("a", 65), "octo&cursor=x"} {
		if rec := h.do("GET", base+"users?q="+q, alice, nil, nil); rec.Code != http.StatusBadRequest {
			t.Fatalf("query %q = %d", q, rec.Code)
		}
	}
	if after, _, _, _ := gh.snapshot(); len(after) != len(before) {
		t.Fatal("invalid searches reached GitHub")
	}
	_, _, searchAuth, queries := gh.snapshot()
	for _, auth := range searchAuth {
		if auth != "" {
			t.Fatalf("user search sent credentials %q", auth)
		}
	}
	if !strings.Contains(queries[0], "q=octo+cat+type%3Auser") {
		t.Fatalf("search query = %s", queries[0])
	}

	type dirPage struct {
		Data       []map[string]any `json:"data"`
		NextCursor *string          `json:"next_cursor"`
	}
	members := decode[dirPage](t, h.do("GET", base+"members?role=admin", alice, nil, nil))
	if len(members.Data) != 1 || members.Data[0]["login"] != "member1" || len(members.Data[0]) != 2 || members.NextCursor == nil {
		t.Fatalf("members = %+v", members)
	}
	if rec := h.do("GET", base+"members?role=member&cursor="+*members.NextCursor, alice, nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("members cursor reused for another role = %d", rec.Code)
	}
	if rec := h.do("GET", base+"teams?cursor="+*members.NextCursor, alice, nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("members cursor reused for teams = %d", rec.Code)
	}
	if rec := h.do("GET", base+"members?role=owner", alice, nil, nil); rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_role" {
		t.Fatalf("invalid role = %d %s", rec.Code, rec.Body)
	}
	cursor := *members.NextCursor
	for range 2 {
		members = decode[dirPage](t, h.do("GET", base+"members?role=admin&cursor="+cursor, alice, nil, nil))
		if members.NextCursor != nil {
			cursor = *members.NextCursor
		}
	}
	if members.NextCursor != nil || members.Data[0]["login"] != "member3" {
		t.Fatalf("last members page = %+v", members)
	}

	teams := decode[dirPage](t, h.do("GET", base+"teams", alice, nil, nil))
	if len(teams.Data) != 1 || teams.Data[0]["slug"] != "team-1" || teams.Data[0]["name"] != "Team 1" || len(teams.Data[0]) != 3 || teams.NextCursor == nil {
		t.Fatalf("teams = %+v", teams)
	}

	_, tokens, _, queries := gh.snapshot()
	for _, tok := range tokens {
		if tok != "Bearer token-alice" {
			t.Fatalf("directory used credential %q", tok)
		}
	}
	var sawRole bool
	for _, q := range queries {
		if strings.Contains(q, "role=admin") {
			sawRole = true
		}
	}
	if !sawRole {
		t.Fatalf("member role filter not sent upstream: %q", queries)
	}
}

func TestAdministrationInvalidInputsNeverReachGitHub(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	const base = "/v1/organizations/acme/administration"
	for _, suffix := range []string{
		"/teams?cursor=x", "/teams?cursor=", "/teams?cursor=x&cursor=y",
		"/members?role=owner", "/members?role=admin&role=member",
		"/operations?cursor=e30", "/operations?limit=500",
		"/users?q=octo&q=hubot", "/users?q=%C3%A9cto", "/users?q=octo&cursor=",
	} {
		rec := h.do("GET", base+suffix, alice, nil, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d %s", suffix, rec.Code, rec.Body)
		}
	}
	for _, body := range []string{
		`{"kind":"invite","logins":["octo"]} {}`,
		`{"kind":"invite","logins":["octo"],"team_slug":"core"}`,
	} {
		rec := h.do("POST", base+"/operations", alice, []byte(body), adminHeaders("00000000-0000-4000-8000-000000000001"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid submit = %d %s", rec.Code, rec.Body)
		}
	}
	if calls, _, _, _ := gh.snapshot(); len(calls) != 0 {
		t.Fatalf("invalid inputs caused GitHub calls: %q", calls)
	}
}

func TestAdministrationSubmissionAuditIsAtomicAndNotDuplicated(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	const path = "/v1/organizations/acme/administration/operations"
	headers := adminHeaders("00000000-0000-4000-8000-000000000001")
	body := []byte(`{"kind":"team_membership","team_slug":"core","logins":["octo"]}`)
	first := h.do("POST", path, alice, body, headers)
	op := decode[struct{ Data adminOperation }](t, first).Data
	if first.Code != http.StatusAccepted {
		t.Fatalf("submit = %d %s", first.Code, first.Body)
	}
	if rec := h.do("POST", path, alice, body, headers); rec.Code != http.StatusAccepted {
		t.Fatalf("replay = %d %s", rec.Code, rec.Body)
	}
	var count int
	var metadata string
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*), min(metadata::text) FROM audit_events WHERE action = 'organization.administration_requested' AND actor_user_id = $1 AND target = $2`, h.userID(1), "administration_operation:"+op.ID).Scan(&count, &metadata); err != nil {
		t.Fatal(err)
	}
	if count != 1 || strings.Contains(metadata, "token") || strings.Contains(metadata, "octo") {
		t.Fatalf("audit = %d %s", count, metadata)
	}
	// An audit insertion failure rolls back both the durable operation and River job.
	h.exec(`ALTER TABLE audit_events ADD CONSTRAINT reject_administration_audit CHECK (action <> 'organization.administration_requested') NOT VALID`)
	rec := h.do("POST", path, alice, body, adminHeaders("00000000-0000-4000-8000-000000000002"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("audit failure = %d %s", rec.Code, rec.Body)
	}
	for _, table := range []string{"administration_operations", "river_job"} {
		if err := h.pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s after rollback = %d (%v)", table, count, err)
		}
	}
}

func TestAdministrationDirectoryFailuresAreSanitized(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	for _, tc := range []struct {
		upstream, status int
		code             string
	}{
		{401, http.StatusUnauthorized, "github_reauthentication_required"},
		{403, http.StatusForbidden, "github_forbidden"},
		{429, http.StatusServiceUnavailable, "github_rate_limited"},
		{500, http.StatusBadGateway, "github_unavailable"},
	} {
		gh.set(func(f *adminGitHub) { f.directoryStatus = tc.upstream })
		for _, suffix := range []string{"/users?q=octo", "/members", "/teams"} {
			rec := h.do("GET", "/v1/organizations/acme/administration"+suffix, alice, nil, nil)
			if rec.Code != tc.status || decode[adminError](t, rec).Error.Code != tc.code {
				t.Fatalf("%d %s = %d %s", tc.upstream, suffix, rec.Code, rec.Body)
			}
			for _, private := range []string{"private-detail", "token-secret", "private@example.com"} {
				if strings.Contains(rec.Body.String(), private) {
					t.Fatalf("directory leaked upstream detail: %s", rec.Body)
				}
			}
		}
	}
}

func TestAdministrationDirectoryCursorsCannotCrossActorsOrOrganizations(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	carol := h.seedActor(3, "carol", "token-carol")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"], f.roles["token-carol"] = "admin", "admin" })
	const path = "/v1/organizations/acme/administration/teams"
	page := decode[struct {
		NextCursor *string `json:"next_cursor"`
	}](t, h.do("GET", path, alice, nil, nil))
	if page.NextCursor == nil {
		t.Fatal("expected a team cursor")
	}
	before, _, _, _ := gh.snapshot()
	for _, tc := range []struct{ path, actor string }{
		{path, carol},
		{"/v1/organizations/globex/administration/teams", alice},
		{"/v1/organizations/acme/administration/users?q=octo", alice},
	} {
		separator := "?"
		if strings.Contains(tc.path, "?") {
			separator = "&"
		}
		rec := h.do("GET", tc.path+separator+"cursor="+*page.NextCursor, tc.actor, nil, nil)
		if rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_cursor" {
			t.Fatalf("cross-scope cursor = %d %s", rec.Code, rec.Body)
		}
	}
	if after, _, _, _ := gh.snapshot(); len(after) != len(before) {
		t.Fatal("cross-scope cursors reached GitHub")
	}
}

func TestAdministrationTeamReplaySurvivesTeamRemoval(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	const path = "/v1/organizations/acme/administration/operations"
	headers := adminHeaders("00000000-0000-4000-8000-000000000001")
	body := []byte(`{"kind":"team_membership","team_slug":"core","logins":["octo"]}`)
	first := h.do("POST", path, alice, body, headers)
	if first.Code != http.StatusAccepted {
		t.Fatalf("submit = %d %s", first.Code, first.Body)
	}
	id := decode[struct{ Data adminOperation }](t, first).Data.ID
	gh.set(func(f *adminGitHub) { f.directoryStatus = http.StatusNotFound })
	before, _, _, _ := gh.snapshot()
	replay := h.do("POST", path, alice, body, headers)
	if replay.Code != http.StatusAccepted || decode[struct{ Data adminOperation }](t, replay).Data.ID != id {
		t.Fatalf("replay after removal = %d %s", replay.Code, replay.Body)
	}
	after, _, _, _ := gh.snapshot()
	for _, call := range after[len(before):] {
		if strings.Contains(call, "/teams/") {
			t.Fatalf("idempotent replay revalidated removed team: %s", call)
		}
	}
}

func TestAdministrationDirectoryCursorPagesAreBoundedBeforeAuthorization(t *testing.T) {
	h, gh := newAdminHarness(t)
	alice := h.seedActor(1, "alice", "token-alice")
	gh.set(func(f *adminGitHub) { f.roles["token-alice"] = "admin" })
	for _, tc := range []struct {
		resource, filter, query string
		page                    int
	}{
		{"users", "octo", "?q=octo&", 35},
		{"teams", "", "?", 10001},
		{"members", "all", "?", -1},
	} {
		payload, err := json.Marshal(map[string]any{
			"resource": tc.resource, "actor": h.userID(1), "organization": "acme",
			"filter": tc.filter, "page": tc.page,
		})
		if err != nil {
			t.Fatal(err)
		}
		raw := base64.RawURLEncoding.EncodeToString(payload)
		rec := h.do("GET", "/v1/organizations/acme/administration/"+tc.resource+tc.query+"cursor="+raw, alice, nil, nil)
		if rec.Code != http.StatusBadRequest || decode[adminError](t, rec).Error.Code != "invalid_cursor" {
			t.Fatalf("page %d = %d %s", tc.page, rec.Code, rec.Body)
		}
	}
	if calls, _, _, _ := gh.snapshot(); len(calls) != 0 {
		t.Fatalf("out-of-bounds cursors reached GitHub: %q", calls)
	}
}
