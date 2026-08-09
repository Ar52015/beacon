package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Ar52015/beacon/internal/store"
)

func TestHealthz(t *testing.T) {
	// SETUP
	st := store.NewStore()
	srv := NewServer(st, "testtoken123")

	// SERVE
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	// TEST-CASES
	// Status Code
	if rec.Code != 200 {
		t.Errorf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 200, rec.Code)
	}
	// Response Body
	if rec.Body.String() != "ok\n" {
		t.Errorf("Wrong response body recieved\nexpected: %s\nrecieved: %s", `ok\n`, rec.Body.String())
	}
}

func TestAuth(t *testing.T) {
	// CASES
	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{"1. /healthz bypasses Auth", "/healthz", "", 200},
		{"2. No token rejects", "/targets", "", 401},
		{"3. Wrong token rejects", "/targets", "wrong", 401},
		{"4. Correct token accepts", "/targets", "testtoken123", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// SETUP
			st := store.NewStore()
			srv := NewServer(st, "testtoken123")

			// SERVE
			req := httptest.NewRequest("GET", tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			srv.Routes().ServeHTTP(rec, req)

			// TEST
			if rec.Code != tc.wantStatus {
				t.Errorf("Wrong status code recieved\nexpected: %d\nrecieved: %d", tc.wantStatus, rec.Code)
			}
		})
	}
}

func setupRequest(method, path, body string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+"testtoken123")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return httptest.NewRecorder(), req
}

func TestTargetCRUD(t *testing.T) {
	// SETUP
	st := store.NewStore()
	srv := NewServer(st, "testtoken123")

	// SERVE: CREATE - POST /targets
	body := `{"url":"https://a.com","kind":"http","interval_sec":30}`
	rec, req := setupRequest("POST", "/targets", body)
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 201 {
		t.Fatalf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 201, rec.Code)
	}
	// Recieved Body
	var created store.Target
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("Error during decoding request body: %v", err)
	}
	// Recieved ID
	if created.ID == "" {
		t.Fatalf("Empty ID recieved")
	}

	// SERVE: READ - GET /targets/{id}
	id := created.ID
	rec, req = setupRequest("GET", "/targets/"+id, "")
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 200 {
		t.Fatalf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 200, rec.Code)
	}
	// Recieved Body
	var recieved store.Target
	if err := json.NewDecoder(rec.Body).Decode(&recieved); err != nil {
		t.Fatalf("Error during decoding request body: %v", err)
	} else if recieved != created {
		t.Fatalf("Recieved body doesn't match expected body\nRecieved: %v\nExpected: %v", recieved, created)
	}

	// SERVE: DELETE - DELETE /targets/{id}
	rec, req = setupRequest("DELETE", "/targets/"+id, "")
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 204 {
		t.Errorf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 204, rec.Code)
	}
}

func TestTargetResultsAndStats(t *testing.T) {
	// SETUP
	st := store.NewStore()
	srv := NewServer(st, "testtoken123")

	// SERVE: CREATE - POST /targets
	body := `{"url":"https://a.com","kind":"http","interval_sec":30}`
	rec, req := setupRequest("POST", "/targets", body)
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 201 {
		t.Fatalf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 201, rec.Code)
	}
	// Recieved Body
	var created store.Target
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("Error during decoding request body: %v", err)
	}
	// Recieved ID
	if created.ID == "" {
		t.Fatalf("Empty ID recieved")
	}

	// SEED DATA: CREATE - POST /targets/{id}/results
	id := created.ID
	latencies := []int{30, 3, 200, 42, 500, 11, 90, 25, 150, 7, 60, 5, 120, 35, 9, 75, 21, 14, 50, 18}
	for _, ms := range latencies {
		body := fmt.Sprintf(`{"latency_ms": %d, "ok": true}`, ms)
		rec, req := setupRequest("POST", "/targets/"+id+"/results", body)
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != 201 {
			t.Fatalf("seed result %d:\nwant: %d\ngot: %d", ms, 201, rec.Code)
		}
	}

	// READ DATA: READ - GET /targets/{id}/results
	rec, req = setupRequest("GET", "/targets/"+id+"/results", "")
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 200 {
		t.Fatalf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 200, rec.Code)
	}
	// Recieved Body
	var recieved []store.Result
	if err := json.NewDecoder(rec.Body).Decode(&recieved); err != nil {
		t.Fatalf("Error during decoding request body: %v", err)
	}
	// Recieved len
	if len(recieved) == 0 {
		t.Fatalf("Results list came up empty\nexpected: %d", len(latencies))
	}

	// READ RESULTS: READ - GET /targets/{id}/stats
	rec, req = setupRequest("GET", "/targets/"+id+"/stats", "")
	srv.Routes().ServeHTTP(rec, req)

	// TEST
	// Status Code
	if rec.Code != 200 {
		t.Fatalf("Wrong status code recieved\nexpected: %d\nrecieved: %d", 200, rec.Code)
	}
	// Recieved Body
	var statsRecieved store.StatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&statsRecieved); err != nil {
		t.Fatalf("Error during decoding request body: %v", err)
	}
	// Calculate & Test stats
	slices.Sort(latencies)
	N := float64(len(latencies))
	if statsRecieved.P50 != latencies[int(math.Ceil((50.0/100)*N))-1] ||
		statsRecieved.P90 != latencies[int(math.Ceil((90.0/100)*N))-1] ||
		statsRecieved.P95 != latencies[int(math.Ceil((95.0/100)*N))-1] ||
		statsRecieved.P99 != latencies[int(math.Ceil((99.0/100)*N))-1] {
		t.Fatalf("Stats mismatched")
	}
}

func TestIntegration(t *testing.T) {
	// SETUP
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	st := store.NewStore()
	srv := NewServer(st, "testtoken123")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	// POST
	body := `{"url":"https://a.com","kind":"http","interval_sec":30}`
	req, err := http.NewRequest("POST", ts.URL+"/targets", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer testtoken123")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("Failed to recieve POST output: %v", err)
	}

	// TEST
	// Status Code
	if resp.StatusCode != 201 {
		t.Fatalf("Status Code Mismatch for POST")
	}
	// Decode
	var created store.Target
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Response decoding failed: %v", err)
	}
	id := created.ID
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// GET
	req, err = http.NewRequest("GET", ts.URL+"/targets/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer testtoken123")

	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("Failed to recieve GET output: %v", err)
	}

	// TEST
	// Status Code
	if resp.StatusCode != 200 {
		t.Fatalf("Status Code Mismatch for GET")
	}
	// Decode
	var recieved store.Target
	if err := json.NewDecoder(resp.Body).Decode(&recieved); err != nil {
		t.Fatalf("Response Decoding failed")
	}
	// Compare
	if recieved != created {
		t.Fatalf("Output mismatch, expected: %v, got: %v", created, recieved)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// DELETE
	req, err = http.NewRequest("DELETE", ts.URL+"/targets/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer testtoken123")

	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("Failed to recive DELETE output: %v", err)
	}

	// TEST
	// Status Code
	if resp.StatusCode != 204 {
		t.Fatalf("Status Code Mismatch for DELETE")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()
}
