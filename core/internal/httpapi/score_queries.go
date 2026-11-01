package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

const maxScoreQueryBytes = 16 << 10

// scoreQuery is a decoded query. Re-encoding sorts keys at every level, so equal queries
// share one cache key regardless of key order or whitespace.
type scoreQuery map[string]any

func readScoreQuery(w http.ResponseWriter, r *http.Request) (scoreQuery, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxScoreQueryBytes))
	if err != nil {
		response.Error(w, r, http.StatusBadRequest, "invalid_query", "query exceeds the size limit or cannot be read")
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var query scoreQuery
	if err := dec.Decode(&query); err != nil || query == nil || dec.More() {
		response.Error(w, r, http.StatusBadRequest, "invalid_query", "query must be one JSON object")
		return nil, false
	}
	for key := range query {
		if key != "period" && key != "filter" && key != "rules" {
			response.Error(w, r, http.StatusBadRequest, "invalid_query", "query contains an unknown field")
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
