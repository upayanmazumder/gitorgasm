package middleware

import "net/http"

// LimitConcurrent bounds concurrent calls and stops waiting when the client disconnects.
func LimitConcurrent(n int, next http.HandlerFunc) http.HandlerFunc {
	slots := make(chan struct{}, n)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next(w, r)
		case <-r.Context().Done():
		}
	}
}
