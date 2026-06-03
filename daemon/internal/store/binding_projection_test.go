package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
)

// B13/B41: runtime binding evidence records the selection and applied classification.
func TestRecordRuntimeBindingEvidence_AppliedClassification(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	sel := bindings.EffectiveBindingSelection{
		Outcome:             bindings.OutcomeResolved,
		BindingScope:        bindings.RuntimeScopeChannel,
		BindingID:           "bnd_1",
		SelectedProfileID:   "prof_1",
		SelectedWorkspaceID: "ws_1",
	}
	ev := bindings.BuildRuntimeBindingEvidence(sel, bindings.RuntimeBindingEvidenceInput{TenantID: "ten_p", ResourceKind: "thread", ResourceID: "thr_1", OccurredAt: time.Now().UTC()})
	if _, err := s.RecordRuntimeBindingEvidence(ctx, ev); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, found, err := s.LatestRuntimeBindingEvidence(ctx, "ten_p", "thread", "thr_1")
	if err != nil || !found {
		t.Fatalf("latest: %v found=%v", err, found)
	}
	if got.Classification != bindings.ClassificationApplied {
		t.Fatalf("expected applied classification, got %s", got.Classification)
	}
	if got.SelectedProfileID != "prof_1" || got.SelectedWorkspaceID != "ws_1" {
		t.Fatalf("evidence selection mismatch: %+v", got)
	}
}

// B15/FR-012: evidence is append-only; a later binding change does not rewrite history.
func TestRuntimeBindingEvidence_HistoricalImmutability(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	first := bindings.BuildRuntimeBindingEvidence(
		bindings.EffectiveBindingSelection{Outcome: bindings.OutcomeResolved, BindingScope: bindings.RuntimeScopeChannel, SelectedProfileID: "prof_old", SelectedWorkspaceID: "ws_old"},
		bindings.RuntimeBindingEvidenceInput{TenantID: "ten_h", ResourceKind: "thread", ResourceID: "thr_h", OccurredAt: time.Now().Add(-time.Hour).UTC()},
	)
	if _, err := s.RecordRuntimeBindingEvidence(ctx, first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	second := bindings.BuildRuntimeBindingEvidence(
		bindings.EffectiveBindingSelection{Outcome: bindings.OutcomeResolved, BindingScope: bindings.RuntimeScopeChannel, SelectedProfileID: "prof_new", SelectedWorkspaceID: "ws_new"},
		bindings.RuntimeBindingEvidenceInput{TenantID: "ten_h", ResourceKind: "thread", ResourceID: "thr_h", OccurredAt: time.Now().UTC()},
	)
	if _, err := s.RecordRuntimeBindingEvidence(ctx, second); err != nil {
		t.Fatalf("record second: %v", err)
	}
	items, err := s.ListRuntimeBindingEvidence(ctx, "ten_h", "thread", "thr_h", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 historical records, got %d", len(items))
	}
	// Oldest still references the original selection (no rewrite).
	if items[1].SelectedProfileID != "prof_old" {
		t.Fatalf("historical evidence was rewritten: %+v", items[1])
	}
}
