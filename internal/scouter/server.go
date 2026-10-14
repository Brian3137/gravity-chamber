package scouter

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
)

type Server struct {
	store Store  // was :_MemStore
	log   *slog.Logger
	ready atomic.Bool
}

func NewServer(store Store, log *slog.Logger) *Server {
    s := &Server{store: store, log: log}
	s.ready.Store(true)
	return s	
}

//func NewServer(store *MemStore, log *slog.Logger) *Server {
//	s := &Server{store: store, log: log}
//	s.ready.Store(true)
//	return s
//}

func (s *Server) NotReady() {
	s.ready.Store(false)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("POST /readings", s.handleRecord)
	mux.HandleFunc("GET /fighters/{name}", s.handleGetFighter)
	mux.HandleFunc("GET /leaderboard", s.handleLeaderboard)
	mux.HandleFunc("GET /train", s.handleTrain)
	

	return s.withLogging(mux)
}

// --- middleware -------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(c int) {
	r.status = c
	r.ResponseWriter.WriteHeader(c)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)

		s.log.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"api_key_seen", r.Header.Get("X-Api-Key"),   // should always be empty
			"caller", r.Header.Get("X-Caller"),          // set by the gateway
			"request_id", r.Header.Get("X-Request-Id"),
		)
	})
}

// --- handlers ---------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
		return
	}
	// Readiness checks dependencies. Liveness never does.
	if p, ok := s.store.(interface{ Ping(context.Context) error }); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := p.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable,
				map[string]string{"status": "database unreachable"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Fighter string `json:"fighter"`
		Power   int64  `json:"power"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if in.Fighter == "" || in.Power < 0 {
		writeErr(w, http.StatusBadRequest, "fighter required and power must be >= 0")
		return
	}

	rd := Reading{
		Fighter:    in.Fighter,
		Power:      in.Power,
		RecordedAt: time.Now().UTC(),
	}

	s.store.Record(rd)

	if in.Power > 9000 {
		s.log.Warn(
			"power level exceeds scouter range",
			"fighter", in.Fighter,
			"power", in.Power,
		)
	}

	writeJSON(w, http.StatusCreated, rd)
}

func (s *Server) handleGetFighter(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	rd, ok := s.store.Latest(name)
	if !ok {
		writeErr(w, http.StatusNotFound, "no readings for "+name)
		return
	}

	writeJSON(w, http.StatusOK, rd)
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := 10

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	writeJSON(w, http.StatusOK, s.store.Leaderboard(limit))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}


// handleTrain burns CPU on purpose: a deliberately serial hash chain that
// no amount of parallelism inside one request can speed up.
//
// CALIBRATE `rounds` ON YOUR MACHINE — do not trust a number from a README.
// ARM chips (Apple Silicon, Graviton) have SHA-256 in hardware and run this
// loop roughly 5x faster than an x86 core without the extension. On an M-series
// Mac 1,000,000 rounds is ~100ms; on an older x86 laptop 200,000 might be.
func (s *Server) handleTrain(w http.ResponseWriter, r *http.Request) {
	rounds := 1_000_000
	if v := r.URL.Query().Get("rounds"); v != "" {
		// NOTE THE SILENT FALLBACK. Ask for more than the cap and you get the
		// DEFAULT, with no error — so a too-large `rounds` looks like a very
		// fast machine rather than a rejected request. Always check
		// `duration_ms` in the response rather than trusting what you asked
		// for. (Capsule 07 leans on this endpoint; if its "slow" requests come
		// back in 20ms, this is why.)
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50_000_000 {
			rounds = n
		}
	}

	start := time.Now()
	sum := sha256.Sum256([]byte("kaio-ken"))
	for i := 0; i < rounds; i++ {
		sum = sha256.Sum256(sum[:]) // each round needs the previous one
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rounds":      rounds,
		"digest":      hex.EncodeToString(sum[:8]),
		"duration_ms": time.Since(start).Milliseconds(),
		"gomaxprocs":  runtime.GOMAXPROCS(0),
		"num_cpu":     runtime.NumCPU(),
	})
}
