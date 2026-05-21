package benchmark

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResultRetrievalPersistsFinishedRun(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	store, err := NewResultStore(t.TempDir())
	require.NoError(t, err)
	manager := NewManager(store)
	run, err := manager.Start(httptest.NewRequest(http.MethodPost, "/", nil).Context(), Config{
		TargetBaseURL: target.URL,
		Concurrency:   1,
		RequestCount:  1,
		Workflow:      []WorkflowStep{{Name: "get", Method: http.MethodGet, Path: "/health"}},
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		loaded, loadErr := manager.Get(run.ID)
		return loadErr == nil && loaded.State == RunStateFinished && loaded.Result.TotalRequests == 1
	}, time.Second, 10*time.Millisecond)

	loaded, err := store.Load(run.ID)
	require.NoError(t, err)
	require.Equal(t, RunStateFinished, loaded.State)
	require.EqualValues(t, 1, loaded.Result.TotalRequests)
}

func TestServerStartStopAndGetRun(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	store, err := NewResultStore(t.TempDir())
	require.NoError(t, err)
	server := httptest.NewServer(NewServer(NewManager(store), filepath.Join(t.TempDir(), "missing.yaml")).Router())
	defer server.Close()

	cfg := Config{TargetBaseURL: target.URL, Concurrency: 1, Workflow: []WorkflowStep{{Name: "get", Method: http.MethodGet, Path: "/health"}}}
	body, err := json.Marshal(cfg)
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/api/runs", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))

	stopReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/runs/"+run.ID+"/stop", nil)
	require.NoError(t, err)
	stopResp, err := http.DefaultClient.Do(stopReq) // #nosec G704 -- test server URL is created by httptest.
	require.NoError(t, err)
	defer func() { _ = stopResp.Body.Close() }()
	require.Equal(t, http.StatusAccepted, stopResp.StatusCode)

	require.Eventually(t, func() bool {
		getResp, getErr := http.Get(server.URL + "/api/runs/" + run.ID)
		if getErr != nil {
			return false
		}
		defer func() { _ = getResp.Body.Close() }()
		var loaded Run
		if err := json.NewDecoder(getResp.Body).Decode(&loaded); err != nil {
			return false
		}
		return loaded.State == RunStateStopped
	}, time.Second, 10*time.Millisecond)
}
