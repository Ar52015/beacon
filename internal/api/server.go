package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/Ar52015/beacon/internal/store"
	"github.com/google/uuid"
)

type Server struct {
	st    *store.Store
	token string
}

type statsResponse struct {
	Count int `json:"count"`
	P50   int `json:"p50_ms"`
	P90   int `json:"p90_ms"`
	P95   int `json:"p95_ms"`
	P99   int `json:"p99_ms"`
}

func percentile(sorted []int, p float64) int {
	N := float64(len(sorted))
	res := math.Ceil((p / 100) * N)
	return sorted[int(res)-1]
}

func NewServer(st *store.Store, token string) *Server {
	return &Server{st: st, token: token}
}

func (s *Server) Routes() http.Handler {
	servemux := http.NewServeMux()

	servemux.HandleFunc("GET /healthz", s.handleHealthz)
	servemux.HandleFunc("GET /slow", s.handleTestSlowFail)
	servemux.HandleFunc("GET /slow200", s.handleTestSlowPass)
	servemux.HandleFunc("POST /targets", s.handleCreateTarget)
	servemux.HandleFunc("GET /targets", s.handleListTargets)
	servemux.HandleFunc("GET /targets/{id}", s.handleGetTarget)
	servemux.HandleFunc("DELETE /targets/{id}", s.handleDeleteTarget)
	servemux.HandleFunc("POST /targets/{id}/results", s.handleAddResult)
	servemux.HandleFunc("GET /targets/{id}/results", s.handleListResults)
	servemux.HandleFunc("GET /targets/{id}/stats", s.handleStats)

	return logging(auth(s.token)(servemux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "err", err)
	}
}

// GET /healthz
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("ok\n")); err != nil {
		slog.Error("write failed", "err", err)
	}
}

// POST /targets
func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var resp store.Target
	err := json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest) // 400
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		http.Error(w, "ID generation failed", http.StatusInternalServerError) // 500
		return
	}
	resp.ID = id.String()
	s.st.AddTarget(resp)
	writeJSON(w, http.StatusOK, resp) // 200
}

// GET /targets
func (s *Server) handleListTargets(w http.ResponseWriter, _ *http.Request) {
	resp := s.st.ListTargets()
	if resp == nil {
		resp = []store.Target{}
	}

	writeJSON(w, http.StatusOK, resp) // 200
}

// GET /target/{id}
func (s *Server) handleGetTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, ok := s.st.GetTarget(id)
	if !ok {
		http.Error(w, "target not found", http.StatusNotFound) // 404
		return
	}

	writeJSON(w, http.StatusOK, resp) // 200
}

// DELETE /targets/{id}
func (s *Server) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := s.st.GetTarget(id)
	if !ok {
		http.Error(w, "target not found", http.StatusNotFound) // 404
		return
	}
	s.st.DeleteTarget(id)
	w.WriteHeader(http.StatusNoContent) // 204
}

// POST /targets/{id}/results
func (s *Server) handleAddResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := s.st.GetTarget(id)
	if !ok {
		http.Error(w, "target not found", http.StatusNotFound) // 404
		return
	}

	var resp store.Result
	err := json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest) // 400
		return
	}

	resp.TargetID = id
	if resp.Timestamp.IsZero() {
		resp.Timestamp = time.Now()
	}

	s.st.AppendResult(resp)
	writeJSON(w, http.StatusCreated, resp) // 201
}

// GET /targets/{id}/results
func (s *Server) handleListResults(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := s.st.GetTarget(id)
	if !ok {
		http.Error(w, "target not found", http.StatusNotFound) // 404
		return
	}

	resp := s.st.ListResults(id)
	writeJSON(w, http.StatusOK, resp) // 200
}

// GET /targets/{id}/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := s.st.GetTarget(id)
	if !ok {
		http.Error(w, "target not found", http.StatusNotFound) // 404
		return
	}

	t := s.st.ListResults(id)

	if len(t) == 0 {
		writeJSON(w, http.StatusOK, statsResponse{}) // 200
		return
	}

	sorted := make([]int, 0, len(t))
	for _, i := range t {
		sorted = append(sorted, i.LatencyMs)
	}
	slices.Sort(sorted)
	resp := statsResponse{Count: len(t)}
	resp.P50 = percentile(sorted, 50)
	resp.P90 = percentile(sorted, 90)
	resp.P95 = percentile(sorted, 95)
	resp.P99 = percentile(sorted, 99)

	writeJSON(w, http.StatusOK, resp) // 200
}

// GET /slow <TEST ENDPOINT CASE: FAIL>
func (s *Server) handleTestSlowFail(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(30 * time.Second)
	if _, err := w.Write([]byte("Slow endpoint test complete!")); err != nil {
		slog.Error("write failed", "err", err)
	}
}

// GET /slow200	<TEST ENDPOINT CASE: SUCCESS>
func (s *Server) handleTestSlowPass(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(5 * time.Second)
	resp := "Slow endpoint test complete!"
	writeJSON(w, http.StatusOK, resp) // 200
}
