package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 review remediation — Finding #1 + #2 follow-up: prove the
// production sub-resource persist paths (steps, tool_calls,
// llm_dispatches) are atomic on cross-tenant collision and bind
// tenant_id at write time rather than via legacy upsert + post-bind.

func TestPersistStep_CrossTenantCollisionIsAtomicallyRefused(t *testing.T) {
	s := newStoreT(t)
	now := time.Now().UTC()

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})

	// Seed parent runs to satisfy steps.run_id FK.
	if err := persistRun(ctxA, s, runtime.Run{
		RunID: "run_a", Entrypoint: "t", Status: runtime.RunStatusQueued,
		Goal: "ga", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed run_a: %v", err)
	}
	if err := persistRun(ctxB, s, runtime.Run{
		RunID: "run_b", Entrypoint: "t", Status: runtime.RunStatusQueued,
		Goal: "gb", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed run_b: %v", err)
	}

	original := runtime.Step{
		StepID: "step_collide", RunID: "run_a", Title: "owned-by-A", Kind: "model",
		Status: runtime.StepStatusQueued, CreatedAt: now, UpdatedAt: now,
	}
	if err := persistStep(ctxA, s, original); err != nil {
		t.Fatalf("persistStep A: %v", err)
	}

	collision := runtime.Step{
		StepID: "step_collide", RunID: "run_b", Title: "B-CLOBBERED", Kind: "model",
		Status: runtime.StepStatusFailed, CreatedAt: now, UpdatedAt: now,
	}
	err := persistStep(ctxB, s, collision)
	if !errors.Is(err, ErrTenantOwnershipDenied) {
		t.Fatalf("cross-tenant persistStep must return ErrTenantOwnershipDenied, got %v", err)
	}

	steps, err := s.ListSteps(context.Background(), "run_a")
	if err != nil {
		t.Fatalf("ListSteps: %v", err)
	}
	var got runtime.Step
	for _, st := range steps {
		if st.StepID == "step_collide" {
			got = st
		}
	}
	if got.StepID == "" {
		t.Fatalf("row vanished after refused write")
	}
	if got.Title != "owned-by-A" {
		t.Fatalf("tenant A step.title was clobbered: got %q", got.Title)
	}
	if got.RunID != "run_a" {
		t.Fatalf("tenant A step.run_id was clobbered: got %q", got.RunID)
	}
	if got.Status != runtime.StepStatusQueued {
		t.Fatalf("tenant A step.status was clobbered: got %q", got.Status)
	}
}

func TestPersistToolCall_CrossTenantCollisionIsAtomicallyRefused(t *testing.T) {
	s := newStoreT(t)
	now := time.Now().UTC()

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})

	// Seed parent run + step rows to satisfy tool_calls FKs.
	for tenantCtx, tenantID := range map[context.Context]string{ctxA: "run_a", ctxB: "run_b"} {
		if err := persistRun(tenantCtx, s, runtime.Run{
			RunID: tenantID, Entrypoint: "t", Status: runtime.RunStatusQueued,
			Goal: "g", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed %s: %v", tenantID, err)
		}
	}
	if err := persistStep(ctxA, s, runtime.Step{
		StepID: "step_a", RunID: "run_a", Title: "t", Kind: "model",
		Status: runtime.StepStatusQueued, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed step_a: %v", err)
	}
	if err := persistStep(ctxB, s, runtime.Step{
		StepID: "step_b", RunID: "run_b", Title: "t", Kind: "model",
		Status: runtime.StepStatusQueued, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed step_b: %v", err)
	}

	original := runtime.ToolCall{
		ToolCallID: "tc_collide", RunID: "run_a", StepID: "step_a",
		CapabilityID: "cap_a", ToolName: "owned-by-A",
		Status:    runtime.ToolCallStatusRequested,
		CreatedAt: now, UpdatedAt: now,
	}
	// Bypass the production helper's parent-existence check by going
	// directly to UpsertToolCallForTenantSafe — the production helper
	// also writes parents which is out of scope for this regression.
	if err := s.UpsertToolCallForTenantSafe(ctxA, original, "ten_aaaa1111"); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	collision := runtime.ToolCall{
		ToolCallID: "tc_collide", RunID: "run_b", StepID: "step_b",
		CapabilityID: "cap_b", ToolName: "B-CLOBBERED",
		Status:    runtime.ToolCallStatusFailed,
		CreatedAt: now, UpdatedAt: now,
	}
	err := s.UpsertToolCallForTenantSafe(ctxB, collision, "ten_bbbb2222")
	if !errors.Is(err, store.ErrCrossTenantRow) {
		t.Fatalf("cross-tenant tool_call write must return ErrCrossTenantRow, got %v", err)
	}

	got, err := s.ListToolCallsForTenantRaw(context.Background(), "ten_aaaa1111", "run_a", "step_a")
	if err != nil {
		t.Fatalf("ListToolCallsForTenantRaw: %v", err)
	}
	if len(got) != 1 || got[0].ToolName != "owned-by-A" {
		t.Fatalf("tenant A row was mutated by refused B write: %+v", got)
	}
}

func TestPersistLLMDispatch_CrossTenantCollisionIsAtomicallyRefused(t *testing.T) {
	s := newStoreT(t)
	now := time.Now().UTC()

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})

	original := llm.Dispatch{
		DispatchID: "dsp_collide", Provider: "openai", Model: "owned-by-A",
		Messages:  []llm.Message{},
		Status:    llm.DispatchStatusCompleted,
		Output:    "A-output",
		Usage:     llm.Usage{},
		TimeoutMs: 1000,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := persistLLMDispatch(ctxA, s, original); err != nil {
		t.Fatalf("persistLLMDispatch A: %v", err)
	}

	collision := llm.Dispatch{
		DispatchID: "dsp_collide", Provider: "openai", Model: "B-CLOBBERED",
		Messages:  []llm.Message{},
		Status:    llm.DispatchStatusFailed,
		Output:    "B-output",
		Usage:     llm.Usage{},
		TimeoutMs: 1000,
		CreatedAt: now, UpdatedAt: now,
	}
	err := persistLLMDispatch(ctxB, s, collision)
	if !errors.Is(err, ErrTenantOwnershipDenied) {
		t.Fatalf("cross-tenant persistLLMDispatch must return ErrTenantOwnershipDenied, got %v", err)
	}

	got, _, err := s.GetLLMDispatch(context.Background(), "dsp_collide")
	if err != nil {
		t.Fatalf("GetLLMDispatch: %v", err)
	}
	if got.Model != "owned-by-A" {
		t.Fatalf("tenant A dispatch.model was clobbered: got %q", got.Model)
	}
	if got.Output != "A-output" {
		t.Fatalf("tenant A dispatch.output was clobbered: got %q", got.Output)
	}
}

func newStoreT(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
