package chat

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestContinuityDefaultWindowAssemblyP95(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	seedChatContinuityThread(t, ctx, sqliteStore, now)
	for i := 0; i < threads.DefaultContinuityMaxPriorTurns; i++ {
		turn := chatContinuityTurn("turn_perf_"+string(rune('a'+i)), "seg_1", int64(i+1), threads.ContinuityRoleUser, "prior", now.Add(time.Duration(i)*time.Second))
		if _, err := sqliteStore.SaveContinuityTurn(ctx, turn); err != nil {
			t.Fatalf("SaveContinuityTurn returned error: %v", err)
		}
	}

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&capturingProvider{name: "continuity-perf"})
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	durations := make([]time.Duration, 0, 40)
	for i := 0; i < 40; i++ {
		started := time.Now()
		if _, err := service.Query(ctx, QueryInput{
			TenantID: "ten_1",
			ThreadID: "thr_1",
			Provider: "continuity-perf",
			Model:    "model-a",
			Query:    "follow up",
		}); err != nil {
			t.Fatalf("Query returned error: %v", err)
		}
		durations = append(durations, time.Since(started))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[int(float64(len(durations))*0.95)-1]
	if p95 > 500*time.Millisecond {
		t.Fatalf("continuity default-window assembly p95=%s exceeds 500ms", p95)
	}
	t.Logf("continuity default-window assembly p95=%s", p95)
}
