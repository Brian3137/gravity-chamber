package scouter

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() http.Handler {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewServer(NewMemStore(), log).Routes()
}

func TestRecordThenRead(t *testing.T) {
	h := newTestServer()

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"fighter":"goku","power":9001}`)
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/readings", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("record: got %d want 201: %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/fighters/goku", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("read: got %d want 200", rec.Code)
	}
	var got Reading
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Power != 9001 {
		t.Errorf("power = %d, want 9001", got.Power)
	}
}

func TestRejectsBadInput(t *testing.T) {
	h := newTestServer()

	for name, body := range map[string]string{
		"empty fighter": `{"fighter":"","power":100}`,
		"negative":      `{"fighter":"krillin","power":-1}`,
		"not json":      `hello`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("POST", "/readings", strings.NewReader(body)))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400", rec.Code)
			}
		})
	}
}

func TestLivenessIgnoresDependencies(t *testing.T) {
	// Liveness must answer from memory. If this ever needs a database
	// running to pass, the check is wrong.
	rec := httptest.NewRecorder()

	newTestServer().ServeHTTP(
		rec,
		httptest.NewRequest("GET", "/healthz", nil),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}
