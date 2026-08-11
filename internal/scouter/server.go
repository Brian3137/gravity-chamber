package scouter

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Server struct {
	store *MemStore
	log   *slog.Logger
	ready atomic.Bool
}

func NewServer(store *MemStore, log *slog.Logger) *Server {
	s := &Server{store: store, log: log}
	s.ready.Store(true)
	return s
}

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
		)
	})
}

// --- handlers ---------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
		return
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
