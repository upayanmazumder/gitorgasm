package organization

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/internal/access"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
)

type syncAccepted struct {
	JobID     int64 `json:"job_id"`
	Duplicate bool  `json:"duplicate"`
}

// Sync deduplicates pending syncs for the same installation.
func (s *Handler) Sync(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	instID, ok := access.ResolveOrganization(s.Pool, s.Log, w, r, u)
	if !ok {
		return
	}
	var res syncAccepted
	err := pgx.BeginFunc(r.Context(), s.Pool, func(tx pgx.Tx) error {
		ins, err := s.Jobs.InsertTx(r.Context(), tx, queue.SyncInstallation{InstallationID: instID}, nil)
		if err != nil {
			return err
		}
		res = syncAccepted{JobID: ins.Job.ID, Duplicate: ins.UniqueSkippedAsDuplicate}
		return audit.Audit(r.Context(), tx, audit.AuditEvent{
			ActorUserID: &u.UserID, Action: "installation.sync_requested", RequestID: requestid.From(r.Context()),
			Target: "installation:" + strconv.FormatInt(instID, 10),
		})
	})
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	response.JSON(w, http.StatusAccepted, res)
}
