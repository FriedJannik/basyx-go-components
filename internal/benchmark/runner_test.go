package benchmark

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunnerExecutesWorkflowWithEncodedIDSubstitution(t *testing.T) {
	var mu sync.Mutex
	seen := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "submodel-1"})
		case http.MethodGet:
			require.Equal(t, "/submodels/"+base64.RawURLEncoding.EncodeToString([]byte("submodel-1")), r.URL.Path)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 1, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"submodel-1"}`},
		{Name: "read", Method: http.MethodGet, Path: "/submodels/{id}"},
		{Name: "delete", Method: http.MethodDelete, Path: "/submodels/{id}"},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 3, result.TotalRequests)
	require.EqualValues(t, 3, result.SuccessfulRequests)
	require.Zero(t, result.FailedRequests)
	require.Contains(t, result.StatusCodes, "200")
	mu.Lock()
	require.Len(t, seen, 3)
	mu.Unlock()
}

func TestRunnerSubstitutesOpenAPIIdentifierAliasWithEncodedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "benchmark-0-dev-seed"})
		case http.MethodGet:
			require.Equal(t, "/submodels/"+base64.RawURLEncoding.EncodeToString([]byte("benchmark-0-dev-seed")), r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 1, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"benchmark-{worker}-{seed}"}`},
		{Name: "read", Method: http.MethodGet, Path: "/submodels/{submodelIdentifier}"},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 2, result.TotalRequests)
	require.EqualValues(t, 2, result.SuccessfulRequests)
	require.Zero(t, result.FailedRequests)
}

func TestRunnerAppendsRequestIndexToPostBodyIDWhenMissing(t *testing.T) {
	var mu sync.Mutex
	ids := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		mu.Lock()
		ids = append(ids, body["id"])
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"id": body["id"]})
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 2, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"resource"}`},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 2, result.SuccessfulRequests)
	require.ElementsMatch(t, []string{"resource-1", "resource-2"}, ids)
}

func TestRunnerDoesNotDoubleAppendRequestIndexWhenPlaceholderIsUsed(t *testing.T) {
	var id string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		id = body["id"]
		_ = json.NewEncoder(w).Encode(map[string]string{"id": body["id"]})
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 1, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"resource-{request}"}`},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 1, result.SuccessfulRequests)
	require.Equal(t, "resource-1", id)
}

func TestRunnerContinuesAfterErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 2, Workflow: []WorkflowStep{{Name: "get", Method: http.MethodGet, Path: "/health"}}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 2, result.TotalRequests)
	require.EqualValues(t, 1, result.SuccessfulRequests)
	require.EqualValues(t, 1, result.FailedRequests)
	require.Len(t, result.Errors, 1)
}

func TestRunnerKeepsWorkerWorkflowContextIsolated(t *testing.T) {
	var mu sync.Mutex
	createdIDs := map[string]bool{}
	paths := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			mu.Lock()
			createdIDs[body["id"]] = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]string{"id": body["id"]})
		case http.MethodGet:
			mu.Lock()
			paths[r.URL.Path]++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 2, RequestCount: 2, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"resource-{worker}-{request}"}`},
		{Name: "read", Method: http.MethodGet, Path: "/submodels/{id}"},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.GreaterOrEqual(t, result.TotalRequests, int64(4))
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, createdIDs, 2)
	for createdID := range createdIDs {
		require.Contains(t, paths, "/submodels/"+base64.RawURLEncoding.EncodeToString([]byte(createdID)))
	}
}

func TestRequestCountCountsFullWorkflowIterations(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "resource"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, RequestCount: 2, Workflow: []WorkflowStep{
		{Name: "create", Method: http.MethodPost, Path: "/submodels", Body: `{"id":"resource-{request}"}`},
		{Name: "read", Method: http.MethodGet, Path: "/submodels/{id}"},
		{Name: "delete", Method: http.MethodDelete, Path: "/submodels/{id}"},
	}}
	result := NewRunner().Execute(context.TODO(), cfg, nil, nil)
	require.EqualValues(t, 6, result.TotalRequests)
	require.Equal(t, 6, requests)
}

func TestRunnerStopBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.TODO())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	cfg := Config{TargetBaseURL: server.URL, Concurrency: 1, Workflow: []WorkflowStep{{Name: "get", Method: http.MethodGet, Path: "/health"}}}
	result := NewRunner().Execute(ctx, cfg, nil, nil)
	require.NotZero(t, result.TotalRequests)
	require.Less(t, result.TotalRequests, int64(10))
}
