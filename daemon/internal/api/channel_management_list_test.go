package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestChannelManagementListDetailDiagnosticsAreTenantScopedOrderedAndPermissioned(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "ready-main", "discord", "Ready Main")
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "broken-main", "slack", "Broken Main")
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "disabled-main", "telegram", "Disabled Main")
	registerChannelManagementTestConnector(t, supervisor, "ten_other", "other-main", "matrix", "Other Main")
	if _, err := supervisor.Disable("disabled-main", "tenant_disabled"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	now := time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	diagnostic, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		DiagnosticStateID: "diag_broken",
		TenantID:          "ten_channels",
		ConnectorID:       "broken-main",
		ReasonCode:        connectors.DiagnosticPermissionMissing,
		EvidenceTimestamp: now,
		RedactionReliable: true,
		SafeEvidence:      map[string]string{"workspace": "workspace_redacted"},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), diagnostic); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors?limit=2", nil)
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list connectors.ChannelConnectorListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 2 || list.Items[0].ConnectorID != "broken-main" || list.Items[1].ConnectorID != "disabled-main" {
		t.Fatalf("unexpected ordered page: %+v", list.Items)
	}
	if list.Page.NextCursor == "" || list.TenantID != "ten_channels" {
		t.Fatalf("expected tenant page cursor, got %+v", list.Page)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/broken-main", nil)
	detailReq = detailReq.WithContext(withTenantContext(detailReq.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	detailRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detail connectors.ChannelConnectorDetail
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.DiagnosticSummary == nil || detail.DiagnosticSummary.DiagnosticStateID != "diag_broken" {
		t.Fatalf("expected diagnostic summary, got %+v", detail.DiagnosticSummary)
	}
	if detail.RoutePolicy == nil || !detail.RoutePolicy.BackgroundDeliveryEligible {
		t.Fatalf("expected default route policy, got %+v", detail.RoutePolicy)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/broken-main/diagnostics", nil)
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	deniedRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("diagnostics denial status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}
	var denialAuditCount int
	if err := sqliteStore.DB().QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM channel_management_audit_records
		WHERE tenant_id = ? AND connector_id = ? AND action = ? AND outcome = ?
	`, "ten_channels", "broken-main", "channel_management.diagnostics", "denied").Scan(&denialAuditCount); err != nil {
		t.Fatalf("query denial audit: %v", err)
	}
	if denialAuditCount != 1 {
		t.Fatalf("expected diagnostics denial audit, got %d", denialAuditCount)
	}
}

func registerChannelManagementTestConnector(t *testing.T, supervisor *connectors.Supervisor, tenantID, connectorID, kind, displayName string) {
	t.Helper()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    tenantID,
		ConnectorID: connectorID,
		Kind:        kind,
		DisplayName: displayName,
	}); err != nil {
		t.Fatalf("Register %s: %v", connectorID, err)
	}
}

func channelManagementTenantContext(permissions ...identity.Permission) identity.TenantContext {
	return identity.TenantContext{
		TenantID:    "ten_channels",
		PrincipalID: "prn_channels",
		Permissions: permissions,
	}
}
