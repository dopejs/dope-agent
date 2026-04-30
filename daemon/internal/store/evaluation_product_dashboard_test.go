package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductDashboardStorePersistsAndPagesTenantProjections(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	for _, item := range []evaluation.DashboardProjection{
		{ProjectionID: "projection_001", TenantID: "ten_dashboard", WindowStart: now.Add(-time.Hour), WindowEnd: now, CampaignStatusCounts: map[string]int{"completed": 1}, GeneratedAt: now.Add(-time.Minute)},
		{ProjectionID: "projection_002", TenantID: "ten_dashboard", WindowStart: now.Add(-time.Hour), WindowEnd: now, CampaignStatusCounts: map[string]int{"failed": 1}, GeneratedAt: now},
		{ProjectionID: "projection_other", TenantID: "ten_other", WindowStart: now.Add(-time.Hour), WindowEnd: now, CampaignStatusCounts: map[string]int{"completed": 99}, GeneratedAt: now},
	} {
		if err := s.SaveDashboardProjection(ctx, item); err != nil {
			t.Fatalf("SaveDashboardProjection(%s): %v", item.ProjectionID, err)
		}
	}

	first, err := s.ListDashboardProjections(ctx, evaluation.ProductListFilter{TenantID: "ten_dashboard", Limit: 1})
	if err != nil {
		t.Fatalf("ListDashboardProjections: %v", err)
	}
	if len(first) != 1 || first[0].ProjectionID != "projection_002" {
		t.Fatalf("first=%+v, want latest tenant projection", first)
	}
	got, ok, err := s.GetDashboardProjection(ctx, "ten_dashboard", first[0].ProjectionID)
	if err != nil {
		t.Fatalf("GetDashboardProjection: %v", err)
	}
	if !ok || got.CampaignStatusCounts["failed"] != 1 {
		t.Fatalf("projection=%+v ok=%v, want persisted projection", got, ok)
	}
	second, err := s.ListDashboardProjections(ctx, evaluation.ProductListFilter{TenantID: "ten_dashboard", Cursor: first[0].ProjectionID, Limit: 10})
	if err != nil {
		t.Fatalf("ListDashboardProjections(second): %v", err)
	}
	if len(second) != 1 || second[0].ProjectionID != "projection_001" {
		t.Fatalf("second=%+v, want deterministic next projection without cross-tenant data", second)
	}

	if err := s.ApplyRetention(ctx, evaluation.RetentionApplicationFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_dashboard"}, ResourceKinds: []evaluation.ProductResourceKind{evaluation.ProductResourceDashboardProjection}}); err != nil {
		t.Fatalf("ApplyRetention(dashboard): %v", err)
	}
	expired, ok, err := s.GetDashboardProjection(ctx, "ten_dashboard", "projection_002")
	if err != nil {
		t.Fatalf("GetDashboardProjection(after retention): %v", err)
	}
	if !ok || expired.RetentionState != evaluation.RetentionStateExpired {
		t.Fatalf("projection after retention=%+v ok=%v, want expired detail", expired, ok)
	}
	listed, err := s.ListDashboardProjections(ctx, evaluation.ProductListFilter{TenantID: "ten_dashboard"})
	if err != nil {
		t.Fatalf("ListDashboardProjections(after retention): %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed=%+v, want retention-filtered dashboard list", listed)
	}
}
