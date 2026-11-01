package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

const (
	// A custom score must finish inside the HTTP server's 30 s write timeout.
	scoreRequestTimeout = 25 * time.Second
	scoreStatementLimit = 20 * time.Second
)

type scoreResponse struct {
	Data  json.RawMessage     `json:"data"`
	State scores.ScoringState `json:"state"`
	Fresh bool                `json:"fresh"`
}

func (s *Server) scoreError(w http.ResponseWriter, r *http.Request, err error) {
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

func (s *Server) getScores(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
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

func (s *Server) queryScores(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	decoded, ok := readScoreQuery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), scoreRequestTimeout)
	defer cancel()
	state, err := scores.GetScoringState(ctx, s.Pool, id, u.UserID)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if state.Generation == 0 || state.ScoredGeneration != state.Generation || state.LastIngestedAt == nil {
		response.Error(w, r, http.StatusServiceUnavailable, "scores_pending", "the current scores are not ready")
		return
	}
	query := decoded.withAsOf(*state.LastIngestedAt).encode()
	sum := sha256.Sum256(query)
	key := hex.EncodeToString(sum[:])
	result, err := scores.LoadSnapshot(ctx, s.Pool, id, state.Generation, u.UserID, key)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if result == nil {
		if result, err = s.calculate(ctx, id, u.UserID, state.Generation, key, query); err != nil {
			s.scoreError(w, r, err)
			return
		}
	}
	response.JSON(w, http.StatusOK, scoreResponse{Data: result, State: state, Fresh: scoreFresh(state, s.Now())})
}

func (s *Server) calculate(ctx context.Context, id, userID, generation int64, key string, query json.RawMessage) (json.RawMessage, error) {
	if err := s.Scorer.Validate(ctx, query); err != nil {
		return nil, err
	}
	login, err := tenancy.InstallationLogin(ctx, s.Pool, id, userID)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = scorer.ReadOnly(ctx, s.Pool, scoreStatementLimit, func(tx pgx.Tx) error {
		var err error
		result, err = s.Scorer.Calculate(ctx, tx, id, userID, login, query)
		return err
	})
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		return scores.StoreSnapshot(ctx, tx, id, generation, userID, key, result, false)
	})
	return result, err
}
