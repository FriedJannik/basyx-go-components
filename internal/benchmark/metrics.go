// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/lib/pq"
)

// MetricCollector samples local process and PostgreSQL metrics for benchmark results.
type MetricCollector struct{}

// NewMetricCollector creates the default benchmark metrics collector.
func NewMetricCollector() *MetricCollector {
	return &MetricCollector{}
}

// Collect appends system and PostgreSQL metrics to the run accumulator.
func (c *MetricCollector) Collect(ctx context.Context, cfg Config, acc *runAccumulator) {
	acc.addSystemMetric(collectSystemMetrics(cfg.TargetProcessID))
	if cfg.PostgresDSN != "" {
		acc.addPostgresMetric(collectPostgresMetrics(ctx, cfg.PostgresDSN))
	}
}

func collectSystemMetrics(pid int) SystemMetrics {
	metric := SystemMetrics{Time: time.Now(), ProcessID: pid}
	if pid > 0 {
		procMetric, err := collectProcMetrics(pid)
		if err == nil {
			return procMetric
		}
		metric.CollectionMessage = err.Error()
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	metric.ProcessID = os.Getpid()
	metric.RAMBytes = mem.Sys
	if metric.CollectionMessage == "" {
		metric.CollectionMessage = "BENCH-METRICS-RUNTIMEFALLBACK: process metrics use benchmark service runtime stats on this OS"
	}
	return metric
}

func collectProcMetrics(pid int) (SystemMetrics, error) {
	metric := SystemMetrics{Time: time.Now(), ProcessID: pid}
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	// #nosec G304 -- the PID path is constructed from a numeric developer-provided PID.
	statusFile, err := os.Open(statusPath)
	if err != nil {
		return metric, fmt.Errorf("BENCH-METRICS-READPROCSTATUS: %w", err)
	}
	defer func() { _ = statusFile.Close() }()
	scanner := bufio.NewScanner(statusFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				metric.RAMBytes = kb * 1024
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return metric, fmt.Errorf("BENCH-METRICS-SCANPROCSTATUS: %w", err)
	}
	readBytes, writtenBytes, err := readProcIO(pid)
	if err == nil {
		metric.DiskReadBytes = readBytes
		metric.DiskWrittenBytes = writtenBytes
	}
	return metric, nil
}

func readProcIO(pid int) (uint64, uint64, error) {
	ioPath := fmt.Sprintf("/proc/%d/io", pid)
	// #nosec G304 -- the PID path is constructed from a numeric developer-provided PID.
	ioFile, err := os.Open(ioPath)
	if err != nil {
		return 0, 0, fmt.Errorf("BENCH-METRICS-READPROCIO: %w", err)
	}
	defer func() { _ = ioFile.Close() }()
	var readBytes uint64
	var writtenBytes uint64
	scanner := bufio.NewScanner(ioFile)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "read_bytes":
			readBytes = value
		case "write_bytes":
			writtenBytes = value
		}
	}
	return readBytes, writtenBytes, scanner.Err()
}

func collectPostgresMetrics(ctx context.Context, dsn string) PostgresMetric {
	metric := PostgresMetric{Time: time.Now()}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		metric.CollectionMessage = fmt.Sprintf("BENCH-PGMETRICS-OPENDB: %v", err)
		return metric
	}
	defer func() { _ = db.Close() }()
	query, _, err := goqu.From(goqu.T("pg_stat_database")).
		Select(
			goqu.SUM(goqu.C("xact_commit")).As("commits"),
			goqu.SUM(goqu.C("xact_rollback")).As("rollbacks"),
			goqu.SUM(goqu.C("tup_returned")).As("returned"),
			goqu.SUM(goqu.C("tup_fetched")).As("fetched"),
			goqu.SUM(goqu.C("tup_inserted")).As("inserted"),
			goqu.SUM(goqu.C("tup_updated")).As("updated"),
			goqu.SUM(goqu.C("tup_deleted")).As("deleted"),
		).ToSQL()
	if err != nil {
		metric.CollectionMessage = fmt.Sprintf("BENCH-PGMETRICS-BUILDQUERY: %v", err)
		return metric
	}
	row := db.QueryRowContext(ctx, query)
	if err := row.Scan(&metric.TransactionsCommitted, &metric.TransactionsRolledBack, &metric.TuplesReturned, &metric.TuplesFetched, &metric.TuplesInserted, &metric.TuplesUpdated, &metric.TuplesDeleted); err != nil {
		metric.CollectionMessage = fmt.Sprintf("BENCH-PGMETRICS-EXECSTATDB: %v", err)
		return metric
	}

	connQuery, _, err := goqu.From(goqu.T("pg_stat_activity")).
		Select(
			goqu.SUM(goqu.Case().When(goqu.C("state").Eq("active"), 1).Else(0)).As("active"),
			goqu.SUM(goqu.Case().When(goqu.C("state").Eq("idle"), 1).Else(0)).As("idle"),
		).ToSQL()
	if err != nil {
		metric.CollectionMessage = fmt.Sprintf("BENCH-PGMETRICS-BUILDCONNQUERY: %v", err)
		return metric
	}
	if err := db.QueryRowContext(ctx, connQuery).Scan(&metric.ActiveConnections, &metric.IdleConnections); err != nil {
		metric.CollectionMessage = fmt.Sprintf("BENCH-PGMETRICS-EXECACTIVITY: %v", err)
	}
	return metric
}
