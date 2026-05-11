package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestChannelManagementSupportEvidenceRetentionExpiresNormalInspection(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	expired := connectors.SupportEvidenceBundle{
		SupportEvidenceID:  "support_expired",
		TenantID:           "ten_channels",
		ConnectorID:        "discord-main",
		GeneratedAt:        now.Add(-100 * 24 * time.Hour),
		CurrentState:       connectors.ManagementStateReady,
		RetentionExpiresAt: now.Add(-time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}
	active := expired
	active.SupportEvidenceID = "support_active"
	active.GeneratedAt = now
	active.RetentionExpiresAt = now.Add(90 * 24 * time.Hour)
	if _, err := sqliteStore.SaveChannelSupportEvidence(ctx, expired); err != nil {
		t.Fatalf("SaveChannelSupportEvidence expired: %v", err)
	}
	if _, err := sqliteStore.SaveChannelSupportEvidence(ctx, active); err != nil {
		t.Fatalf("SaveChannelSupportEvidence active: %v", err)
	}

	got, ok, err := sqliteStore.GetLatestChannelSupportEvidence(ctx, "ten_channels", "discord-main", now)
	if err != nil || !ok {
		t.Fatalf("GetLatestChannelSupportEvidence ok=%v err=%v", ok, err)
	}
	if got.SupportEvidenceID != "support_active" {
		t.Fatalf("expected active support evidence, got %+v", got)
	}
}

func TestChannelManagementRouteReplyAndDeliveryOutcomesPersistWithRetention(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	policy := connectors.RoutePolicy{
		RoutePolicyID:              "route_policy_active",
		TenantID:                   "ten_channels",
		ConnectorID:                "matrix-main",
		EligibleRooms:              []string{"room_redacted"},
		BackgroundDeliveryEligible: false,
		ValidationState:            "valid",
		ValidatedAt:                now,
		AuditEventID:               "audit_route",
		RedactionStatus:            connectors.RedactionStatusRedacted,
	}
	if err := sqliteStore.SaveChannelRoutePolicy(ctx, policy); err != nil {
		t.Fatalf("SaveChannelRoutePolicy: %v", err)
	}
	var snapshots int
	if err := sqliteStore.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM channel_route_policy_snapshots WHERE route_policy_id = ?
	`, "route_policy_active").Scan(&snapshots); err != nil {
		t.Fatalf("query route policy snapshots: %v", err)
	}
	if snapshots != 1 {
		t.Fatalf("expected route policy snapshot, got %d", snapshots)
	}

	activeDecision, err := sqliteStore.SaveChannelRoutingDecision(ctx, connectors.RoutingDecision{
		RoutingDecisionID:  "route_active",
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
		t.Fatalf("SaveChannelRoutingDecision(active): %v", err)
	}
	if _, err := sqliteStore.SaveChannelRoutingDecision(ctx, connectors.RoutingDecision{
		RoutingDecisionID:  "route_expired",
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		ConnectorKind:      "matrix",
		Outcome:            connectors.RouteDecisionAccepted,
		OccurredAt:         now.Add(-100 * 24 * time.Hour),
		RetentionExpiresAt: now.Add(-time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelRoutingDecision(expired): %v", err)
	}
	if _, err := sqliteStore.SaveChannelForegroundReplyOutcome(ctx, connectors.ForegroundReplyOutcome{
		ReplyOutcomeID:     "reply_active",
		TenantID:           "ten_channels",
		ConnectorID:        "matrix-main",
		RoutingDecisionID:  activeDecision.RoutingDecisionID,
		Status:             "failed",
		ReasonCode:         "provider_unavailable",
		OccurredAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelForegroundReplyOutcome: %v", err)
	}
	if _, err := sqliteStore.SaveChannelBackgroundDeliveryOutcome(ctx, connectors.BackgroundDeliveryOutcome{
		DeliveryOutcomeID:  "delivery_active",
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

	decisions, err := sqliteStore.ListChannelRoutingDecisions(ctx, "ten_channels", "matrix-main", now)
	if err != nil {
		t.Fatalf("ListChannelRoutingDecisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0].RoutingDecisionID != "route_active" {
		t.Fatalf("unexpected decisions: %+v", decisions)
	}
	replies, err := sqliteStore.ListChannelForegroundReplyOutcomes(ctx, "ten_channels", "matrix-main", now)
	if err != nil {
		t.Fatalf("ListChannelForegroundReplyOutcomes: %v", err)
	}
	if len(replies) != 1 || replies[0].RoutingDecisionID != "route_active" {
		t.Fatalf("unexpected replies: %+v", replies)
	}
	deliveries, err := sqliteStore.ListChannelBackgroundDeliveryOutcomes(ctx, "ten_channels", "matrix-main", now)
	if err != nil {
		t.Fatalf("ListChannelBackgroundDeliveryOutcomes: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].DeliveryTargetID != "target_redacted" {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}
}
