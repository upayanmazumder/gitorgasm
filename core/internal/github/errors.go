package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorKind classifies GitHub failures for retry decisions.
type ErrorKind int

const (
	KindTransient    ErrorKind = iota // network errors and 5xx: retry with backoff
	KindRateLimited                   // primary or secondary rate limit: wait RetryAfter
	KindUnauthorized                  // 401: credential revoked or expired
	KindForbidden                     // 403 without rate-limit signals: permission missing
	KindNotFound                      // 404: resource gone or not visible to this credential
	KindInvalid                       // other 4xx: request is wrong, do not retry
)

// APIError is a classified GitHub error.
type APIError struct {
	Operation  string
	Status     int
	Kind       ErrorKind
	RetryAfter time.Duration
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s: status %d: %s", e.Operation, e.Status, e.Message)
}

// IsKind reports whether err is an APIError of the given kind.
func IsKind(err error, kind ErrorKind) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == kind
}

// classify prioritizes Retry-After, then rate-limit reset, then a one-minute secondary-limit delay.
func classify(op string, resp *http.Response, body []byte, now time.Time) error {
	e := &APIError{Operation: op, Status: resp.StatusCode, Message: summarize(body)}
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	retryAfter := resp.Header.Get("Retry-After")
	switch {
	case resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && (remaining == "0" || retryAfter != "" ||
			strings.Contains(strings.ToLower(e.Message), "rate limit"))):
		e.Kind = KindRateLimited
		switch {
		case retryAfter != "":
			if s, err := strconv.Atoi(retryAfter); err == nil {
				e.RetryAfter = time.Duration(s) * time.Second
			}
		case remaining == "0":
			if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				e.RetryAfter = time.Unix(reset, 0).Sub(now) + time.Second
			}
		}
		if e.RetryAfter <= 0 {
			e.RetryAfter = time.Minute
		}
	case resp.StatusCode == http.StatusUnauthorized:
		e.Kind = KindUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		e.Kind = KindForbidden
	case resp.StatusCode == http.StatusNotFound:
		e.Kind = KindNotFound
	case resp.StatusCode >= 500:
		e.Kind = KindTransient
	default:
		e.Kind = KindInvalid
	}
	return e
}

func summarize(body []byte) string {
	var m struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &m) == nil && m.Message != "" {
		return m.Message
	}
	if len(body) > 200 {
		body = body[:200]
	}
	return string(body)
}

func hasStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}
