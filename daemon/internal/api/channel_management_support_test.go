package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestChannelManagementSupportEvidenceIsPermissionedAndMetadataOnly(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "telegram-main", "telegram", "Telegram Main")

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/telegram-main/support-evidence", nil)
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), channelManagementTenantContext()))
	deniedRec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, nil, sqliteStore, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("support denied status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	eventBus := events.NewBus()
	req := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/telegram-main/support-evidence", nil)
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, eventBus, sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("support status=%d body=%s", rec.Code, rec.Body.String())
	}
	var bundle connectors.SupportEvidenceBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode support evidence: %v", err)
	}
	if bundle.RedactionStatus != connectors.RedactionStatusRedacted || bundle.SupportEvidenceID == "" {
		t.Fatalf("unexpected support evidence: %+v", bundle)
	}
	lower := strings.ToLower(rec.Body.String())
	for _, forbidden := range []string{"access_token", "bearer ", "message body:", "raw payload:"} {
		if bytes.Contains([]byte(lower), []byte(forbidden)) {
			t.Fatalf("support evidence leaked %q: %s", forbidden, rec.Body.String())
		}
	}
	published := eventBus.List(events.Filter{Category: "connector"})
	if len(published) != 1 || published[0].Name != events.ConnectorEventSupportEvidenceGenerated {
		t.Fatalf("expected support evidence generated event, got %+v", published)
	}
}

func TestChannelManagementSupportEvidenceAggregatesIncidentReferences(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "matrix-main", "matrix", "Matrix Main")
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if _, err := sqliteStore.SaveChannelRepairAction(context.Background(), connectors.RepairAction{
		RepairActionID:   "repair_1",
		TenantID:         "ten_channels",
		ConnectorID:      "matrix-main",
		ConnectorKind:    "matrix",
		ActionKind:       connectors.ManagementActionRepair,
		Status:           connectors.ManagementTerminalActionRequired,
		StartedAt:        now,
		AuditEventID:     "audit_repair",
		RedactionStatus:  connectors.RedactionStatusRedacted,
		RemediationOwner: connectors.RemediationOwnerAdmin,
	}); err != nil {
		t.Fatalf("SaveChannelRepairAction: %v", err)
	}
	if _, err := sqliteStore.SaveChannelRoutingDecision(context.Background(), connectors.RoutingDecision{
		RoutingDecisionID:  "route_1",
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		ConnectorKind:      "matrix",
		Outcome:            connectors.RouteDecisionBlocked,
		ReasonCode:         "blocked_route",
		OccurredAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelRoutingDecision: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/matrix-main/support-evidence", nil)
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, events.NewBus(), sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("support status=%d body=%s", rec.Code, rec.Body.String())
	}
	var bundle connectors.SupportEvidenceBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if !containsString(bundle.RepairRefs, "repair_1") || !containsString(bundle.RoutingDecisionRefs, "route_1") {
		t.Fatalf("support evidence did not aggregate incident refs: %+v", bundle)
	}
}

func TestChannelManagementSupportEvidenceEmitsRedactionAndRetentionEvents(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	supervisor := connectors.NewSupervisor()
	registerChannelManagementTestConnector(t, supervisor, "ten_channels", "slack-main", "slack", "Slack Main")
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), connectors.ConnectorDiagnosticState{
		DiagnosticStateID:  "diagnostic_redaction_failed",
		TenantID:           "ten_channels",
		ConnectorID:        "slack-main",
		Status:             connectors.LifecycleStateFailed,
		ReasonCode:         connectors.DiagnosticReplyFailed,
		RemediationOwner:   connectors.RemediationOwnerAdmin,
		RetrySafety:        connectors.RetrySafetyRetryable,
		EvidenceTimestamp:  now,
		FreshnessState:     connectors.FreshnessFresh,
		RedactionStatus:    connectors.RedactionStatusFailed,
		RetentionExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState: %v", err)
	}
	if _, err := sqliteStore.SaveChannelSupportEvidence(context.Background(), connectors.SupportEvidenceBundle{
		SupportEvidenceID:  "support_expired_1",
		TenantID:           "ten_channels",
		ConnectorID:        "slack-main",
		GeneratedAt:        now.Add(-48 * time.Hour),
		CurrentState:       connectors.ManagementStateReady,
		RetentionExpiresAt: now.Add(-24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelSupportEvidence: %v", err)
	}

	eventBus := events.NewBus()
	req := httptest.NewRequest(http.MethodGet, "/v1/channel-management/connectors/slack-main/support-evidence", nil)
	req = req.WithContext(withTenantContext(req.Context(), channelManagementTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleChannelManagementRoutes(supervisor, eventBus, sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("support status=%d body=%s", rec.Code, rec.Body.String())
	}
	var bundle connectors.SupportEvidenceBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.RedactionStatus != connectors.RedactionStatusSuppressed || !containsString(bundle.Redactions, "diagnostic_evidence") {
		t.Fatalf("expected redaction-suppressed evidence bundle, got %+v", bundle)
	}
	eventsByName := map[string]bool{}
	for _, event := range eventBus.List(events.Filter{Category: "connector"}) {
		eventsByName[event.Name] = true
	}
	if !eventsByName[events.ConnectorEventManagementRedactionFailed] || !eventsByName[events.ConnectorEventManagementRetentionApplied] {
		t.Fatalf("expected redaction and retention events, got %+v", eventBus.List(events.Filter{Category: "connector"}))
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
