package jobs_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	adminjobs "github.com/upayanmazumder/gitorgasm/core/internal/jobs/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

const (
	actorToken    = "token-alice"
	actorGitHubID = 1
	actorSession  = "alice-session"
	dropWrite     = -1 // close the connection without a response
	blockWrite    = -2 // wait until the client gives up
)

type orgMember struct {
	ID    int64
	State string
	Role  string
}

// orgGitHub models organization acme for the administration worker. Maps use lowercase logins.
type orgGitHub struct {
	key *rsa.PrivateKey

	mu          sync.Mutex
	ownerRole   string // the actor's organization role; "revoked" answers 401
	kinds       map[string]string
	ids         map[string]int64
	members     map[string]orgMember
	team        map[string]orgMember
	teamMissing bool
	writeStatus map[string]int
	readStatus  map[string]int
	ownerChecks []time.Time
	onWrite     func(login string)
	writes      []string // "METHOD path body"
	badTokens   []string
	writing     chan struct{}
}

func newOrgGitHub() *orgGitHub {
	return &orgGitHub{
		ownerRole: "admin", kinds: map[string]string{}, ids: map[string]int64{},
		members: map[string]orgMember{}, team: map[string]orgMember{}, writeStatus: map[string]int{},
		writing: make(chan struct{}, 1),
	}
}

// user registers a GitHub account. kind is User or Organization.
func (f *orgGitHub) user(login string, id int64, kind string) {
	f.kinds[strings.ToLower(login)] = kind
	f.ids[strings.ToLower(login)] = id
}

func (f *orgGitHub) set(fn func(*orgGitHub)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *orgGitHub) recorded() (writes, badTokens []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...), append([]string(nil), f.badTokens...)
}

func (f *orgGitHub) loginOf(id int64) string {
	for login, v := range f.ids {
		if v == id {
			return login
		}
	}
	return ""
}

func (f *orgGitHub) handler() http.Handler {
	mux := http.NewServeMux()
	send := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	notFound := func(w http.ResponseWriter) { send(w, http.StatusNotFound, map[string]string{"message": "Not Found"}) }
	// write records the request and applies a scripted failure. It returns false when the handler must stop.
	write := func(w http.ResponseWriter, r *http.Request, login string, body []byte) bool {
		f.writes = append(f.writes, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		if f.onWrite != nil {
			f.onWrite(login)
		}
		switch status := f.writeStatus[login]; status {
		case 0:
			return true
		case dropWrite:
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
		case blockWrite:
			f.writing <- struct{}{}
			f.mu.Unlock()
			<-r.Context().Done()
			f.mu.Lock()
		case http.StatusTooManyRequests:
			w.Header().Set("Retry-After", "60")
			send(w, status, map[string]string{"message": "secondary rate limit private-detail"})
		default:
			send(w, status, map[string]string{"message": "Validation Failed private-detail"})
		}
		return false
	}
	membership := func(m orgMember, login string) map[string]any {
		return map[string]any{
			"state": m.State, "role": m.Role, "organization": map[string]any{"id": 100, "login": "acme"},
			"user": map[string]any{"id": m.ID, "login": login},
		}
	}

	mux.HandleFunc("GET /app/installations/{id}", func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if _, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) { return &f.key.PublicKey, nil }, jwt.WithValidMethods([]string{"RS256"})); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		send(w, http.StatusOK, map[string]any{
			"id": instID, "repository_selection": "all", "permissions": map[string]string{"members": "write"},
			"account": map[string]any{"id": 100, "login": "acme", "type": "Organization"},
		})
	})
	mux.HandleFunc("GET /user/memberships/orgs/{org}", func(w http.ResponseWriter, _ *http.Request) {
		f.ownerChecks = append(f.ownerChecks, time.Now())
		switch f.ownerRole {
		case "revoked":
			send(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		case "":
			notFound(w)
		default:
			send(w, http.StatusOK, membership(orgMember{ID: actorGitHubID, State: "active", Role: f.ownerRole}, "alice"))
		}
	})
	mux.HandleFunc("GET /users/{username}", func(w http.ResponseWriter, r *http.Request) {
		login := strings.ToLower(r.PathValue("username"))
		kind, ok := f.kinds[login]
		if !ok {
			notFound(w)
			return
		}
		send(w, http.StatusOK, map[string]any{"id": f.ids[login], "login": r.PathValue("username"), "type": kind, "email": "private@example.com"})
	})
	mux.HandleFunc("GET /orgs/acme/memberships/{username}", func(w http.ResponseWriter, r *http.Request) {
		m, ok := f.members[strings.ToLower(r.PathValue("username"))]
		if !ok {
			notFound(w)
			return
		}
		send(w, http.StatusOK, membership(m, r.PathValue("username")))
	})
	mux.HandleFunc("PUT /orgs/acme/memberships/{username}", func(w http.ResponseWriter, r *http.Request) {
		login := strings.ToLower(r.PathValue("username"))
		body, _ := io.ReadAll(r.Body)
		if !write(w, r, login, body) {
			return
		}
		var in struct {
			Role string `json:"role"`
		}
		_ = json.Unmarshal(body, &in)
		m, ok := f.members[login]
		if !ok {
			m = orgMember{ID: f.ids[login], State: "pending"} // GitHub invites nonmembers here.
		}
		m.Role = in.Role
		f.members[login] = m
		send(w, http.StatusOK, membership(m, login))
	})
	mux.HandleFunc("POST /orgs/acme/invitations", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var in struct {
			InviteeID int64 `json:"invitee_id"`
		}
		_ = json.Unmarshal(body, &in)
		if !write(w, r, f.loginOf(in.InviteeID), body) {
			return
		}
		send(w, http.StatusCreated, map[string]any{"id": 9, "role": "direct_member", "login": nil, "email": nil})
	})
	mux.HandleFunc("GET /orgs/acme/teams/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if f.teamMissing || r.PathValue("slug") != "core" {
			notFound(w)
			return
		}
		send(w, http.StatusOK, map[string]any{"id": 5, "name": "Core", "slug": "core"})
	})
	mux.HandleFunc("GET /orgs/acme/teams/core/memberships/{username}", func(w http.ResponseWriter, r *http.Request) {
		m, ok := f.team[strings.ToLower(r.PathValue("username"))]
		if !ok {
			notFound(w)
			return
		}
		send(w, http.StatusOK, map[string]any{"state": m.State, "role": m.Role})
	})
	mux.HandleFunc("PUT /orgs/acme/teams/core/memberships/{username}", func(w http.ResponseWriter, r *http.Request) {
		login := strings.ToLower(r.PathValue("username"))
		body, _ := io.ReadAll(r.Body)
		if !write(w, r, login, body) {
			return
		}
		state := "pending"
		if m, ok := f.members[login]; ok && m.State == "active" {
			state = "active"
		}
		f.team[login] = orgMember{State: state, Role: "member"}
		send(w, http.StatusOK, map[string]any{"state": state, "role": "member"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if !strings.HasPrefix(r.URL.Path, "/app/") && r.Header.Get("Authorization") != "Bearer "+actorToken {
			f.badTokens = append(f.badTokens, r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-GitHub-Api-Version") != github.APIVersion {
			f.badTokens = append(f.badTokens, "missing API version")
		}
		if status := f.readStatus[r.URL.Path]; r.Method == http.MethodGet && status != 0 {
			if status == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", "60")
			}
			send(w, status, map[string]string{"message": "private-detail actor-token " + actorToken})
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type adminFixture struct {
	pool   *pgxpool.Pool
	gh     *orgGitHub
	worker *adminjobs.Worker
	userID int64
	keys   int
}

func newAdminFixture(t *testing.T, f *orgGitHub) *adminFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.key = key
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	pool := testdb.New(t)
	ctx := context.Background()
	inst := &models.Installation{ID: instID, RepositorySelection: "all", Account: &models.Account{ID: 100, Login: "acme", Type: "Organization"}}
	if err := tenancy.UpsertInstallation(ctx, pool, inst); err != nil {
		t.Fatal(err)
	}
	userID, err := identity.UpsertUser(ctx, pool, &models.User{ID: actorGitHubID, Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tenancy.ReplaceUserInstallations(ctx, pool, userID, []int64{instID}); err != nil {
		t.Fatal(err)
	}
	sealer, err := secure.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.CreateSession(ctx, pool, identity.NewSession{
		IDHash: secure.Hash(actorSession), UserID: userID, KeyVersion: secure.KeyVersion,
		TokenSealed: sealer.Seal([]byte(actorToken), []byte(strconv.FormatInt(userID, 10))), ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	gh := github.New(&config.GitHub{AppID: 42, PrivateKey: key, APIBaseURL: srv.URL, WebBaseURL: srv.URL, RequestTimeout: 5 * time.Second})
	return &adminFixture{
		pool: pool, gh: f, userID: userID,
		worker: &adminjobs.Worker{Pool: pool, GitHub: gh, Sealer: sealer, Now: time.Now, Log: slog.New(slog.DiscardHandler), WriteInterval: time.Millisecond},
	}
}

// submit stores an operation the way the API does and returns its id.
func (a *adminFixture) submit(t *testing.T, body string) string {
	t.Helper()
	req, err := administration.ParseRequest([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	a.keys++
	id := secure.RandomToken()
	ctx := context.Background()
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		_, _, err := adminstore.InsertOperation(ctx, tx, adminstore.NewOperation{
			ID: id, InstallationID: instID, AccountID: 100, ActorUserID: a.userID, SessionHash: secure.Hash(actorSession),
			IdempotencyKey: fmt.Sprintf("00000000-0000-4000-8000-%012d", a.keys), Fingerprint: req.Fingerprint(),
			Kind: req.Kind, TeamSlug: req.TeamSlug, Role: req.Role, Logins: req.Logins,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (a *adminFixture) run(ctx context.Context, t *testing.T, id string) {
	t.Helper()
	if err := a.worker.Work(ctx, &river.Job[queue.AdministerOrganization]{Args: queue.AdministerOrganization{OperationID: id}}); err != nil {
		t.Fatalf("work: %v", err)
	}
}

// results returns the operation status and "login=status:code" for every target in order.
func (a *adminFixture) results(t *testing.T, id string) (string, string) {
	t.Helper()
	ctx := context.Background()
	var status string
	if err := a.pool.QueryRow(ctx, `SELECT status FROM administration_operations WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	rows, err := a.pool.Query(ctx, `SELECT login || '=' || status || ':' || coalesce(code, '') FROM administration_targets WHERE operation_id = $1 ORDER BY position`, id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return status, strings.Join(got, " ")
}
