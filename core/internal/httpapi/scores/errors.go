package scores

import (
	"context"
	"errors"
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
)

func (s *Handler) scoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		response.Error(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
	case errors.Is(err, scorer.ErrInvalidQuery):
		response.Error(w, r, http.StatusBadRequest, "invalid_query", "score query is invalid")
	case errors.Is(err, scorer.ErrBusy), errors.Is(err, context.DeadlineExceeded):
		response.Error(w, r, http.StatusServiceUnavailable, "scores_busy", "score calculation is busy; try again")
	case errors.Is(err, scores.ErrStaleGeneration):
		response.Error(w, r, http.StatusServiceUnavailable, "scores_pending", "the scores changed during this request")
	case errors.Is(err, scores.ErrPresetLimit):
		response.Error(w, r, http.StatusConflict, "preset_limit", "this organization has too many presets")
	default:
		response.InternalError(s.Log, w, r, err)
	}
}
