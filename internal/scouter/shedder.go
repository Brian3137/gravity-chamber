package scouter

import "net/http"

// Shedder bounds concurrent in-flight requests. Past the limit it rejects
// IMMEDIATELY rather than queueing — a request that would time out anyway
// is better rejected now, while the client can still do something about it.
type Shedder struct {
	sem chan struct{}
}

func NewShedder(max int) *Shedder {
	return &Shedder{sem: make(chan struct{}, max)}
}

func (s *Shedder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never shed health checks — losing them makes the load balancer
		// remove you, which is the opposite of what an overloaded but
		// working service needs.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeErr(w, http.StatusServiceUnavailable, "overloaded, retry shortly")
		}
	})
}