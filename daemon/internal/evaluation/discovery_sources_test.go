package evaluation

import (
	"context"
	"errors"
	"testing"
)

type fakeDiscoverySourceReader struct {
	records []DiscoverySourceRecord
	cursor  string
	filter  DiscoverySourceFilter
}

func (r *fakeDiscoverySourceReader) ListDiscoverySources(_ context.Context, filter DiscoverySourceFilter) ([]DiscoverySourceRecord, string, error) {
	r.filter = filter
	return r.records, r.cursor, nil
}

func TestDiscoverySourceRefsRequireSingleTenant(t *testing.T) {
	refs, err := CollectDiscoverySourceRefs("ten_eval", []DiscoverySourceRecord{
		{TenantID: "ten_eval", Kind: SourceKindRun, ID: "run_1"},
		{TenantID: "ten_eval", Kind: SourceKindWorkflow, ID: "workflow_1"},
		{TenantID: "ten_eval", Kind: SourceKind("tool_call"), ID: "tool_call_1"},
		{TenantID: "ten_eval", Kind: SourceKindFixture, ID: "fixture_1"},
		{TenantID: "ten_eval", Kind: SourceKind("live_validation_ledger"), ID: "ledger_1"},
	})
	if err != nil {
		t.Fatalf("CollectDiscoverySourceRefs: %v", err)
	}
	if len(refs) != 5 {
		t.Fatalf("refs=%+v, want 5", refs)
	}
	if refs[2].Route != "/v1/tool-calls/tool_call_1" {
		t.Fatalf("tool call route=%q", refs[2].Route)
	}
}

func TestDiscoverySourceRefsRejectCrossTenantRecords(t *testing.T) {
	_, err := CollectDiscoverySourceRefs("ten_eval", []DiscoverySourceRecord{
		{TenantID: "ten_eval", Kind: SourceKindRun, ID: "run_1"},
		{TenantID: "ten_other", Kind: SourceKindWorkflow, ID: "workflow_1"},
	})
	if !errors.Is(err, ErrEvaluationProductCrossTenantSource) {
		t.Fatalf("err=%v, want ErrEvaluationProductCrossTenantSource", err)
	}
}

func TestReadDiscoverySourceRefsNormalizesBoundsAndRejectsCrossTenantRows(t *testing.T) {
	reader := &fakeDiscoverySourceReader{
		cursor: "cursor_next",
		records: []DiscoverySourceRecord{
			{TenantID: "ten_eval", Kind: SourceKindRun, ID: "run_1"},
		},
	}
	refs, cursor, err := ReadDiscoverySourceRefs(context.Background(), reader, DiscoverySourceFilter{TenantID: "ten_eval"})
	if err != nil {
		t.Fatalf("ReadDiscoverySourceRefs: %v", err)
	}
	if reader.filter.Limit != DefaultProductPageLimit {
		t.Fatalf("limit=%d, want default %d", reader.filter.Limit, DefaultProductPageLimit)
	}
	if cursor != "cursor_next" || len(refs) != 1 || refs[0].Route != "/v1/runs/run_1" {
		t.Fatalf("unexpected refs/cursor: refs=%+v cursor=%s", refs, cursor)
	}

	reader.records = []DiscoverySourceRecord{{TenantID: "ten_other", Kind: SourceKindRun, ID: "run_2"}}
	if _, _, err := ReadDiscoverySourceRefs(context.Background(), reader, DiscoverySourceFilter{TenantID: "ten_eval"}); !errors.Is(err, ErrEvaluationProductCrossTenantSource) {
		t.Fatalf("err=%v, want cross-tenant source error", err)
	}
}
