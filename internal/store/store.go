package store

import (
	"cmp"
	"maps"
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

type Store struct {
	mu      sync.RWMutex
	targets map[string]Target
	results map[string][]Result
}

func NewStore() *Store {
	return &Store{
		targets: map[string]Target{},
		results: map[string][]Result{},
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
}

func (s *Store) AppendResult(r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[r.TargetID] = append(s.results[r.TargetID], r)
}

func (s *Store) ListResults(id string) []Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.results[id])
}
