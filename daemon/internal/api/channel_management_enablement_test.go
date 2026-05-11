package api

import (
	"bytes"
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

func TestChannelManagementDisableReEnablePersistsAuditAndRejectsStaleDiagnostics(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "discord-main", "discord", "Discord Main")

	disableReq := httptest.NewRequest(http.MethodPost, "/v1/channel-management/connectors/discord-main/disable", bytes.NewBufferString(`{"reasonCode":"maintenance"}`))
	disableReq = disableReq.WithContext(withTenantContext(disableReq.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	disableRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}
	var result connectors.EnablementMutationResult
	if err := json.Unmarshal(disableRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode disable: %v", err)
	}
	if result.EnablementState != connectors.ManagementStateDisabled || result.DeliveryEligible {
		t.Fatalf("unexpected disable result: %+v", result)
	}
	state, ok, err := sqliteStore.GetChannelConnectorEnablementState(context.Background(), "ten_channels", "discord-main")
	if err != nil || !ok {
		t.Fatalf("GetChannelConnectorEnablementState ok=%v err=%v", ok, err)
	}
	if state.State != "disabled" || state.AuditEventID == "" {
		t.Fatalf("expected disabled persisted state, got %+v", state)
	}

	stale, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		DiagnosticStateID: "diag_stale",
		TenantID:          "ten_channels",
		ConnectorID:       "discord-main",
		ReasonCode:        connectors.DiagnosticNetworkFailed,
		EvidenceTimestamp: time.Now().UTC().Add(-20 * time.Minute),
		RedactionReliable: true,
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), stale); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState: %v", err)
	}
	enableReq := httptest.NewRequest(http.MethodPost, "/v1/channel-management/connectors/discord-main/re-enable", nil)
	enableReq = enableReq.WithContext(withTenantContext(enableReq.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	enableRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, enableRec, enableReq)
	if enableRec.Code != http.StatusConflict {
		t.Fatalf("stale re-enable status=%d body=%s", enableRec.Code, enableRec.Body.String())
	}
}

func TestChannelManagementDisableFailsClosedWhenEnablementPersistenceFails(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "discord-main", "discord", "Discord Main")
	if _, err := sqliteStore.DB().Exec(`
		CREATE TRIGGER fail_channel_enablement_insert
		BEFORE INSERT ON channel_connector_enablement_states
		BEGIN
			SELECT RAISE(FAIL, 'enablement persistence failed');
		END;
	`); err != nil {
		t.Fatalf("create failing trigger: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/channel-management/connectors/discord-main/disable", bytes.NewBufferString(`{"reasonCode":"maintenance"}`))
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("disable status=%d body=%s", rec.Code, rec.Body.String())
	}
	connector, ok := supervisor.GetForTenant("discord-main", "ten_channels")
	if !ok {
		t.Fatal("expected connector to remain registered")
	}
	if connector.Status == connectors.StatusDisabled {
		t.Fatalf("disable changed supervisor state despite failed persistence: %+v", connector)
	}
}
