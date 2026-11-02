package administration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
)

const (
	maxDirectoryPage = 10000
	maxSearchPage    = 34 // GitHub search exposes at most 1,000 results at 30 per page.
)

var operationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

type cursor struct {
	Resource     string    `json:"resource"`
	Actor        int64     `json:"actor"`
	Organization string    `json:"organization"`
	Filter       string    `json:"filter"`
	Page         int       `json:"page,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	ID           string    `json:"id,omitempty"`
}

func validateQuery(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 4096 {
		response.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid query parameters")
		return false
	}
	for key, values := range q {
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
				break
			}
		}
		if !known || len(values) != 1 || (key == "cursor" && values[0] == "") {
			response.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid query parameters")
			return false
		}
	}
	return true
}

func cursorScope(r *http.Request, actor int64, resource, filter string) cursor {
	return cursor{Resource: resource, Actor: actor, Organization: strings.ToLower(r.PathValue("login")), Filter: filter}
}

func decodeCursor(raw string, expected cursor) (cursor, bool) {
	if len(raw) > 2048 {
		return cursor{}, false
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, false
	}
	var got cursor
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&got) != nil || dec.Decode(new(any)) != io.EOF || got.Resource != expected.Resource ||
		got.Actor != expected.Actor || got.Organization != expected.Organization || got.Filter != expected.Filter {
		return cursor{}, false
	}
	if got.Resource == "operations" {
		if got.Page != 0 || got.CreatedAt.IsZero() || !operationIDPattern.MatchString(got.ID) {
			return cursor{}, false
		}
	} else if got.Page < 2 || got.Page > directoryPageLimit(got.Resource) || !got.CreatedAt.IsZero() || got.ID != "" {
		return cursor{}, false
	}
	return got, true
}

func encodeCursor(c cursor) *string {
	data, _ := json.Marshal(c) // This fixed struct contains only JSON-supported values.
	raw := base64.RawURLEncoding.EncodeToString(data)
	return &raw
}

func directoryCursor(w http.ResponseWriter, r *http.Request, actor int64, resource, filter string) (cursor, bool) {
	c := cursorScope(r, actor, resource, filter)
	c.Page = 1
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var ok bool
		c, ok = decodeCursor(raw, c)
		if !ok {
			response.Error(w, r, http.StatusBadRequest, "invalid_cursor", "the cursor does not match this directory request")
			return cursor{}, false
		}
	}
	return c, true
}

func directoryPageLimit(resource string) int {
	if resource == "users" {
		return maxSearchPage
	}
	return maxDirectoryPage
}

func nextDirectoryCursor(c cursor, next bool) *string {
	if !next || c.Page >= directoryPageLimit(c.Resource) {
		return nil
	}
	c.Page++
	return encodeCursor(c)
}

func recentCursor(w http.ResponseWriter, r *http.Request, actor int64) (*adminstore.Cursor, bool) {
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, ok := decodeCursor(raw, cursorScope(r, actor, "operations", ""))
		if !ok {
			response.Error(w, r, http.StatusBadRequest, "invalid_cursor", "the cursor does not match this operation history")
			return nil, false
		}
		return &adminstore.Cursor{CreatedAt: c.CreatedAt, ID: c.ID}, true
	}
	return nil, true
}
