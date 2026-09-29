// Package gateway is a minimal API gateway: authenticate, rate-limit,
// tag with a request ID, route, and proxy. It inspects and rejects.
// It does not compute.
package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Route struct {
	Prefix string // path prefix to match, e.g. "/api/"
	Target string // backend base URL, e.g. "http://scouter:8080"
	Strip  bool   // strip the prefix before forwarding
	Public bool   // if false, requires an API key
}

type Config struct {
	Routes       []Route
	APIKeys      map[string]string // key -> caller name
	RateLimit    int               // requests per minute per caller
	UpstreamWait time.Duration     // total budget for a backend response
}

type Gateway struct {
	cfg     Config
	log     *slog.Logger
	proxies map[string]*httputil.ReverseProxy
	limiter *limiter
}

func New(cfg Config, log *slog.Logger) (*Gateway, error) {
	g := &Gateway{
		cfg:     cfg,
		log:     log,
		proxies: make(map[string]*httputil.ReverseProxy),
		limiter: newLimiter(cfg.RateLimit),
	}

	for _, r := range cfg.Routes {
		target, err := url.Parse(r.Target)
		if err != nil {
			return nil, err
		}
		p := httputil.NewSingleHostReverseProxy(target)

		// The backend must never see the caller's credentials, and must
		// always see who the gateway decided they are.
		orig := p.Director
		p.Director = func(req *http.Request) {
			orig(req)
			req.Header.Del("X-Api-Key")            // strip: already verified
			req.Header.Del("Authorization")        // strip: already verified
			req.Header.Set("X-Forwarded-By", "king-yemma")
		}

		// A backend failure must not leak its error page to the client.
		p.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			log.Error("upstream failure",
				"target", target.String(), "path", req.URL.Path, "err", err)
			http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		}

		g.proxies[r.Prefix] = p
	}
	return g, nil
}

func (g *Gateway) Handler() http.Handler {
	return g.withRequestID(
		g.withLogging(
			http.HandlerFunc(g.route)))
}

// --- the chain --------------------------------------------------------------

func (g *Gateway) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		// Forward it downstream AND return it to the client, so a user can
		// quote an ID in a bug report and you can find the exact trace.
		r.Header.Set("X-Request-Id", id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		g.log.Info("gateway_request",
			"request_id", r.Header.Get("X-Request-Id"),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"caller", r.Header.Get("X-Caller"),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (g *Gateway) route(w http.ResponseWriter, r *http.Request) {
	route, proxy, ok := g.match(r.URL.Path)
	if !ok {
		writeErr(w, http.StatusNotFound, "no route for "+r.URL.Path)
		return
	}

	if !route.Public {
		caller, ok := g.cfg.APIKeys[r.Header.Get("X-Api-Key")]
		if !ok {
			// 401: we don't know who you are. (403 would mean: we know, and no.)
			writeErr(w, http.StatusUnauthorized, "valid X-Api-Key required")
			return
		}
		r.Header.Set("X-Caller", caller)

		if !g.limiter.allow(caller) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
	}

	if route.Strip {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(route.Prefix, "/"))
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
	}
	proxy.ServeHTTP(w, r)
}

func (g *Gateway) match(path string) (Route, *httputil.ReverseProxy, bool) {
	best := -1
	for i, rt := range g.cfg.Routes {
		if strings.HasPrefix(path, rt.Prefix) &&
			(best == -1 || len(rt.Prefix) > len(g.cfg.Routes[best].Prefix)) {
			best = i // longest prefix wins — same rule as a routing table
		}
	}
	if best == -1 {
		return Route{}, nil, false
	}
	return g.cfg.Routes[best], g.proxies[g.cfg.Routes[best].Prefix], true
}

// --- a fixed-window rate limiter -------------------------------------------
// Deliberately the simple version. Capsule 17 explains why the boundary
// between windows lets a caller burst to 2x the limit, and what to use instead.

type limiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]*window
}

type window struct {
	count int
	start time.Time
}

func newLimiter(limit int) *limiter {
	return &limiter{limit: limit, windows: make(map[string]*window)}
}

func (l *limiter) allow(key string) bool {
	if l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[key]
	if !ok || time.Since(w.start) > time.Minute {
		l.windows[key] = &window{count: 1, start: time.Now()}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// --- helpers ----------------------------------------------------------------

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(c int) { r.status = c; r.ResponseWriter.WriteHeader(c) }

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}