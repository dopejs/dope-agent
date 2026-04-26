package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Workflows is the tenant-aware accessor for the workflows family
// (workflows, workflow_steps, workflow_dependencies, workflow_handoffs).
// Pass A: workflow_steps / workflow_dependencies / workflow_handoffs are
// CRUD'd via the orchestration package against the parent workflow row;
// the binding here covers the parent workflow and inherits to children
// at backfill time per the parent-child orphan rule documented in T067.
type Workflows struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewWorkflows(s *store.SQLiteStore, emitter *audit.Emitter) *Workflows {
	return &Workflows{store: s, emitter: emitter}
}

func (a *Workflows) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Workflows) ListWorkflowsForTenant(ctx context.Context, environmentScope, runID string) ([]orchestration.Workflow, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListWorkflowsForTenantRaw(ctx, tenantID, environmentScope, runID)
}

func (a *Workflows) GetWorkflowForTenant(ctx context.Context, environmentScope, runID, workflowID string) (orchestration.Workflow, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return orchestration.Workflow{}, false, err
	}
	owner, ok, err := a.store.LookupRowTenant(ctx, "workflows", "workflow_id", workflowID)
	if err != nil {
		return orchestration.Workflow{}, false, err
	}
	if !ok {
		return orchestration.Workflow{}, false, nil
	}
	if owner != "" && owner != tenantID {
		a.emit(ctx, "store:GetWorkflowForTenant", "workflow")
		return orchestration.Workflow{}, false, nil
	}
	return a.store.GetWorkflow(ctx, environmentScope, runID, workflowID)
}

func (a *Workflows) UpsertWorkflowForTenant(ctx context.Context, workflow orchestration.Workflow) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertWorkflow(ctx, workflow); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "workflows", "workflow_id", workflow.WorkflowID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertWorkflowForTenant", "workflow")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
