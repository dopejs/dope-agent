package evaluation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEvaluationProductCrossTenantSource = errors.New("evaluation product source belongs to another tenant")

type DiscoverySourceRecord struct {
	TenantID   string
	Kind       SourceKind
	ID         string
	ObservedAt time.Time
	Payload    map[string]any
}

type DiscoverySourceFilter struct {
	TenantID    string
	SourceKinds []SourceKind
	WindowStart time.Time
	WindowEnd   time.Time
	Limit       int
	Cursor      string
}

type DiscoverySourceReader interface {
	ListDiscoverySources(context.Context, DiscoverySourceFilter) ([]DiscoverySourceRecord, string, error)
}

func ReadDiscoverySourceRefs(ctx context.Context, reader DiscoverySourceReader, filter DiscoverySourceFilter) ([]SourceRef, string, error) {
	if reader == nil {
		return nil, "", errors.New("evaluation discovery source reader required")
	}
	if err := ValidateTenantScopedProductRequest(filter.TenantID); err != nil {
		return nil, "", err
	}
	filter.Limit = NormalizeProductLimit(filter.Limit)
	records, nextCursor, err := reader.ListDiscoverySources(ctx, filter)
	if err != nil {
		return nil, "", err
	}
	refs, err := CollectDiscoverySourceRefs(filter.TenantID, records)
	if err != nil {
		return nil, "", err
	}
	return refs, nextCursor, nil
}

func CollectDiscoverySourceRefs(tenantID string, records []DiscoverySourceRecord) ([]SourceRef, error) {
	if err := ValidateTenantScopedProductRequest(tenantID); err != nil {
		return nil, err
	}
	refs := make([]SourceRef, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.TenantID) != strings.TrimSpace(tenantID) {
			return nil, ErrEvaluationProductCrossTenantSource
		}
		if strings.TrimSpace(record.ID) == "" {
			continue
		}
		refs = append(refs, SourceRef{
			Kind:  record.Kind,
			ID:    record.ID,
			Route: discoverySourceRoute(record.Kind, record.ID),
		})
	}
	return refs, nil
}

func discoverySourceRoute(kind SourceKind, id string) string {
	switch kind {
	case SourceKindRun:
		return fmt.Sprintf("/v1/runs/%s", id)
	case SourceKindWorkflow:
		return fmt.Sprintf("/v1/workflows/%s", id)
	case SourceKindFixture:
		return fmt.Sprintf("/v1/evaluation/fixtures/%s", id)
	case SourceKind("tool_call"):
		return fmt.Sprintf("/v1/tool-calls/%s", id)
	case SourceKind("live_validation_ledger"):
		return fmt.Sprintf("/v1/live-validations/ledger/%s", id)
	default:
		return ""
	}
}
