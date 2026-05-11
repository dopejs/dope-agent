package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestConnectorIngressRecordsDisabledRoutingDecisionBeforeRunCreation(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "telegram-main", "telegram", "Telegram Main")
	connector, ok := supervisor.GetForTenant("telegram-main", "ten_channels")
	if !ok {
		t.Fatal("expected registered connector")
	}
	if err := sqliteStore.UpsertConnector(context.Background(), connector); err != nil {
		t.Fatalf("UpsertConnector: %v", err)
	}
	if _, err := supervisor.Disable(connector.ConnectorID, "maintenance"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if err := sqliteStore.SaveChannelConnectorEnablementState(context.Background(), connectors.EnablementState{
		TenantID:     "ten_channels",
		ConnectorID:  connector.ConnectorID,
		State:        "disabled",
		ChangedAt:    time.Now().UTC(),
		AuditEventID: "audit_disable",
	}); err != nil {
		t.Fatalf("SaveChannelConnectorEnablementState: %v", err)
	}

	manager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	t.Cleanup(func() { _ = checkpointManager.Close() })
	req := httptest.NewRequest(http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", strings.NewReader(`{
		"tenantId":"ten_channels",
		"route":{"kind":"direct","peerId":"dm-1"},
		"message":{"messageId":"msg_disabled"},
		"run":{"entrypoint":"connector.message"}
	}`))
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext()))
	rec := httptest.NewRecorder()
	handleConnectorIngressMessages(supervisor, router.NewSessionRouter(), manager, events.NewBus(), sqliteStore, checkpointManager, rec, req, connector.ConnectorID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("disabled ingress status=%d body=%s", rec.Code, rec.Body.String())
	}
	decisions, err := sqliteStore.ListChannelRoutingDecisions(context.Background(), "ten_channels", connector.ConnectorID, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListChannelRoutingDecisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Outcome != connectors.RouteDecisionDisabled || decisions[0].ReasonCode != "connector_disabled" {
		t.Fatalf("expected disabled routing decision, got %+v", decisions)
	}
	runs, err := sqliteStore.ListRunsAllTenantsForTest(context.Background())
	if err != nil {
		t.Fatalf("ListRunsAllTenantsForTest: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("disabled ingress created runs: %+v", runs)
	}
}
