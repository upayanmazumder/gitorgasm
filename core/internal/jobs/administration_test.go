package jobs_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

func assertAdminResults(t *testing.T, a *adminFixture, id, status, results string) {
	t.Helper()
	gotStatus, gotResults := a.results(t, id)
	if gotStatus != status || gotResults != results {
		t.Fatalf("operation = %s %s; want %s %s", gotStatus, gotResults, status, results)
	}
	_, bad := a.gh.recorded()
	if len(bad) != 0 {
		t.Fatalf("privileged requests used wrong credentials: %v", bad)
	}
}

func TestAdministrationInvitesOnlyNewUsersAndNeverReplays(t *testing.T) {
	f := newOrgGitHub()
	for i, login := range []string{"new-user", "member", "pending"} {
		f.user(login, int64(i+20), "User")
	}
	f.user("company", 80, "Organization")
	f.members["member"] = orgMember{ID: 21, State: "active", Role: "member"}
	f.members["pending"] = orgMember{ID: 22, State: "pending", Role: "member"}
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"invite","logins":["new-user","member","pending","company","missing"]}`)
	f.onWrite = func(string) {
		var status string
		if err := a.pool.QueryRow(context.Background(), `SELECT status FROM administration_targets WHERE operation_id=$1 AND position=0`, id).Scan(&status); err != nil || status != "running" {
			t.Errorf("write was not durably marked running: %s, %v", status, err)
		}
	}
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "completed", "new-user=invited:invitation_sent member=unchanged:already_member pending=unchanged:already_invited company=failed:not_a_user missing=failed:user_not_found")
	a.run(context.Background(), t, id)
	writes, _ := f.recorded()
	if len(writes) != 1 || !strings.Contains(writes[0], `POST /orgs/acme/invitations {"invitee_id":20,"role":"direct_member"}`) {
		t.Fatalf("invitation writes = %v", writes)
	}
}

func TestAdministrationTeamPreservesMaintainersAndSeparatesPending(t *testing.T) {
	f := newOrgGitHub()
	for i, login := range []string{"maintainer", "pending", "active", "outsider"} {
		f.user(login, int64(i+20), "User")
	}
	f.team["maintainer"] = orgMember{State: "active", Role: "maintainer"}
	f.team["pending"] = orgMember{State: "pending", Role: "member"}
	f.members["active"] = orgMember{ID: 22, State: "active", Role: "member"}
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"team_membership","team_slug":"core","logins":["maintainer","pending","active","outsider"]}`)
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "completed", "maintainer=unchanged:already_team_member pending=unchanged:already_team_member active=succeeded:team_member_added outsider=invited:team_invitation_sent")
	writes, _ := f.recorded()
	var maintainerRole string
	f.set(func(f *orgGitHub) { maintainerRole = f.team["maintainer"].Role })
	if len(writes) != 2 || strings.Contains(strings.Join(writes, " "), "/maintainer") || maintainerRole != "maintainer" {
		t.Fatalf("team writes demoted/replayed existing membership: %v", writes)
	}
}

func TestAdministrationRolesRequireActiveMemberAndStableSelfIdentity(t *testing.T) {
	f := newOrgGitHub()
	f.user("renamed-alice", actorGitHubID, "User")
	f.user("owner", 20, "User")
	f.user("member", 21, "User")
	f.user("pending", 22, "User")
	f.user("outsider", 23, "User")
	f.members["renamed-alice"] = orgMember{ID: actorGitHubID, State: "active", Role: "admin"}
	f.members["owner"] = orgMember{ID: 20, State: "active", Role: "admin"}
	f.members["member"] = orgMember{ID: 21, State: "active", Role: "member"}
	f.members["pending"] = orgMember{ID: 22, State: "pending", Role: "member"}
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"organization_role","role":"member","logins":["renamed-alice","owner","member","pending","outsider"]}`)
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "completed", "renamed-alice=failed:self_demotion_forbidden owner=succeeded:role_changed member=unchanged:role_unchanged pending=failed:active_member_required outsider=failed:active_member_required")
	writes, _ := f.recorded()
	if len(writes) != 1 || !strings.Contains(writes[0], "PUT /orgs/acme/memberships/owner") {
		t.Fatalf("role change invited/demoted another account: %v", writes)
	}
	promote := a.submit(t, `{"kind":"organization_role","role":"admin","logins":["member"]}`)
	a.run(context.Background(), t, promote)
	assertAdminResults(t, a, promote, "completed", "member=succeeded:role_changed")
}

func TestAdministrationStopsWithoutReplayingAmbiguousWrites(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		status                          int
		wantStatus, wantCode, remaining string
		interrupted                     bool
	}{
		{"disconnect", dropWrite, "unknown", "outcome_unknown", "github_unavailable", true},
		{"upstream outage", 503, "unknown", "outcome_unknown", "github_unavailable", true},
		{"rate limit", 429, "failed", "github_rate_limited", "github_rate_limited", true},
		{"revoked token", 401, "failed", "github_reauthentication_required", "github_reauthentication_required", true},
		{"lost permission", 403, "failed", "github_forbidden", "github_forbidden", true},
		{"invalid target", 422, "failed", "github_invalid", "", false},
		{"missing target", 404, "failed", "user_not_found", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrgGitHub()
			f.user("first", 20, "User")
			f.user("second", 21, "User")
			f.writeStatus["first"] = tc.status
			a := newAdminFixture(t, f)
			id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
			a.run(context.Background(), t, id)
			status, rest := "completed", "second=invited:invitation_sent"
			wantWrites := 2
			if tc.interrupted {
				status, rest, wantWrites = "interrupted", "second=not_attempted:"+tc.remaining, 1
			}
			assertAdminResults(t, a, id, status, "first="+tc.wantStatus+":"+tc.wantCode+" "+rest)
			a.run(context.Background(), t, id)
			writes, _ := f.recorded()
			if len(writes) != wantWrites {
				t.Fatalf("write count = %d, want %d (including duplicate delivery)", len(writes), wantWrites)
			}
			var messages string
			rows, err := a.pool.Query(context.Background(), `SELECT coalesce(code,'') FROM administration_targets WHERE operation_id=$1`, id)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var code string
				if err := rows.Scan(&code); err != nil {
					t.Fatal(err)
				}
				messages += adminstore.OutcomeMessage(code)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(messages, "private-detail") || strings.Contains(messages, actorToken) {
				t.Fatalf("private upstream data in outcomes: %s", messages)
			}
		})
	}
}

func TestAdministrationReauthorizesEachTarget(t *testing.T) {
	for _, reason := range []string{"owner", "token", "session", "grant"} {
		t.Run(reason, func(t *testing.T) {
			f := newOrgGitHub()
			f.user("first", 20, "User")
			f.user("second", 21, "User")
			a := newAdminFixture(t, f)
			code := "not_owner"
			f.onWrite = func(string) {
				switch reason {
				case "owner":
					f.ownerRole = "member"
				case "token":
					f.ownerRole = "revoked"
				case "session":
					_, err := a.pool.Exec(context.Background(), `DELETE FROM sessions WHERE id_hash=$1`, secure.Hash(actorSession))
					if err != nil {
						t.Error(err)
					}
				case "grant":
					_, err := a.pool.Exec(context.Background(), `DELETE FROM user_installations WHERE user_id=$1`, a.userID)
					if err != nil {
						t.Error(err)
					}
				}
			}
			if reason == "token" {
				code = "github_reauthentication_required"
			}
			if reason == "session" {
				code = "session_expired"
			}
			if reason == "grant" {
				code = "organization_not_found"
			}
			id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
			a.run(context.Background(), t, id)
			assertAdminResults(t, a, id, "interrupted", "first=invited:invitation_sent second=not_attempted:"+code)
			writes, _ := f.recorded()
			if len(writes) != 1 {
				t.Fatalf("wrote after actor authorization loss: %v", writes)
			}
		})
	}
}

func TestAdministrationCancellationFinalizesAttemptAndRemainder(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	f.user("second", 21, "User")
	f.writeStatus["first"] = blockWrite
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- a.worker.Work(ctx, &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: id}})
	}()
	select {
	case <-f.writing:
	case <-time.After(10 * time.Second):
		t.Fatal("worker never attempted write")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && (strings.Contains(err.Error(), actorToken) || strings.Contains(err.Error(), "private-detail")) {
			t.Fatalf("unsafe error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not finish cancellation")
	}
	assertAdminResults(t, a, id, "interrupted", "first=unknown:outcome_unknown second=not_attempted:interrupted")
	a.run(context.Background(), t, id)
	writes, _ := f.recorded()
	if len(writes) != 1 {
		t.Fatalf("cancelled write replayed: %v", writes)
	}
}

func TestAdministrationPreviouslyClaimedOperationCannotReplay(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	f.user("second", 21, "User")
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
	claimed, err := adminstore.ClaimOperation(context.Background(), a.pool, id)
	if err != nil || !claimed {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err := adminstore.StartTarget(context.Background(), a.pool, id, 0); err != nil {
		t.Fatal(err)
	}
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "interrupted", "first=unknown:outcome_unknown second=not_attempted:interrupted")
	writes, _ := f.recorded()
	if len(writes) != 0 {
		t.Fatalf("interrupted operation replayed: %v", writes)
	}
}

func TestAdministrationGlobalLockSerializesDistinctBatches(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	a := newAdminFixture(t, f)
	first := a.submit(t, `{"kind":"invite","logins":["first"]}`)
	second := a.submit(t, `{"kind":"invite","logins":["first"]}`)
	entered, release := make(chan struct{}), make(chan struct{})
	f.onWrite = func(string) { close(entered); <-release }
	done := make(chan error, 1)
	go func() {
		done <- a.worker.Work(context.Background(), &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: first}})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first worker never wrote")
	}
	// A separately constructed worker represents another process/replica sharing PostgreSQL.
	replica := *a.worker
	err := replica.Work(context.Background(), &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: second}})
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		close(release)
		t.Fatalf("contending batch = %v; want snooze", err)
	}
	status, results := a.results(t, second)
	if status != "queued" || results != "first=queued:" {
		t.Errorf("contender claimed before lock: %s %s", status, results)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	f.set(func(f *orgGitHub) { f.onWrite = nil })
	a.run(context.Background(), t, second)
	assertAdminResults(t, a, first, "completed", "first=invited:invitation_sent")
	assertAdminResults(t, a, second, "completed", "first=invited:invitation_sent")
}

func TestAdministrationPacesWritesBeforeFreshAuthorization(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	f.user("second", 21, "User")
	a := newAdminFixture(t, f)
	a.worker.WriteInterval = 100 * time.Millisecond
	var first time.Time
	f.onWrite = func(login string) {
		if login == "first" {
			first = time.Now()
			return
		}
		if delta := time.Since(first); delta < a.worker.WriteInterval {
			t.Errorf("writes only %s apart; need %s", delta, a.worker.WriteInterval)
		}
	}
	id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "completed", "first=invited:invitation_sent second=invited:invitation_sent")
	f.set(func(f *orgGitHub) {
		if len(f.ownerChecks) != 2 || f.ownerChecks[1].Sub(first) < a.worker.WriteInterval {
			t.Errorf("second owner check preceded pacing delay: %v, first write %v", f.ownerChecks, first)
		}
	})
}

func TestAdministrationRevalidatesTeamBeforeEachWrite(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	f.user("second", 21, "User")
	f.onWrite = func(string) { f.teamMissing = true }
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"team_membership","team_slug":"core","logins":["first","second"]}`)
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "interrupted", "first=invited:team_invitation_sent second=failed:team_not_found")
	writes, _ := f.recorded()
	if len(writes) != 1 {
		t.Fatalf("wrote to removed team: %v", writes)
	}
}

func TestAdministrationWriteValidationDoesNotExposeUpstreamError(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	f.writeStatus["first"] = http.StatusUnprocessableEntity
	a := newAdminFixture(t, f)
	id := a.submit(t, `{"kind":"invite","logins":["first"]}`)
	a.run(context.Background(), t, id)
	assertAdminResults(t, a, id, "completed", "first=failed:github_invalid")
}

func TestAdministrationReadFailuresNeverBecomeAttemptedWrites(t *testing.T) {
	for _, tc := range []struct {
		name, path, code string
		status           int
	}{
		{"target outage", "/users/second", "github_unavailable", 503},
		{"membership outage", "/orgs/acme/memberships/second", "github_unavailable", 503},
		{"rate limit", "/users/second", "github_rate_limited", 429},
		{"credentials", "/users/second", "github_reauthentication_required", 401},
		{"permissions", "/users/second", "github_forbidden", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrgGitHub()
			f.user("first", 20, "User")
			f.user("second", 21, "User")
			f.user("third", 22, "User")
			f.readStatus = map[string]int{tc.path: tc.status}
			a := newAdminFixture(t, f)
			id := a.submit(t, `{"kind":"invite","logins":["first","second","third"]}`)
			a.run(context.Background(), t, id)
			assertAdminResults(t, a, id, "interrupted", "first=invited:invitation_sent second=not_attempted:"+tc.code+" third=not_attempted:"+tc.code)
			writes, _ := f.recorded()
			if len(writes) != 1 {
				t.Fatalf("read failure allowed writes: %v", writes)
			}
		})
	}
}

func TestAdministrationDurableBoundaryFailureNeverReplaysWrite(t *testing.T) {
	for _, boundary := range []string{"start", "finish"} {
		t.Run(boundary, func(t *testing.T) {
			f := newOrgGitHub()
			f.user("first", 20, "User")
			f.user("second", 21, "User")
			a := newAdminFixture(t, f)
			id := a.submit(t, `{"kind":"invite","logins":["first","second"]}`)
			condition := "NEW.status = 'running'"
			if boundary == "finish" {
				condition = "OLD.status = 'running' AND NEW.status = 'invited'"
			}
			_, err := a.pool.Exec(context.Background(), `
				CREATE FUNCTION reject_admin_boundary() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN RAISE EXCEPTION 'private-detail token-alice'; END; $$;
				CREATE TRIGGER reject_admin_boundary BEFORE UPDATE ON administration_targets
				FOR EACH ROW WHEN (`+condition+`) EXECUTE FUNCTION reject_admin_boundary()`)
			if err != nil {
				t.Fatal(err)
			}
			err = a.worker.Work(context.Background(), &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: id}})
			if err == nil {
				t.Fatal("durable boundary failure was not reported")
			}
			if strings.Contains(err.Error(), actorToken) || strings.Contains(err.Error(), "private-detail") {
				t.Fatalf("database error leaked into River: %v", err)
			}
			want, writeCount := "first=not_attempted:interrupted second=not_attempted:interrupted", 0
			if boundary == "finish" {
				want, writeCount = "first=unknown:outcome_unknown second=not_attempted:interrupted", 1
			}
			assertAdminResults(t, a, id, "interrupted", want)
			a.run(context.Background(), t, id)
			writes, _ := f.recorded()
			if len(writes) != writeCount {
				t.Fatalf("write count = %d; want %d", len(writes), writeCount)
			}
		})
	}
}

func TestAdministrationMembershipChangedDuringRoleWriteIsUnknown(t *testing.T) {
	for _, response := range []string{"removed", "malformed"} {
		t.Run(response, func(t *testing.T) {
			f := newOrgGitHub()
			f.user("first", 20, "User")
			f.user("second", 21, "User")
			f.members["first"] = orgMember{ID: 20, State: "active", Role: "member"}
			f.members["second"] = orgMember{ID: 21, State: "active", Role: "member"}
			if response == "removed" {
				// GitHub's role PUT invites an account removed after the active-member GET.
				f.onWrite = func(string) { delete(f.members, "first") }
			} else {
				f.writeStatus["first"] = http.StatusOK
			}
			a := newAdminFixture(t, f)
			id := a.submit(t, `{"kind":"organization_role","role":"admin","logins":["first","second"]}`)
			a.run(context.Background(), t, id)
			assertAdminResults(t, a, id, "interrupted", "first=unknown:membership_changed second=not_attempted:membership_changed")
			a.run(context.Background(), t, id)
			writes, _ := f.recorded()
			if len(writes) != 1 {
				t.Fatalf("ambiguous role write replayed: %v", writes)
			}
		})
	}
}

func TestAdministrationLockDoesNotBlockAnotherInstallation(t *testing.T) {
	f := newOrgGitHub()
	f.user("first", 20, "User")
	a := newAdminFixture(t, f)
	ctx := context.Background()
	other := int64(20)
	if err := tenancy.UpsertInstallation(ctx, a.pool, &models.Installation{ID: other, RepositorySelection: "all", Account: &models.Account{ID: 200, Login: "globex", Type: "Organization"}}); err != nil {
		t.Fatal(err)
	}
	if err := tenancy.ReplaceUserInstallations(ctx, a.pool, a.userID, []int64{instID, other}); err != nil {
		t.Fatal(err)
	}
	first := a.submit(t, `{"kind":"invite","logins":["first"]}`)
	req, err := administration.ParseRequest([]byte(`{"kind":"invite","logins":["first"]}`))
	if err != nil {
		t.Fatal(err)
	}
	secondID := secure.RandomToken()
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		_, _, err := adminstore.InsertOperation(ctx, tx, adminstore.NewOperation{
			ID: secondID, InstallationID: other, AccountID: 200, ActorUserID: a.userID, SessionHash: secure.Hash(actorSession),
			IdempotencyKey: "00000000-0000-4000-8000-0000000000aa", Fingerprint: req.Fingerprint(),
			Kind: req.Kind, Logins: req.Logins,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	f.onWrite = func(string) { close(entered); <-release }
	done := make(chan error, 1)
	go func() {
		done <- a.worker.Work(ctx, &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: first}})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first worker never wrote")
	}
	replica := *a.worker
	err = replica.Work(ctx, &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: secondID}})
	var snooze *river.JobSnoozeError
	if errors.As(err, &snooze) {
		close(release)
		t.Fatalf("another installation's batch was snoozed by the lock: %v", err)
	}
	status, _ := a.results(t, secondID)
	if status == "queued" {
		close(release)
		t.Fatal("another installation's batch never started while the first was running")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
