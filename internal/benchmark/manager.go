// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager owns benchmark run lifecycle, live state, and result persistence.
type Manager struct {
	runner    *Runner
	collector *MetricCollector
	store     *ResultStore
	mu        sync.Mutex
	runs      map[string]*managedRun
}

type managedRun struct {
	run    Run
	cancel context.CancelFunc
}

// NewManager creates a benchmark run manager backed by the supplied result store.
func NewManager(store *ResultStore) *Manager {
	return &Manager{runner: NewRunner(), collector: NewMetricCollector(), store: store, runs: make(map[string]*managedRun)}
}

// Start validates and starts a benchmark run asynchronously.
func (m *Manager) Start(ctx context.Context, cfg Config) (Run, error) {
	if cfg.TargetBaseURL == "" {
		return Run{}, fmt.Errorf("BENCH-MANAGER-VALIDATETARGET: targetBaseUrl is required")
	}
	if len(cfg.Workflow) == 0 {
		return Run{}, fmt.Errorf("BENCH-MANAGER-VALIDATEWORKFLOW: workflow must contain at least one step")
	}
	id := fmt.Sprintf("run-%d", time.Now().UnixNano())
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	run := Run{ID: id, State: RunStateRunning, Config: cfg, StartedAt: time.Now(), Result: Result{StatusCodes: map[string]int64{}}}

	m.mu.Lock()
	m.runs[id] = &managedRun{run: run, cancel: cancel}
	m.mu.Unlock()

	go m.execute(runCtx, id, cfg)
	return run, nil
}

func (m *Manager) execute(ctx context.Context, id string, cfg Config) {
	result := m.runner.Execute(ctx, cfg, m.collector, func(result Result) {
		m.updateResult(id, result)
	})
	endedAt := time.Now()
	m.mu.Lock()
	managed := m.runs[id]
	managed.run.Result = result
	managed.run.EndedAt = &endedAt
	if ctx.Err() != nil {
		managed.run.State = RunStateStopped
	} else {
		managed.run.State = RunStateFinished
	}
	run := managed.run
	m.mu.Unlock()
	_ = m.store.Save(run)
}

func (m *Manager) updateResult(id string, result Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if managed, ok := m.runs[id]; ok {
		managed.run.Result = result
	}
}

// Stop requests cancellation for a running benchmark.
func (m *Manager) Stop(id string) (Run, error) {
	m.mu.Lock()
	managed, ok := m.runs[id]
	if !ok {
		m.mu.Unlock()
		return Run{}, fmt.Errorf("BENCH-MANAGER-FINDRUN: run not found")
	}
	managed.cancel()
	run := managed.run
	m.mu.Unlock()
	return run, nil
}

// Get returns a live or persisted benchmark run by ID.
func (m *Manager) Get(id string) (Run, error) {
	m.mu.Lock()
	managed, ok := m.runs[id]
	if ok {
		run := managed.run
		m.mu.Unlock()
		return run, nil
	}
	m.mu.Unlock()
	return m.store.Load(id)
}

// List returns live and persisted benchmark runs.
func (m *Manager) List() ([]Run, error) {
	stored, err := m.store.List()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool, len(stored)+len(m.runs))
	runs := make([]Run, 0, len(stored)+len(m.runs))
	for _, managed := range m.runs {
		runs = append(runs, managed.run)
		seen[managed.run.ID] = true
	}
	for _, run := range stored {
		if !seen[run.ID] {
			runs = append(runs, run)
		}
	}
	return runs, nil
}
