package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestChannelManagementRepairRequiresSecretsForReconnectAndKeepsDisabledTerminal(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "slack-main", "slack", "Slack Main")
	if _, err := supervisor.Disable("slack-main", "maintenance"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	deniedReq := httptest.NewRequest(http.MethodPost, "/v1/channel-management/connectors/slack-main/repair-actions", bytes.NewBufferString(`{"actionKind":"reconnect"}`))
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	deniedRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("reconnect without secrets status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/channel-management/connectors/slack-main/repair-actions", bytes.NewBufferString(`{"actionKind":"reconnect","sourceDiagnosticStateId":"diag_1"}`))
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage, identity.PermissionSecretsManage)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("repair status=%d body=%s", rec.Code, rec.Body.String())
	}
	var action connectors.RepairAction
	if err := json.Unmarshal(rec.Body.Bytes(), &action); err != nil {
		t.Fatalf("decode repair action: %v", err)
	}
	if action.Status != connectors.ManagementTerminalDisabled || action.SourceDiagnosticStateID != "diag_1" {
		t.Fatalf("unexpected disabled repair action: %+v", action)
	}
}
