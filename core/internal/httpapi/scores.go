package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

const (
	maxScoreQueryBytes = 16 << 10
	maxPresetNameRunes = 80
	// A custom score must finish inside the HTTP server's 30 s write timeout.
	scoreRequestTimeout = 25 * time.Second
	scoreStatementLimit = 20 * time.Second
)

// scoreQuery is a decoded query. Re-encoding sorts keys at every level, so equal queries
// share one cache key regardless of key order or whitespace.
type scoreQuery map[string]any

func readScoreQuery(w http.ResponseWriter, r *http.Request) (scoreQuery, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxScoreQueryBytes))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "query exceeds the size limit or cannot be read")
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var query scoreQuery
	if err := dec.Decode(&query); err != nil || query == nil || dec.More() {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "query must be one JSON object")
		return nil, false
	}
	for key := range query {
		if key != "period" && key != "filter" && key != "rules" {
			writeError(w, r, http.StatusBadRequest, "invalid_query", "query contains an unknown field")
			return nil, false
		}
	}
	return query, true
}

// withAsOf anchors a named period at asOf when the caller left the anchor out. Saved presets
// store no anchor, so they always resolve against the current data.
func (q scoreQuery) withAsOf(asOf time.Time) scoreQuery {
	period, ok := q["period"].(map[string]any)
	if !ok || period["kind"] != "preset" || period["as_of"] != nil {
		return q
	}
	out := make(scoreQuery, len(q))
	for k, v := range q {
		out[k] = v
	}
	anchored := make(map[string]any, len(period)+1)
	for k, v := range period {
		anchored[k] = v
	}
	anchored["as_of"] = asOf.UTC().Format(time.RFC3339)
	out["period"] = anchored
	return out
}

func (q scoreQuery) withoutAsOf() scoreQuery {
	period, ok := q["period"].(map[string]any)
	if !ok || period["kind"] != "preset" {
		return q
	}
	delete(period, "as_of")
	return q
}

func (q scoreQuery) encode() json.RawMessage {
	data, _ := json.Marshal(q) // Values came from json.Decoder, so encoding cannot fail.
	return data
}

func (s *Server) validateQuery(w http.ResponseWriter, r *http.Request, query json.RawMessage) bool {
	err := s.Scorer.Validate(r.Context(), query)
	if err != nil {
		s.scoreError(w, r, err)
	}
	return err == nil
}

type scoreResponse struct {
	Data  json.RawMessage    `json:"data"`
	State store.ScoringState `json:"state"`
	Fresh bool               `json:"fresh"`
}

func (s *Server) scoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
	case errors.Is(err, scorer.ErrInvalidQuery):
		writeError(w, r, http.StatusBadRequest, "invalid_query", "score query is invalid")
	case errors.Is(err, scorer.ErrBusy), errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusServiceUnavailable, "scores_busy", "score calculation is busy; try again")
	case errors.Is(err, store.ErrStaleGeneration):
		writeError(w, r, http.StatusServiceUnavailable, "scores_pending", "the scores changed during this request")
	case errors.Is(err, store.ErrPresetLimit):
		writeError(w, r, http.StatusConflict, "preset_limit", "this organization has too many presets")
	default:
		s.internalError(w, r, err)
	}
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
	decoded, ok := readScoreQuery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), scoreRequestTimeout)
	defer cancel()
	state, err := store.GetScoringState(ctx, s.Pool, id, u.UserID)
	if err != nil {
		s.scoreError(w, r, err)
		return
	}
	if state.Generation == 0 || state.ScoredGeneration != state.Generation || state.LastIngestedAt == nil {
		writeError(w, r, http.StatusServiceUnavailable, "scores_pending", "the current scores are not ready")
		return
	}
	query := decoded.withAsOf(*state.LastIngestedAt).encode()
	sum := sha256.Sum256(query)
	key := hex.EncodeToString(sum[:])
	result, err := store.LoadSnapshot(ctx, s.Pool, id, state.Generation, u.UserID, key)
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
	writeJSON(w, http.StatusOK, scoreResponse{Data: result, State: state, Fresh: scoreFresh(state, s.Now())})
}

func (s *Server) calculate(ctx context.Context, id, userID, generation int64, key string, query json.RawMessage) (json.RawMessage, error) {
	if err := s.Scorer.Validate(ctx, query); err != nil {
		return nil, err
	}
	login, err := store.InstallationLogin(ctx, s.Pool, id, userID)
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
		return store.StoreSnapshot(ctx, tx, id, generation, userID, key, result, false)
	})
	return result, err
}

// presetName trims the path value and accepts 1 to 80 printable characters.
func presetName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := strings.TrimSpace(r.PathValue("name"))
	n := utf8.RuneCountInString(name)
	valid := utf8.ValidString(name) && n >= 1 && n <= maxPresetNameRunes &&
		strings.IndexFunc(name, func(c rune) bool { return !unicode.IsPrint(c) }) < 0
	if !valid {
		writeError(w, r, http.StatusBadRequest, "invalid_name", "preset name must contain 1 to 80 printable characters")
	}
	return name, valid
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
	name, ok := presetName(w, r)
	if !ok {
		return
	}
	decoded, ok := readScoreQuery(w, r)
	if !ok {
		return
	}
	stored := decoded.withoutAsOf()
	// Validation needs an anchor for named periods; any instant checks the shape.
	if !s.validateQuery(w, r, stored.withAsOf(s.Now()).encode()) {
		return
	}
	query := stored.encode()
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
	name, ok := presetName(w, r)
	if !ok {
		return
	}
	deleted, err := store.DeleteScoringPreset(r.Context(), s.Pool, id, u.UserID, name)
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
