package store

import (
	"context"
	"database/sql"
	"sort"
	"testing"
	"time"
)

// Roadmap 35 (US3 / T087-T089) — perf p95 regression helpers.
//
// Pre/post comparison shape:
//
//   pre  → open at v21 (no tenant_id), bulk-insert N rows, measure p95 of
//          the legacy list query.
//   post → open at head (tenant_id present + tenant-aware index), bulk-
//          insert N rows split across two tenants, measure p95 of the
//          tenant-filtered list query.
//
// Assertion: p95(post) ≤ 1.2 × p95(pre). The ceiling guards against
// migration-induced regression; in practice the post path is faster
// because the new tenant index is more selective.
//
// Tests are skipped under `go test -short` so local iteration stays fast;
// CI runs without -short and exercises the full N=10000 seed.

const (
	perfRowCount   = 10_000
	perfIterations = 200
	perfRegressionFactor = 1.2

	perfTenantA = "ten_perfaaaa"
	perfTenantB = "ten_perfbbbb"
)

// percentile returns the value at the requested percentile (0..1) of
// durations. Caller passes a slice that does not need to be pre-sorted;
// the helper sorts in place.
func percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	idx := int(float64(len(durations))*p) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(durations) {
		idx = len(durations) - 1
	}
	return durations[idx]
}

// measureP95 runs query `iterations` times, draining each result set
// fully to ensure the observed latency reflects the full read path.
func measureP95(t *testing.T, db *sql.DB, iterations int, query string, args ...any) time.Duration {
	t.Helper()
	ctx := context.Background()
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		for rows.Next() {
			_ = rows.Scan(new(any))
		}
		_ = rows.Close()
		durations = append(durations, time.Since(start))
	}
	return percentile(durations, 0.95)
}

// assertNoRegression compares post ≤ factor × pre and fails the test
// with a descriptive message otherwise.
func assertNoRegression(t *testing.T, label string, pre, post time.Duration) {
	t.Helper()
	if pre <= 0 {
		t.Fatalf("%s: pre p95 is zero (no measurement)", label)
	}
	limit := time.Duration(float64(pre) * perfRegressionFactor)
	if post > limit {
		t.Errorf("%s: post p95 %s exceeds 1.2× pre p95 %s (limit %s)", label, post, pre, limit)
	} else {
		t.Logf("%s: pre=%s post=%s (post/pre=%.2f, limit=%.2f)", label, pre, post, float64(post)/float64(pre), perfRegressionFactor)
	}
}
