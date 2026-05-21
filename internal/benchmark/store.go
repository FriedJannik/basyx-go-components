// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// ResultStore persists benchmark runs as JSON files.
type ResultStore struct {
	dir string
	mu  sync.Mutex
}

// NewResultStore creates a filesystem-backed benchmark result store.
func NewResultStore(dir string) (*ResultStore, error) {
	if dir == "" {
		dir = "benchmark-results"
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("BENCH-STORE-MKDIR: %w", err)
	}
	return &ResultStore{dir: dir}, nil
}

// Save writes a benchmark run result to disk.
func (s *ResultStore) Save(run Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("BENCH-STORE-MARSHALRUN: %w", err)
	}
	path := filepath.Join(s.dir, run.ID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("BENCH-STORE-WRITERUN: %w", err)
	}
	return nil
}

// Load reads a persisted benchmark run by ID.
func (s *ResultStore) Load(id string) (Run, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path) //nolint:gosec // run IDs are resolved within the configured result directory.
	if err != nil {
		return Run{}, fmt.Errorf("BENCH-STORE-READRUN: %w", err)
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return Run{}, fmt.Errorf("BENCH-STORE-PARSERUN: %w", err)
	}
	return run, nil
}

// List reads all persisted benchmark runs.
func (s *ResultStore) List() ([]Run, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("BENCH-STORE-LISTDIR: %w", err)
	}
	runs := make([]Run, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		// #nosec G304 -- file names come from the configured benchmark result directory.
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("BENCH-STORE-READLISTITEM: %w", err)
		}
		var run Run
		if err := json.Unmarshal(data, &run); err != nil {
			return nil, fmt.Errorf("BENCH-STORE-PARSELISTITEM: %w", err)
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.After(runs[j].StartedAt) })
	return runs, nil
}
