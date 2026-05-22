// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Runner executes ordered HTTP workflows against a configured target service.
type Runner struct {
	client *http.Client
}

type runAccumulator struct {
	mu            sync.Mutex
	latencies     []float64
	statusCodes   map[string]int64
	errors        []ErrorLog
	systemMetrics []SystemMetrics
	pgMetrics     []PostgresMetric
	total         atomic.Int64
	success       atomic.Int64
	failed        atomic.Int64
	iterations    atomic.Int64
}

var encodedIDAliases = []string{
	"id",
	"submodelIdentifier",
	"aasIdentifier",
	"assetAdministrationShellIdentifier",
	"conceptDescriptionIdentifier",
}

// NewRunner creates a benchmark runner with a bounded HTTP client timeout.
func NewRunner() *Runner {
	return &Runner{client: &http.Client{Timeout: 30 * time.Second}}
}

// Execute runs the configured workflow until stopped, timed out, or request-limited.
func (r *Runner) Execute(ctx context.Context, cfg Config, metricCollector *MetricCollector, onUpdate func(Result)) Result {
	cfg = normalizeConfig(cfg)
	acc := &runAccumulator{statusCodes: make(map[string]int64)}
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for workerID := 0; workerID < cfg.Concurrency; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.executeWorker(ctx, cfg, id, acc, started)
		}(workerID)
	}

	metricsDone := make(chan struct{})
	go collectMetrics(ctx, cfg, metricCollector, acc, onUpdate, metricsDone)

	wg.Wait()
	cancel()
	<-metricsDone
	return acc.snapshot(time.Since(started))
}

func normalizeConfig(cfg Config) Config {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.MetricsIntervalSec < 1 {
		cfg.MetricsIntervalSec = 2
	}
	return cfg
}

func (r *Runner) executeWorker(ctx context.Context, cfg Config, workerID int, acc *runAccumulator, started time.Time) {
	values := map[string]string{
		"worker": strconv.Itoa(workerID),
		"seed":   cfg.Seed,
	}
	for {
		iteration, ok := acc.nextIteration(ctx, cfg, started)
		if !ok {
			return
		}
		values["request"] = strconv.FormatInt(iteration, 10)
		for _, step := range cfg.Workflow {
			if shouldStop(ctx, cfg, started) {
				return
			}
			r.executeStep(ctx, cfg, workerID, step, values, acc)
		}
	}
}

func (acc *runAccumulator) nextIteration(ctx context.Context, cfg Config, started time.Time) (int64, bool) {
	if shouldStop(ctx, cfg, started) {
		return 0, false
	}
	if cfg.RequestCount <= 0 {
		return acc.iterations.Add(1), true
	}
	for {
		current := acc.iterations.Load()
		if current >= int64(cfg.RequestCount) {
			return 0, false
		}
		if acc.iterations.CompareAndSwap(current, current+1) {
			return current + 1, true
		}
	}
}

func shouldStop(ctx context.Context, cfg Config, started time.Time) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	if cfg.DurationSeconds > 0 && time.Since(started) >= time.Duration(cfg.DurationSeconds)*time.Second {
		return true
	}
	return false
}

func (r *Runner) executeStep(ctx context.Context, cfg Config, workerID int, step WorkflowStep, values map[string]string, acc *runAccumulator) {
	path := replacePlaceholders(step.Path, values)
	body := replacePlaceholders(step.Body, values)
	if strings.EqualFold(step.Method, http.MethodPost) && !strings.Contains(step.Body, "{request}") {
		body = appendRequestIndexToJSONID(body, values["request"])
	}
	target, err := joinURL(cfg.TargetBaseURL, path)
	if err != nil {
		acc.recordError(workerID, step, 0, err.Error(), 0)
		return
	}

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, step.Method, target, reader)
	if err != nil {
		acc.recordError(workerID, step, 0, err.Error(), 0)
		return
	}
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	for key, value := range step.Headers {
		req.Header.Set(key, value)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	started := time.Now()
	// #nosec G704 -- the benchmark target URL is explicitly supplied by trusted developers.
	resp, err := r.client.Do(req)
	latencyMs := float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		acc.recordError(workerID, step, 0, err.Error(), latencyMs)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	statusOK := resp.StatusCode >= 200 && resp.StatusCode < 400
	if readErr != nil {
		acc.recordError(workerID, step, resp.StatusCode, readErr.Error(), latencyMs)
		return
	}
	if strings.EqualFold(step.Method, http.MethodPost) && statusOK {
		storeEncodedID(responseBody, values)
	}
	if !statusOK {
		acc.recordError(workerID, step, resp.StatusCode, string(responseBody), latencyMs)
		return
	}
	acc.recordSuccess(resp.StatusCode, latencyMs)
}

func replacePlaceholders(value string, values map[string]string) string {
	for key, replacement := range values {
		value = strings.ReplaceAll(value, "{"+key+"}", replacement)
	}
	return value
}

func appendRequestIndexToJSONID(body string, requestIndex string) string {
	if body == "" || requestIndex == "" {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return body
	}
	id, ok := payload["id"].(string)
	if !ok || id == "" || strings.HasSuffix(id, "-"+requestIndex) {
		return body
	}
	payload["id"] = id + "-" + requestIndex
	updatedBody, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return string(updatedBody)
}

func joinURL(baseURL string, path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("BENCH-RUNNER-PARSETARGET: invalid target base URL")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	joined, err := url.Parse(base.String() + path)
	if err != nil {
		return "", fmt.Errorf("BENCH-RUNNER-PARSEPATH: %w", err)
	}
	return joined.String(), nil
}

func storeEncodedID(responseBody []byte, values map[string]string) {
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return
	}
	id, ok := payload["id"].(string)
	if !ok || id == "" {
		return
	}
	encodedID := base64.RawURLEncoding.EncodeToString([]byte(id))
	for _, alias := range encodedIDAliases {
		values[alias] = encodedID
	}
}

func (acc *runAccumulator) recordSuccess(statusCode int, latencyMs float64) {
	acc.total.Add(1)
	acc.success.Add(1)
	acc.mu.Lock()
	defer acc.mu.Unlock()
	acc.latencies = append(acc.latencies, latencyMs)
	acc.statusCodes[strconv.Itoa(statusCode)]++
}

func (acc *runAccumulator) recordError(workerID int, step WorkflowStep, statusCode int, message string, latencyMs float64) {
	acc.total.Add(1)
	acc.failed.Add(1)
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if latencyMs > 0 {
		acc.latencies = append(acc.latencies, latencyMs)
	}
	if statusCode > 0 {
		acc.statusCodes[strconv.Itoa(statusCode)]++
	}
	if len(acc.errors) < 1000 {
		acc.errors = append(acc.errors, ErrorLog{Time: time.Now(), WorkerID: workerID, Step: step.Name, Method: step.Method, Path: step.Path, Status: statusCode, Message: strings.TrimSpace(message)})
	}
}

func (acc *runAccumulator) addSystemMetric(metric SystemMetrics) {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	acc.systemMetrics = append(acc.systemMetrics, metric)
}

func (acc *runAccumulator) addPostgresMetric(metric PostgresMetric) {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	acc.pgMetrics = append(acc.pgMetrics, metric)
}

func (acc *runAccumulator) snapshot(elapsed time.Duration) Result {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	latencies := append([]float64(nil), acc.latencies...)
	statusCodes := make(map[string]int64, len(acc.statusCodes))
	for code, count := range acc.statusCodes {
		statusCodes[code] = count
	}
	total := acc.total.Load()
	failed := acc.failed.Load()
	result := Result{
		TotalRequests:      total,
		SuccessfulRequests: acc.success.Load(),
		FailedRequests:     failed,
		StatusCodes:        statusCodes,
		Errors:             append([]ErrorLog(nil), acc.errors...),
		SystemMetrics:      append([]SystemMetrics(nil), acc.systemMetrics...),
		PostgresMetrics:    append([]PostgresMetric(nil), acc.pgMetrics...),
		Latency:            calculateLatency(latencies),
	}
	if elapsed.Seconds() > 0 {
		result.RequestsPerSecond = float64(total) / elapsed.Seconds()
	}
	if total > 0 {
		result.ErrorRate = float64(failed) / float64(total)
	}
	return result
}

func calculateLatency(values []float64) LatencyStats {
	if len(values) == 0 {
		return LatencyStats{}
	}
	sort.Float64s(values)
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return LatencyStats{
		MinMs: values[0],
		AvgMs: sum / float64(len(values)),
		P50Ms: percentile(values, 0.50),
		P95Ms: percentile(values, 0.95),
		P99Ms: percentile(values, 0.99),
		MaxMs: values[len(values)-1],
	}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}

func collectMetrics(ctx context.Context, cfg Config, collector *MetricCollector, acc *runAccumulator, onUpdate func(Result), done chan<- struct{}) {
	defer close(done)
	if collector == nil {
		return
	}
	ticker := time.NewTicker(time.Duration(cfg.MetricsIntervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			metricCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			collector.Collect(metricCtx, cfg, acc)
			cancel()
			return
		case <-ticker.C:
			collector.Collect(ctx, cfg, acc)
			if onUpdate != nil {
				onUpdate(acc.snapshot(time.Second))
			}
		}
	}
}
