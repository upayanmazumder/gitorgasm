package administration

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"

	domain "github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
)

const maxRequestBody = 16 << 10

var errTeamNotFound = errors.New("target team not found")

type operationResponse struct {
	Data adminstore.Operation `json:"data"`
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r) {
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	key, valid := domain.ParseIdempotencyKey(r.Header.Get("Idempotency-Key"))
	if len(keys) != 1 || !valid {
		response.Error(w, r, http.StatusBadRequest, "invalid_idempotency_key", "send one UUID Idempotency-Key header")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		response.Error(w, r, http.StatusUnsupportedMediaType, "invalid_request", "the request must use application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "administration requests must not exceed 16 KiB")
		} else {
			response.Error(w, r, http.StatusBadRequest, "invalid_request", "the request body could not be read")
		}
		return
	}
	input, err := domain.ParseRequest(body)
	if err != nil {
		var invalid *domain.InputError
		if errors.As(err, &invalid) {
			response.Error(w, r, http.StatusBadRequest, invalid.Code, invalid.Message)
		} else {
			response.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid administration request")
		}
		return
	}
	access := h.authorize(w, r, u, true)
	if access == nil {
		return
	}
	var teamError error
	if input.Kind == domain.KindTeamMembership {
		exists, err := adminstore.HasIdempotencyKey(r.Context(), h.Pool, u.UserID, access.InstallationID, key)
		if err != nil {
			response.InternalError(h.Log, w, r, errors.New("administration idempotency lookup failed"))
			return
		}
		if !exists {
			_, teamError = h.GitHub.OrganizationTeam(r.Context(), access.Token, access.Organization, input.TeamSlug)
			if github.IsKind(teamError, github.KindNotFound) {
				teamError = errTeamNotFound
			}
		}
	}
	var operation adminstore.Operation
	err = pgx.BeginFunc(r.Context(), h.Pool, func(tx pgx.Tx) error {
		var created bool
		var err error
		operation, created, err = adminstore.InsertOperation(r.Context(), tx, adminstore.NewOperation{
			ID: secure.RandomToken(), InstallationID: access.InstallationID, AccountID: access.AccountID,
			ActorUserID: u.UserID, SessionHash: u.SessionHash, IdempotencyKey: key, Fingerprint: input.Fingerprint(),
			Kind: input.Kind, TeamSlug: input.TeamSlug, Role: input.Role, Logins: input.Logins,
		})
		if err != nil {
			return err
		}
		if !created {
			operation, err = adminstore.GetOperation(r.Context(), tx, u.UserID, access.InstallationID, operation.ID)
			return err
		}
		if teamError != nil {
			return teamError
		}
		job, err := h.Jobs.InsertTx(r.Context(), tx, queue.AdministerOrganization{OperationID: operation.ID}, nil)
		if err != nil {
			return err
		}
		if err := adminstore.SetJobID(r.Context(), tx, operation.ID, job.Job.ID); err != nil {
			return err
		}
		return audit.Audit(r.Context(), tx, audit.AuditEvent{
			ActorUserID: &u.UserID, Action: "organization.administration_requested",
			Target: "administration_operation:" + operation.ID, RequestID: requestid.From(r.Context()),
			Metadata: map[string]any{"kind": input.Kind, "target_count": len(input.Logins), "installation_id": access.InstallationID},
		})
	})
	if err != nil {
		switch {
		case errors.Is(err, adminstore.ErrIdempotencyConflict):
			response.Error(w, r, http.StatusConflict, "idempotency_conflict", "this idempotency key was already used for a different request")
		case errors.Is(err, errTeamNotFound):
			response.Error(w, r, http.StatusUnprocessableEntity, "team_not_found", "the selected team is no longer available")
		case errors.Is(err, store.ErrNotFound):
			response.Error(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		default:
			var upstream *github.APIError
			if errors.Is(err, teamError) || errors.As(err, &upstream) {
				h.githubError(w, r, err)
			} else {
				response.InternalError(h.Log, w, r, errors.New("administration enqueue transaction failed"))
			}
		}
		return
	}
	status := http.StatusAccepted
	if operation.Status == "completed" || operation.Status == "interrupted" {
		status = http.StatusOK
	}
	w.Header().Set("Location", "/v1/organizations/"+url.PathEscape(access.Organization)+"/administration/operations/"+operation.ID)
	response.JSON(w, status, operationResponse{operation})
}

func (h *Handler) GetOperation(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r) {
		return
	}
	id := r.PathValue("id")
	if !operationIDPattern.MatchString(id) {
		response.Error(w, r, http.StatusNotFound, "operation_not_found", "operation not found")
		return
	}
	access := h.authorize(w, r, u, true)
	if access == nil {
		return
	}
	operation, err := adminstore.GetOperation(r.Context(), h.Pool, u.UserID, access.InstallationID, id)
	if errors.Is(err, store.ErrNotFound) {
		response.Error(w, r, http.StatusNotFound, "operation_not_found", "operation not found")
		return
	}
	if err != nil {
		response.InternalError(h.Log, w, r, errors.New("administration receipt lookup failed"))
		return
	}
	response.JSON(w, http.StatusOK, operationResponse{operation})
}

func (h *Handler) ListOperations(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r, "cursor") {
		return
	}
	position, ok := recentCursor(w, r, u.UserID)
	if !ok {
		return
	}
	access := h.authorize(w, r, u, true)
	if access == nil {
		return
	}
	operations, err := adminstore.ListOperations(r.Context(), h.Pool, u.UserID, access.InstallationID, position, 11)
	if err != nil {
		response.InternalError(h.Log, w, r, errors.New("administration history lookup failed"))
		return
	}
	var next *string
	if len(operations) > 10 {
		operations = operations[:10]
		last := operations[len(operations)-1]
		c := cursorScope(r, u.UserID, "operations", "")
		c.CreatedAt, c.ID = last.CreatedAt, last.ID
		next = encodeCursor(c)
	}
	if operations == nil {
		operations = []adminstore.OperationSummary{}
	}
	response.JSON(w, http.StatusOK, struct {
		Data       []adminstore.OperationSummary `json:"data"`
		NextCursor *string                       `json:"next_cursor"`
	}{operations, next})
}
