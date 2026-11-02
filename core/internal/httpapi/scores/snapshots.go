package scores

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/internal/access"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
)

type scoreResponse struct {
	Data  json.RawMessage     `json:"data"`
	State scores.ScoringState `json:"state"`
	Fresh bool                `json:"fresh"`
}

func (s *Handler) Get(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := access.ResolveOrganization(s.Pool, s.Log, w, r, u)
	if !ok {
		return
	}
	state, err := scores.GetScoringState(r.Context(), s.Pool, id, u.UserID)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if state.ScoredGeneration == 0 {
		response.Error(w, r, http.StatusServiceUnavailable, "scores_pending", "scores are not ready")
		return
	}
	result, err := scores.LoadSnapshot(r.Context(), s.Pool, id, state.ScoredGeneration, u.UserID, "default")
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if result == nil {
		response.InternalError(s.Log, w, r, errors.New("default score snapshot missing"))
		return
	}
	response.JSON(w, http.StatusOK, scoreResponse{Data: result, State: state, Fresh: scoreFresh(state, s.Now())})
}

func scoreFresh(state scores.ScoringState, now time.Time) bool {
	return !state.IngestionFailed && state.Generation == state.ScoredGeneration && state.LastIngestedAt != nil &&
		!state.LastIngestedAt.Before(now.UTC().Add(-24*time.Hour))
}
