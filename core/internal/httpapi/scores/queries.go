package scores

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/internal/access"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

const (
	// A custom score must finish inside the HTTP server's 30 s write timeout.
	scoreRequestTimeout = 25 * time.Second
	scoreStatementLimit = 20 * time.Second
)

func (s *Handler) Query(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := access.ResolveOrganization(s.Pool, s.Log, w, r, u)
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

func (s *Handler) calculate(ctx context.Context, id, userID, generation int64, key string, query json.RawMessage) (json.RawMessage, error) {
	if err := s.Scorer.Validate(ctx, id, userID, query); err != nil {
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
