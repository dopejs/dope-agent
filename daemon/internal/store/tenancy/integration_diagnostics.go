package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type IntegrationDiagnostics struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewIntegrationDiagnostics(s *store.SQLiteStore, emitter *audit.Emitter) *IntegrationDiagnostics {
	return &IntegrationDiagnostics{store: s, emitter: emitter}
}

func (a *IntegrationDiagnostics) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *IntegrationDiagnostics) SaveRunForTenant(ctx context.Context, item integrations.DiagnosticRun) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.store.SaveIntegrationDiagnosticRun(ctx, item); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveRunForTenant", "integration_diagnostic_run")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *IntegrationDiagnostics) SaveResultForTenant(ctx context.Context, item integrations.DiagnosticResult) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.store.SaveIntegrationDiagnosticResult(ctx, item); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveResultForTenant", "integration_diagnostic_result")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *IntegrationDiagnostics) ListLatestForTenant(ctx context.Context, filter integrations.DiagnosticResultFilter) ([]integrations.DiagnosticResult, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.LatestIntegrationDiagnosticResults(ctx, filter)
}

func (a *IntegrationDiagnostics) ListRunsForTenant(ctx context.Context, filter integrations.DiagnosticRunFilter) ([]integrations.DiagnosticRun, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListIntegrationDiagnosticRuns(ctx, filter)
}

func (a *IntegrationDiagnostics) GetRunForTenant(ctx context.Context, runID string) (integrations.DiagnosticRun, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return integrations.DiagnosticRun{}, false, err
	}
	return a.store.GetIntegrationDiagnosticRun(ctx, tenantID, runID, false)
}

func (a *IntegrationDiagnostics) SaveProviderClassificationForTenant(ctx context.Context, item integrations.ProviderErrorClassification) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	return a.store.SaveProviderClassification(ctx, item)
}

func (a *IntegrationDiagnostics) SaveSmokeReportForTenant(ctx context.Context, item opsreadiness.SmokeMatrixReport) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	for index := range item.ProbeOutcomes {
		item.ProbeOutcomes[index].TenantID = tenantID
	}
	return a.store.SaveSmokeMatrixReport(ctx, item)
}
