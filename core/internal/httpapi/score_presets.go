package httpapi

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
)

const maxPresetNameRunes = 80

// presetName trims the path value and accepts 1 to 80 printable characters.
func presetName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := strings.TrimSpace(r.PathValue("name"))
	n := utf8.RuneCountInString(name)
	valid := utf8.ValidString(name) && n >= 1 && n <= maxPresetNameRunes &&
		strings.IndexFunc(name, func(c rune) bool { return !unicode.IsPrint(c) }) < 0
	if !valid {
		response.Error(w, r, http.StatusBadRequest, "invalid_name", "preset name must contain 1 to 80 printable characters")
	}
	return name, valid
}

func (s *Server) listScorePresets(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	presets, err := scores.ListScoringPresets(r.Context(), s.Pool, id, u.UserID)
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": presets})
}

func (s *Server) saveScorePreset(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
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
	if err := scores.UpsertScoringPreset(r.Context(), s.Pool, id, u.UserID, name, query); err != nil {
		s.scoreError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, scores.ScoringPreset{Name: name, Query: query})
}

func (s *Server) deleteScorePreset(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	id, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	name, ok := presetName(w, r)
	if !ok {
		return
	}
	deleted, err := scores.DeleteScoringPreset(r.Context(), s.Pool, id, u.UserID, name)
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	if !deleted {
		response.Error(w, r, http.StatusNotFound, "preset_not_found", "preset not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
