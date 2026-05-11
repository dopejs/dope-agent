package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestContinuityDefaultWindowLookupP95(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_perf", "thr_perf", "seg_perf", now)
	for i := 0; i < threads.DefaultContinuityMaxPriorTurns; i++ {
		turn := continuityStoreTestTurn("turn_perf_"+string(rune('a'+i)), "ten_perf", "thr_perf", "seg_perf", 0, now.Add(time.Duration(i)*time.Second))
		if _, err := store.SaveContinuityTurn(ctx, turn); err != nil {
			t.Fatalf("SaveContinuityTurn: %v", err)
		}
	}

	durations := make([]time.Duration, 0, 50)
	for i := 0; i < 50; i++ {
		started := time.Now()
		turns, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_perf", ThreadID: "thr_perf", SessionSegmentID: "seg_perf", Now: now})
		if err != nil {
			t.Fatalf("ListContinuityTurns: %v", err)
		}
		if len(turns) != threads.DefaultContinuityMaxPriorTurns {
			t.Fatalf("turns=%d want %d", len(turns), threads.DefaultContinuityMaxPriorTurns)
		}
		durations = append(durations, time.Since(started))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[int(float64(len(durations))*0.95)-1]
	if p95 > 25*time.Millisecond {
		t.Fatalf("continuity default-window lookup p95=%s exceeds 25ms", p95)
	}
	t.Logf("continuity default-window lookup p95=%s", p95)
}
