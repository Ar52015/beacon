package store

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"sync"
	"time"
)

type Kind string

const (
	KindHTTP    Kind = "http"
	KindTCP     Kind = "tcp"
	KindDNS     Kind = "dns"
	KindTLS     Kind = "tls"
	KindJSONRPC Kind = "jsonrpc"
)

type Target struct {
	ID          string `json:"id,omitempty"`
	URL         string `json:"url"`
	Kind        Kind   `json:"kind"`
	IntervalSec int    `json:"interval_sec"`
}

type Result struct {
	TargetID  string    `json:"target_id,omitempty"`
	Timestamp time.Time `json:"time_stamp"`
	LatencyMs int       `json:"latency_ms"`
	OK        bool      `json:"ok"`
	Error     string    `json:"error,omitempty"`
}

type StatsResponse struct {
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

type Store struct {
	mu        sync.RWMutex
	targets   map[string]Target
	results   map[string][]Result
	latencies map[string][]int
}

func NewStore() *Store {
	return &Store{
		targets:   map[string]Target{},
		results:   map[string][]Result{},
		latencies: map[string][]int{},
	}
}

func (s *Store) AddTarget(t Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[t.ID] = t
}

func (s *Store) GetTarget(id string) (Target, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.targets[id]
	return t, ok
}

func (s *Store) ListTargets() []Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := slices.Collect(maps.Values(s.targets))
	slices.SortFunc(t, func(a, b Target) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return t
}

func (s *Store) DeleteTarget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.targets, id)
	delete(s.results, id)
	delete(s.latencies, id)
}

func (s *Store) AppendResult(r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[r.TargetID] = append(s.results[r.TargetID], r)
	i, _ := slices.BinarySearch(s.latencies[r.TargetID], r.LatencyMs)
	s.latencies[r.TargetID] = slices.Insert(s.latencies[r.TargetID], i, r.LatencyMs)
}

func (s *Store) ListResults(id string) []Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.results[id])
}

func (s *Store) LatencyStats(id string) StatsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// sanity check
	if len(s.latencies[id]) == 0 {
		return StatsResponse{}
	}

	// calculate and return
	return StatsResponse{
		Count: len(s.latencies[id]),
		P50:   percentile(s.latencies[id], 50),
		P90:   percentile(s.latencies[id], 90),
		P95:   percentile(s.latencies[id], 95),
		P99:   percentile(s.latencies[id], 99),
	}
}
