package api

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
)

// Roadmap 35 round-3 Finding #1: GET /v1/llm/dispatches previously
// returned every tenant's dispatches because listLLMDispatches called
// store.ListLLMDispatches without scoping. The fix routes the result
// through filterLLMDispatchesByTenant. This test proves dual-tenant
// isolation: tenant A sees only A's dispatch, tenant B sees only B's,
// and a context with no tenant sees everything (admin/maintenance).
func TestListLLMDispatches_DualTenantIsolation(t *testing.T) {
	s := newStoreT(t)
	now := time.Now().UTC()

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})

	dispatchA := llm.Dispatch{
		DispatchID: "dsp_a", Provider: "openai", Model: "gpt-a",
		Messages: []llm.Message{}, Status: llm.DispatchStatusCompleted,
		Output: "A-output", Usage: llm.Usage{}, TimeoutMs: 1000,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := persistLLMDispatch(ctxA, s, dispatchA); err != nil {
		t.Fatalf("persistLLMDispatch A: %v", err)
	}

	dispatchB := llm.Dispatch{
		DispatchID: "dsp_b", Provider: "openai", Model: "gpt-b",
		Messages: []llm.Message{}, Status: llm.DispatchStatusCompleted,
		Output: "B-output", Usage: llm.Usage{}, TimeoutMs: 1000,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := persistLLMDispatch(ctxB, s, dispatchB); err != nil {
		t.Fatalf("persistLLMDispatch B: %v", err)
	}

	listA, err := listLLMDispatches(ctxA, s)
	if err != nil {
		t.Fatalf("listLLMDispatches A: %v", err)
	}
	if len(listA) != 1 || listA[0].DispatchID != "dsp_a" {
		t.Fatalf("tenant A list MUST contain only A's dispatch, got %+v", listA)
	}

	listB, err := listLLMDispatches(ctxB, s)
	if err != nil {
		t.Fatalf("listLLMDispatches B: %v", err)
	}
	if len(listB) != 1 || listB[0].DispatchID != "dsp_b" {
		t.Fatalf("tenant B list MUST contain only B's dispatch, got %+v", listB)
	}

	// No-tenant context (admin/maintenance) keeps the unfiltered view.
	listAll, err := listLLMDispatches(context.Background(), s)
	if err != nil {
		t.Fatalf("listLLMDispatches (no tenant): %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("admin list MUST surface both dispatches, got %d", len(listAll))
	}
}
