package api

import (
	"context"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type runtimeProfileProjectionTarget struct {
	ResourceKind     profiles.RuntimeResourceKind
	ResourceID       string
	ThreadID         string
	SessionSegmentID string
	RunID            string
	WorkflowID       string
	HandoffID        string
}

func projectSessionProfileProjections(ctx context.Context, sqliteStore *store.SQLiteStore, sessions []router.Session) ([]router.Session, error) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" || sqliteStore == nil {
		return sessions, nil
	}
	if !canInspectProfileRuntime(ctx) {
		for idx := range sessions {
			sessions[idx].ActiveProfileProjection = nil
		}
		return sessions, nil
	}
	for idx := range sessions {
		projection, err := latestRuntimeProfileProjection(ctx, sqliteStore, tenantContext.TenantID, profiles.RuntimeResourceSession, sessions[idx].SessionID)
		if err != nil {
			return nil, err
		}
		sessions[idx].ActiveProfileProjection = projection
	}
	return sessions, nil
}

func recordActiveProfileProjectionForTarget(ctx context.Context, sqliteStore *store.SQLiteStore, eventBus *events.Bus, target runtimeProfileProjectionTarget) (*profiles.RuntimeProjection, error) {
	if sqliteStore == nil || strings.TrimSpace(target.ResourceID) == "" {
		return nil, nil
	}
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" {
		return nil, nil
	}
	profile, selection, found, err := sqliteStore.ActiveAgentProfileSelection(ctx, strings.TrimSpace(tenantContext.TenantID))
	if err != nil || !found {
		return nil, err
	}
	projection := profiles.BuildRuntimeProjection(profile, selection, profiles.RuntimeProjectionInput{
		ResourceKind: target.ResourceKind,
		ResourceID:   strings.TrimSpace(target.ResourceID),
		ThreadID:     strings.TrimSpace(target.ThreadID),
		SessionID:    strings.TrimSpace(target.SessionSegmentID),
		RunID:        strings.TrimSpace(target.RunID),
		WorkflowID:   strings.TrimSpace(target.WorkflowID),
		HandoffID:    strings.TrimSpace(target.HandoffID),
		OccurredAt:   time.Now().UTC(),
	})
	recorded, err := sqliteStore.RecordRuntimeProfileProjection(ctx, projection)
	if err != nil {
		return nil, err
	}
	if eventBus != nil {
		if _, err := publishEvent(ctx, eventBus, sqliteStore, events.AgentProfileRuntimeProjectedEvent(recorded)); err != nil {
			return nil, err
		}
	}
	return &recorded, nil
}

func latestRuntimeProfileProjection(ctx context.Context, sqliteStore *store.SQLiteStore, tenantID string, resourceKind profiles.RuntimeResourceKind, resourceID string) (*profiles.RuntimeProjection, error) {
	if sqliteStore == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(resourceID) == "" {
		return nil, nil
	}
	items, err := sqliteStore.ListRuntimeProfileProjections(ctx, strings.TrimSpace(tenantID), string(resourceKind), strings.TrimSpace(resourceID), "", 1)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func projectRunProfileProjections(ctx context.Context, sqliteStore *store.SQLiteStore, runs []runtime.Run) ([]runtime.Run, error) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" || sqliteStore == nil {
		return runs, nil
	}
	if !canInspectProfileRuntime(ctx) {
		for idx := range runs {
			runs[idx].ActiveProfileProjection = nil
		}
		return runs, nil
	}
	for idx := range runs {
		projection, err := latestRuntimeProfileProjection(ctx, sqliteStore, tenantContext.TenantID, profiles.RuntimeResourceRun, runs[idx].RunID)
		if err != nil {
			return nil, err
		}
		runs[idx].ActiveProfileProjection = projection
	}
	return runs, nil
}

func projectRunProfileProjection(ctx context.Context, sqliteStore *store.SQLiteStore, run runtime.Run) (runtime.Run, error) {
	items, err := projectRunProfileProjections(ctx, sqliteStore, []runtime.Run{run})
	if err != nil || len(items) == 0 {
		return runtime.Run{}, err
	}
	return items[0], nil
}

func projectWorkflowProfileProjections(ctx context.Context, sqliteStore *store.SQLiteStore, workflows []orchestration.Workflow) ([]orchestration.Workflow, error) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" || sqliteStore == nil {
		return workflows, nil
	}
	if !canInspectProfileRuntime(ctx) {
		for idx := range workflows {
			workflows[idx].ActiveProfileProjection = nil
		}
		return workflows, nil
	}
	for idx := range workflows {
		projection, err := latestRuntimeProfileProjection(ctx, sqliteStore, tenantContext.TenantID, profiles.RuntimeResourceWorkflow, workflows[idx].WorkflowID)
		if err != nil {
			return nil, err
		}
		workflows[idx].ActiveProfileProjection = projection
	}
	return workflows, nil
}

func projectWorkflowProfileProjection(ctx context.Context, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow) (orchestration.Workflow, error) {
	items, err := projectWorkflowProfileProjections(ctx, sqliteStore, []orchestration.Workflow{workflow})
	if err != nil || len(items) == 0 {
		return orchestration.Workflow{}, err
	}
	return items[0], nil
}

func canInspectProfileRuntime(ctx context.Context) bool {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok {
		return false
	}
	return identity.RequirePermission(tenantContext, identity.PermissionProfilesInspect) == nil
}
