package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestChannelManagementRoutePolicyAndOutcomeHandlers(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "matrix-main", "matrix", "Matrix Main")

	req := httptest.NewRequest(http.MethodPut, "/v1/channel-management/connectors/matrix-main/route-policy", bytes.NewBufferString(`{"eligibleRooms":["room_redacted"],"backgroundDeliveryEligible":false}`))
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("route update status=%d body=%s", rec.Code, rec.Body.String())
	}
	var policy connectors.RoutePolicy
	if err := json.Unmarshal(rec.Body.Bytes(), &policy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	if policy.BackgroundDeliveryEligible || len(policy.EligibleRooms) != 1 || policy.AuditEventID == "" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	decision, err := sqliteStore.SaveChannelRoutingDecision(req.Context(), connectors.RoutingDecision{
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		ConnectorKind:      "matrix",
		Outcome:            connectors.RouteDecisionBlocked,
		ReasonCode:         "blocked_route",
		OccurredAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	})
	if err != nil {
		t.Fatalf("SaveChannelRoutingDecision: %v", err)
	}
	if _, err := sqliteStore.SaveChannelForegroundReplyOutcome(req.Context(), connectors.ForegroundReplyOutcome{
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		RoutingDecisionID:  decision.RoutingDecisionID,
		Status:             "failed",
		ReasonCode:         "provider_unavailable",
		OccurredAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelForegroundReplyOutcome: %v", err)
	}
	if _, err := sqliteStore.SaveChannelBackgroundDeliveryOutcome(req.Context(), connectors.BackgroundDeliveryOutcome{
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		DeliveryTargetID:   "target_redacted",
		Status:             "blocked",
		ReasonCode:         "connector_disabled",
		OccurredAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelBackgroundDeliveryOutcome: %v", err)
	}

	for _, path := range []string{"reply-outcomes", "delivery-outcomes"} {
		outcomeReq := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/matrix-main/"+path, nil)
		outcomeReq = outcomeReq.WithContext(withTenantContext(outcomeReq.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
		outcomeRec := httptest.NewRecorder()
		handleChannelManagementRoutes(supervisor, nil, sqliteStore, outcomeRec, outcomeReq)
		if outcomeRec.Code != http.StatusOK || !bytes.Contains(outcomeRec.Body.Bytes(), []byte(`"items":[{`)) {
			t.Fatalf("%s status=%d body=%s", path, outcomeRec.Code, outcomeRec.Body.String())
		}
	}
}

func TestChannelManagementRoutePolicyRejectsUnsupportedConnectorKind(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "legacy-main", "legacy", "Legacy Main")

	req := httptest.NewRequest(http.MethodPut, "/v1/channel-management/connectors/legacy-main/route-policy", bytes.NewBufferString(`{"eligibleRooms":["room_redacted"]}`))
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("route update status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("route editing is unsupported")) {
		t.Fatalf("expected unsupported capability error, body=%s", rec.Body.String())
	}
}
