package administration

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

const (
	MaxTargets           = 50
	KindInvite           = "invite"
	KindTeamMembership   = "team_membership"
	KindOrganizationRole = "organization_role"
	RoleMember           = "member"
	RoleAdmin            = "admin"
	CodeInvalidRequest   = "invalid_request"
	CodeInvalidLogin     = "invalid_login"
	CodeTooManyLogins    = "too_many_logins"
)

var (
	loginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)
	teamPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,99}$`)
	keyPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Request struct {
	Kind     string   `json:"kind"`
	Logins   []string `json:"logins"`
	TeamSlug string   `json:"team_slug,omitempty"`
	Role     string   `json:"role,omitempty"`
}

type InputError struct {
	Code    string
	Message string
}

func (e *InputError) Error() string { return e.Message }

func invalidRequest() error {
	return &InputError{Code: CodeInvalidRequest, Message: "Provide a valid administration action and its required fields."}
}

// ParseRequest rejects ambiguous JSON before canonicalizing the reviewed intent.
func ParseRequest(body []byte) (Request, error) {
	var req Request
	if len(body) == 0 || len(body) > 16<<10 {
		return req, invalidRequest()
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return req, invalidRequest()
	}
	seenFields := make(map[string]bool, 4)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return Request{}, invalidRequest()
		}
		name, ok := key.(string)
		if !ok || seenFields[name] {
			return Request{}, invalidRequest()
		}
		seenFields[name] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return Request{}, invalidRequest()
		}
		switch name {
		case "kind":
			err = json.Unmarshal(raw, &req.Kind)
		case "logins":
			err = json.Unmarshal(raw, &req.Logins)
		case "team_slug":
			err = json.Unmarshal(raw, &req.TeamSlug)
		case "role":
			err = json.Unmarshal(raw, &req.Role)
		default:
			return Request{}, invalidRequest()
		}
		if err != nil {
			return Request{}, invalidRequest()
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return Request{}, invalidRequest()
	}
	if _, err := dec.Token(); err != io.EOF {
		return Request{}, invalidRequest()
	}
	if len(req.Logins) == 0 {
		return Request{}, invalidRequest()
	}
	switch req.Kind {
	case KindInvite:
		if seenFields["team_slug"] || (seenFields["role"] && req.Role != RoleMember) {
			return Request{}, invalidRequest()
		}
		req.Role = RoleMember
	case KindTeamMembership:
		req.TeamSlug = strings.ToLower(strings.TrimSpace(req.TeamSlug))
		if !teamPattern.MatchString(req.TeamSlug) || (seenFields["role"] && req.Role != RoleMember) {
			return Request{}, invalidRequest()
		}
		req.Role = RoleMember
	case KindOrganizationRole:
		if seenFields["team_slug"] || (req.Role != RoleMember && req.Role != RoleAdmin) {
			return Request{}, invalidRequest()
		}
	default:
		return Request{}, invalidRequest()
	}
	seen := make(map[string]bool, len(req.Logins))
	canonical := req.Logins[:0]
	for _, login := range req.Logins {
		login = strings.TrimPrefix(strings.TrimSpace(login), "@")
		if !loginPattern.MatchString(login) {
			return Request{}, &InputError{Code: CodeInvalidLogin, Message: "Each username must be a safe GitHub username, not a URL or path."}
		}
		folded := strings.ToLower(login)
		if seen[folded] {
			continue
		}
		seen[folded] = true
		canonical = append(canonical, login)
		if len(canonical) > MaxTargets {
			return Request{}, &InputError{Code: CodeTooManyLogins, Message: "Choose at most 50 unique GitHub usernames."}
		}
	}
	req.Logins = canonical
	return req, nil
}

// Fingerprint binds idempotency to canonical intent, including stable target order.
func (r Request) Fingerprint() []byte {
	canonical := r
	canonical.Logins = make([]string, len(r.Logins))
	for i, login := range r.Logins {
		canonical.Logins[i] = strings.ToLower(login)
	}
	body, _ := json.Marshal(canonical) // Request contains only JSON-safe strings and slices.
	sum := sha256.Sum256(body)
	return sum[:]
}

func ParseIdempotencyKey(key string) (string, bool) {
	if !keyPattern.MatchString(key) {
		return "", false
	}
	return strings.ToLower(key), true
}
