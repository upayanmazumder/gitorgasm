package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

const maxScoreQueryBytes = 16 << 10

func readScoreQuery(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxScoreQueryBytes))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "query exceeds the size limit or cannot be read")
		return nil, false
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil || value == nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "query must be a JSON object")
		return nil, false
	}
	for key := range value {
		if key != "period" && key != "filter" && key != "rules" {
			writeError(w, r, http.StatusBadRequest, "invalid_query", "query contains an unknown field")
			return nil, false
		}
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "query is invalid")
		return nil, false
	}
	if err := scorer.Validate(r.Context(), canonical); err != nil {
		var invalid *exec.ExitError
		if errors.As(err, &invalid) {
			writeError(w, r, http.StatusBadRequest, "invalid_query", "score query is invalid")
		} else {
			writeError(w, r, http.StatusServiceUnavailable, "scores_unavailable", "score service is unavailable")
		}
		return nil, false
	}
	return canonical, true
}

type scoreResponse struct {
	Data  json.RawMessage    `json:"data"`
	State store.ScoringState `json:"state"`
	Fresh bool               `json:"fresh"`
}

func (s *Server) scoreError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		return
	}
	s.internalError(w, r, err)
}

func (s *Server) getScores(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	state, err := store.GetScoringState(r.Context(), s.Pool, id, u.UserID)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if state.ScoredGeneration == 0 {
		writeError(w, r, http.StatusServiceUnavailable, "scores_pending", "scores are not ready")
		return
	}
	result, err := store.LoadSnapshot(r.Context(), s.Pool, id, state.ScoredGeneration, u.UserID, "default")
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if result == nil {
		s.internalError(w, r, errors.New("default score snapshot missing"))
		return
	}
	writeJSON(w, http.StatusOK, scoreResponse{Data: result, State: state, Fresh: scoreFresh(state, s.Now())})
}

func scoreFresh(state store.ScoringState, now time.Time) bool {
	return !state.IngestionFailed && state.Generation == state.ScoredGeneration && state.LastIngestedAt != nil &&
		!state.LastIngestedAt.Before(now.UTC().Add(-24*time.Hour))
}

func (s *Server) queryScores(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	query, ok := readScoreQuery(w, r)
	if !ok {
		return
	}
	state, err := store.GetScoringState(r.Context(), s.Pool, id, u.UserID)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if state.Generation == 0 || state.ScoredGeneration != state.Generation {
		writeError(w, r, http.StatusServiceUnavailable, "scores_pending", "the current scores are not ready")
		return
	}
	sum := sha256.Sum256(query)
	key := hex.EncodeToString(sum[:])
	result, err := store.LoadSnapshot(r.Context(), s.Pool, id, state.Generation, u.UserID, key)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if result == nil {
		login, err := store.InstallationLogin(r.Context(), s.Pool, id, u.UserID)
		if err != nil {
			s.scoreError(w, r, err)
			return
		}
		result, err = scorer.Calculate(r.Context(), s.Pool, id, u.UserID, login, query)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		err = pgx.BeginFunc(r.Context(), s.Pool, func(tx pgx.Tx) error {
			return store.StoreSnapshot(r.Context(), tx, id, state.Generation, u.UserID, key, result, false)
		})
		if errors.Is(err, store.ErrStaleGeneration) {
			writeError(w, r, http.StatusServiceUnavailable, "scores_pending", "the scores changed during this request")
			return
		}
		if err != nil {
			s.scoreError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, scoreResponse{Data: result, State: state, Fresh: scoreFresh(state, s.Now())})
}

func (s *Server) listScorePresets(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	presets, err := store.ListScoringPresets(r.Context(), s.Pool, id, u.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": presets})
}

func (s *Server) saveScorePreset(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if len(name) < 1 || len(name) > 80 {
		writeError(w, r, http.StatusBadRequest, "invalid_name", "preset name must contain 1 to 80 characters")
		return
	}
	query, ok := readScoreQuery(w, r)
	if !ok {
		return
	}
	if err := store.UpsertScoringPreset(r.Context(), s.Pool, id, u.UserID, name, query); err != nil {
		s.scoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, store.ScoringPreset{Name: name, Query: query})
}

func (s *Server) deleteScorePreset(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	deleted, err := store.DeleteScoringPreset(r.Context(), s.Pool, id, u.UserID, r.PathValue("name"))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if !deleted {
		writeError(w, r, http.StatusNotFound, "preset_not_found", "preset not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
