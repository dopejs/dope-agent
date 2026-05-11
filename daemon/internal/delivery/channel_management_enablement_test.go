package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestDeliveryManagerBlocksDisabledChannelConnectorTargets(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_channels"})
	manager := NewManager("test", events.NewBus(), sqliteStore, NewConnectorAdapter(sqliteStore))
	sender := &fakeConnectorSender{}
	manager.adapters[0].(*ConnectorAdapter).Register("discord-main", sender)
	if err := sqliteStore.SaveChannelConnectorEnablementState(ctx, connectors.EnablementState{
		TenantID:     "ten_channels",
		ConnectorID:  "discord-main",
		State:        "disabled",
		ReasonCode:   "maintenance",
		ChangedAt:    time.Now().UTC(),
		AuditEventID: "audit_disable",
	}); err != nil {
		t.Fatalf("SaveChannelConnectorEnablementState: %v", err)
	}
	target, err := manager.CreateTarget(ctx, DeliveryTarget{
		TargetID:    "channel-target",
		DisplayName: "Channel Target",
		TargetKind:  TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "discord-main",
			ChannelID:   "channel_redacted",
		},
	})
	if err != nil {
		t.Fatalf("CreateTarget: %v", err)
	}
	if _, err := manager.UpsertPreference(ctx, DeliveryPreference{
		PreferenceID: "pref-channel",
		ScopeKind:    PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[ResultClass]string{
			ResultClassFailure: target.TargetID,
		},
		Active: true,
	}); err != nil {
		t.Fatalf("UpsertPreference: %v", err)
	}

	outcome, err := manager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_disabled_connector",
		ResultClass:    ResultClassFailure,
		PayloadPreview: "safe preview",
	})
	if err != nil {
		t.Fatalf("EmitOutcome: %v", err)
	}
	if outcome.Status != OutcomeStatusFailed || len(outcome.Attempts) != 1 || outcome.Attempts[0].FailureClass != "connector_disabled" {
		t.Fatalf("expected connector_disabled failed outcome, got %+v", outcome)
	}
	if len(sender.replies) != 0 {
		t.Fatalf("disabled connector should not receive background replies: %+v", sender.replies)
	}
	items, err := sqliteStore.ListChannelBackgroundDeliveryOutcomes(ctx, "ten_channels", "discord-main", time.Now().UTC())
	if err != nil {
		t.Fatalf("ListChannelBackgroundDeliveryOutcomes: %v", err)
	}
	if len(items) != 1 || items[0].Status != string(OutcomeStatusFailed) || items[0].ReasonCode != "connector_disabled" {
		t.Fatalf("expected channel management delivery outcome, got %+v", items)
	}
}
