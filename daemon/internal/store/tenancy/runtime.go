package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// Runtime is the tenant-aware accessor for the runtime spine
// (sessions, runs, steps, tool_calls, llm_dispatches, checkpoints).
//
// Pass A semantics:
//   - Reads return only rows whose tenant_id equals the caller's tenant
//     (rows still NULL pre-backfill are NOT returned, fail-closed).
//   - Writes call the existing tenantless `*SQLiteStore` helpers and then
//     bind the freshly-written row's tenant_id via BindRowTenant.
//   - By-id lookups whose target row exists in another tenant return
//     not-found AND emit `audit.cross_tenant_access_denied`. The
//     existence of the row is never leaked.
//
// Pass B (post-backfill) collapses the write-then-bind pattern into a
// single tenant-aware INSERT and deletes the existing tenantless
// helpers; the surface defined here stays stable.
type Runtime struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

// NewRuntime constructs a Runtime accessor. emitter may be nil (the
// resulting Runtime will skip audit publication, but callers should wire
// the daemon's shared emitter in production).
func NewRuntime(s *store.SQLiteStore, emitter *audit.Emitter) *Runtime {
	return &Runtime{store: s, emitter: emitter}
}

func (r *Runtime) emit(ctx context.Context, surface, resourceKind string) {
	if r == nil || r.emitter == nil {
		return
	}
	if _, ok := tenantctx.FromContext(ctx); !ok {
		return
	}
	_ = r.emitter.Emit(ctx, surface, resourceKind)
}

// ----- runs -----

func (r *Runtime) ListRunsForTenant(ctx context.Context) ([]runtime.Run, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListRunsForTenantRaw(ctx, tenantID)
}

func (r *Runtime) GetRunForTenant(ctx context.Context, runID string) (runtime.Run, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return runtime.Run{}, false, err
	}
	run, ok, err := r.store.GetRunForTenantRaw(ctx, runID, tenantID)
	if errors.Is(err, store.ErrCrossTenantRow) {
		r.emit(ctx, "store:GetRunForTenant", "run")
		return runtime.Run{}, false, nil
	}
	return run, ok, err
}

func (r *Runtime) UpsertRunForTenant(ctx context.Context, run runtime.Run) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	// Atomic Pass A write: tenant_id is bound in the same INSERT and
	// the ON-CONFLICT branch refuses to update a row owned by a
	// different tenant. No legacy upsert + bind sequence — the prior
	// design leaked B's update onto A's row before the bind step
	// detected the mismatch.
	if err := r.store.UpsertRunForTenantSafe(ctx, run, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			r.emit(ctx, "store:UpsertRunForTenant", "run")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (r *Runtime) DeleteRunForTenant(ctx context.Context, runID string) (bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return false, err
	}
	deleted, err := r.store.DeleteRowForTenant(ctx, "runs", "run_id", runID, tenantID)
	if errors.Is(err, store.ErrCrossTenantRow) {
		r.emit(ctx, "store:DeleteRunForTenant", "run")
		return false, nil
	}
	return deleted, err
}

// ----- sessions -----

func (r *Runtime) ListSessionsForTenant(ctx context.Context) ([]router.Session, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListSessionsForTenantRaw(ctx, tenantID)
}

func (r *Runtime) UpsertSessionForTenant(ctx context.Context, session router.Session) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := r.store.UpsertSessionForTenantSafe(ctx, session, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			r.emit(ctx, "store:UpsertSessionForTenant", "session")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (r *Runtime) DeleteSessionForTenant(ctx context.Context, sessionID string) (bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return false, err
	}
	deleted, err := r.store.DeleteRowForTenant(ctx, "sessions", "session_id", sessionID, tenantID)
	if errors.Is(err, store.ErrCrossTenantRow) {
		r.emit(ctx, "store:DeleteSessionForTenant", "session")
		return false, nil
	}
	return deleted, err
}

// ----- steps -----

func (r *Runtime) ListStepsForTenant(ctx context.Context, runID string) ([]runtime.Step, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListStepsForTenantRaw(ctx, tenantID, runID)
}

func (r *Runtime) UpsertStepForTenant(ctx context.Context, step runtime.Step) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := r.store.UpsertStepForTenantSafe(ctx, step, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			r.emit(ctx, "store:UpsertStepForTenant", "step")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// ----- tool_calls -----

func (r *Runtime) ListToolCallsForTenant(ctx context.Context, runID, stepID string) ([]runtime.ToolCall, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListToolCallsForTenantRaw(ctx, tenantID, runID, stepID)
}

func (r *Runtime) UpsertToolCallForTenant(ctx context.Context, toolCall runtime.ToolCall) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := r.store.UpsertToolCallForTenantSafe(ctx, toolCall, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			r.emit(ctx, "store:UpsertToolCallForTenant", "tool_call")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// ----- llm_dispatches -----

func (r *Runtime) ListLLMDispatchesForTenant(ctx context.Context) ([]llm.Dispatch, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListLLMDispatchesForTenantRaw(ctx, tenantID)
}

func (r *Runtime) GetLLMDispatchForTenant(ctx context.Context, dispatchID string) (llm.Dispatch, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return llm.Dispatch{}, false, err
	}
	d, ok, err := r.store.GetLLMDispatchForTenantRaw(ctx, dispatchID, tenantID)
	if errors.Is(err, store.ErrCrossTenantRow) {
		r.emit(ctx, "store:GetLLMDispatchForTenant", "llm_dispatch")
		return llm.Dispatch{}, false, nil
	}
	return d, ok, err
}

func (r *Runtime) UpsertLLMDispatchForTenant(ctx context.Context, dispatch llm.Dispatch) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := r.store.UpsertLLMDispatchForTenantSafe(ctx, dispatch, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			r.emit(ctx, "store:UpsertLLMDispatchForTenant", "llm_dispatch")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// ----- checkpoints -----

func (r *Runtime) ListLatestCheckpointsForTenant(ctx context.Context) ([]runtime.RunCheckpoint, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return r.store.ListLatestCheckpointsForTenantRaw(ctx, tenantID)
}

// SaveCheckpointForTenant writes a checkpoint row and binds tenant_id to
// it. Because checkpoint_id is generated server-side and not surfaced to
// callers, the bind step uses (run_id, captured_at) as the lookup key.
func (r *Runtime) SaveCheckpointForTenant(ctx context.Context, checkpoint runtime.RunCheckpoint) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := r.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return err
	}
	// SaveCheckpoint inserts unconditionally and may write multiple rows
	// per (run_id, captured_at) over the run's life. Bind tenant_id on
	// every row currently NULL for this run; this is safe because
	// checkpoints under a single run all belong to the same tenant.
	return r.store.BindRunCheckpointsTenant(ctx, checkpoint.Run.RunID, tenantID)
}

// ErrCrossTenantWrite is returned by the Pass A Upsert*ForTenant helpers
// when the underlying row was previously bound to a different tenant —
// the existing row is preserved and the write is refused. Pass B's
// rewritten INSERTs will reject the same condition with the same error
// before any writes take effect.
var ErrCrossTenantWrite = errors.New("tenancy: refused write to row owned by a different tenant")

// RuntimeTenantID resolves the tenant id for an opportunistic write path
// that does NOT yet go through the Pass A Runtime accessor (e.g., legacy
// store helpers being called during the staged rollout). Returns "" when
// the context carries no tenant; the empty string maps to a NULL
// tenant_id column write, which is allowed pre-backfill because every
// tenant_id column was added as NULLABLE in v23/v24+. Pass B replaces
// every remaining caller of this with the strict Runtime methods above.
func RuntimeTenantID(ctx context.Context) string {
	tenantID, err := Require(ctx)
	if err != nil {
		return ""
	}
	return tenantID
}

// RuntimeTenantPredicate returns the SQL predicate fragment + arg used to
// scope a tenant-owned read against a table whose tenant_id may still be
// NULL during the backfill window. Returned fragment is suitable for
// appending after an existing WHERE clause via " AND ". Pass B drops the
// IS NULL branch and requires a strict match.
func RuntimeTenantPredicate(tenantID string) (string, []any) {
	if tenantID == "" {
		return "", nil
	}
	return "(tenant_id = ? OR tenant_id IS NULL)", []any{tenantID}
}
