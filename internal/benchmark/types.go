// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import "time"

// RequestTemplate describes one configurable HTTP operation derived from OpenAPI.
type RequestTemplate struct {
	ID          string            `json:"id"`
	OperationID string            `json:"operationId"`
	Summary     string            `json:"summary"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	HasBody     bool              `json:"hasBody"`
	Parameters  []TemplateParam   `json:"parameters"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// TemplateParam describes an OpenAPI operation parameter exposed to the UI.
type TemplateParam struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

// Config is the user-provided configuration for a benchmark run.
type Config struct {
	Name               string            `json:"name"`
	TargetBaseURL      string            `json:"targetBaseUrl"`
	Concurrency        int               `json:"concurrency"`
	DurationSeconds    int               `json:"durationSeconds"`
	RequestCount       int               `json:"requestCount"`
	Seed               string            `json:"seed"`
	Workflow           []WorkflowStep    `json:"workflow"`
	Headers            map[string]string `json:"headers,omitempty"`
	TargetProcessID    int               `json:"targetProcessId,omitempty"`
	PostgresDSN        string            `json:"postgresDsn,omitempty"`
	MetricsIntervalSec int               `json:"metricsIntervalSec,omitempty"`
}

// WorkflowStep is one ordered HTTP request in a benchmark workflow.
type WorkflowStep struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers,omitempty"`
}

// RunState is the lifecycle state of a benchmark run.
type RunState string

// Benchmark run states exposed by the backend API.
const (
	RunStateRunning  RunState = "running"
	RunStateStopped  RunState = "stopped"
	RunStateFinished RunState = "finished"
	RunStateFailed   RunState = "failed"
)

// Run combines configuration, lifecycle metadata, and measured results.
type Run struct {
	ID        string     `json:"id"`
	State     RunState   `json:"state"`
	Config    Config     `json:"config"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Result    Result     `json:"result"`
	Error     string     `json:"error,omitempty"`
}

// Result contains aggregate request, error, and infrastructure metrics.
type Result struct {
	TotalRequests      int64            `json:"totalRequests"`
	SuccessfulRequests int64            `json:"successfulRequests"`
	FailedRequests     int64            `json:"failedRequests"`
	RequestsPerSecond  float64          `json:"requestsPerSecond"`
	ErrorRate          float64          `json:"errorRate"`
	Latency            LatencyStats     `json:"latency"`
	StatusCodes        map[string]int64 `json:"statusCodes"`
	Errors             []ErrorLog       `json:"errors"`
	SystemMetrics      []SystemMetrics  `json:"systemMetrics"`
	PostgresMetrics    []PostgresMetric `json:"postgresMetrics"`
}

// LatencyStats contains millisecond latency distribution values.
type LatencyStats struct {
	MinMs float64 `json:"minMs"`
	AvgMs float64 `json:"avgMs"`
	P50Ms float64 `json:"p50Ms"`
	P95Ms float64 `json:"p95Ms"`
	P99Ms float64 `json:"p99Ms"`
	MaxMs float64 `json:"maxMs"`
}

// ErrorLog records a failed request without stopping the benchmark run.
type ErrorLog struct {
	Time     time.Time `json:"time"`
	WorkerID int       `json:"workerId"`
	Step     string    `json:"step"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	Status   int       `json:"status,omitempty"`
	Message  string    `json:"message"`
}

// SystemMetrics captures local process and disk metrics where the OS exposes them.
type SystemMetrics struct {
	Time              time.Time `json:"time"`
	ProcessID         int       `json:"processId,omitempty"`
	CPUPercent        float64   `json:"cpuPercent"`
	RAMBytes          uint64    `json:"ramBytes"`
	DiskReadBytes     uint64    `json:"diskReadBytes"`
	DiskWrittenBytes  uint64    `json:"diskWrittenBytes"`
	CollectionMessage string    `json:"collectionMessage,omitempty"`
}

// PostgresMetric captures a point-in-time PostgreSQL statistics snapshot.
type PostgresMetric struct {
	Time                   time.Time `json:"time"`
	ActiveConnections      int64     `json:"activeConnections"`
	IdleConnections        int64     `json:"idleConnections"`
	TransactionsCommitted  int64     `json:"transactionsCommitted"`
	TransactionsRolledBack int64     `json:"transactionsRolledBack"`
	TuplesReturned         int64     `json:"tuplesReturned"`
	TuplesFetched          int64     `json:"tuplesFetched"`
	TuplesInserted         int64     `json:"tuplesInserted"`
	TuplesUpdated          int64     `json:"tuplesUpdated"`
	TuplesDeleted          int64     `json:"tuplesDeleted"`
	CollectionMessage      string    `json:"collectionMessage,omitempty"`
}
