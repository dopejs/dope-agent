package evaluation

import (
	"testing"
	"time"
)

func TestPageDashboardProjectionsIsDeterministic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	items := []DashboardProjection{
		{ProjectionID: "projection_a", GeneratedAt: now.Add(-2 * time.Minute)},
		{ProjectionID: "projection_c", GeneratedAt: now},
		{ProjectionID: "projection_b", GeneratedAt: now.Add(-time.Minute)},
	}

	first, cursor := PageDashboardProjections(items, "", 2)
	if len(first) != 2 || first[0].ProjectionID != "projection_c" || first[1].ProjectionID != "projection_b" || cursor != "projection_b" {
		t.Fatalf("first page=%+v cursor=%q, want c,b cursor b", first, cursor)
	}
	second, next := PageDashboardProjections(items, cursor, 2)
	if len(second) != 1 || second[0].ProjectionID != "projection_a" || next != "" {
		t.Fatalf("second page=%+v cursor=%q, want a and no next cursor", second, next)
	}
}
