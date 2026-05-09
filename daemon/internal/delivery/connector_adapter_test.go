package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type fakeConnectorSender struct {
	externalID string
	replies    []imtypes.OutboundReply
}

func (s *fakeConnectorSender) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	s.replies = append(s.replies, reply)
	externalID := s.externalID
	if externalID == "" {
		externalID = "discord_reply_1"
	}
	return imtypes.SentReply{ExternalMessageID: externalID}, nil
}

type kindedFakeConnectorSender struct {
	*fakeConnectorSender
	kind string
}

func (s *kindedFakeConnectorSender) ConnectorKind() string {
	return s.kind
}

func TestConnectorAdapterPersistsOutboundTransportEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &fakeConnectorSender{}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.Register("discord-main", sender)
	if err := sqliteStore.UpsertRun(context.Background(), runtime.Run{
		RunID:      "run_connector",
		Entrypoint: "operator",
		Status:     runtime.RunStatusCompleted,
		Goal:       "connector delivery",
	}); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	result, err := adapter.Send(context.Background(), DeliveryTarget{
		TargetID:   "discord-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "discord-main",
			ChannelID:   "channel-1",
			PeerID:      "user-1",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_connector",
		RunID:          "run_connector",
		PayloadPreview: "hello from delivery plane",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(sender.replies) != 1 {
		t.Fatalf("expected one outbound reply, got %+v", sender.replies)
	}
	if result.ConnectorMessageDeliveryID != "delivery_connector" {
		t.Fatalf("expected connector evidence to reuse delivery id, got %+v", result)
	}
	if result.ConnectorDeliveryBoundaryID == "" || result.SeparationStatus != "separate_truths" {
		t.Fatalf("expected connector delivery boundary evidence, got %+v", result)
	}
	record, ok, err := sqliteStore.GetConnectorMessageByExternalID(context.Background(), "discord-main", imtypes.DeliveryDirectionOutbound, "discord_reply_1")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID returned ok=%v err=%v", ok, err)
	}
	if record.ResponseToDeliveryID != "delivery_connector" || record.RunID != "run_connector" {
		t.Fatalf("expected stored connector evidence to link back to delivery truth, got %+v", record)
	}
	if record.BackgroundDeliveryID != "delivery_connector" || record.DeliveryBoundaryKind != "background_delivery" {
		t.Fatalf("expected stored connector boundary fields, got %+v", record)
	}
	var boundaryCount int
	if err := sqliteStore.DB().QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM connector_delivery_boundaries
		WHERE boundary_id = ? AND connector_id = ? AND background_delivery_id = ? AND separation_status = ?
	`, result.ConnectorDeliveryBoundaryID, "discord-main", "delivery_connector", "separate_truths").Scan(&boundaryCount); err != nil {
		t.Fatalf("query connector_delivery_boundaries: %v", err)
	}
	if boundaryCount != 1 {
		t.Fatalf("expected persisted connector delivery boundary row, got %d", boundaryCount)
	}
}

func TestConnectorAdapterSupportsTelegramBackgroundDeliveryBoundary(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &kindedFakeConnectorSender{fakeConnectorSender: &fakeConnectorSender{externalID: "telegram_reply_1"}, kind: "telegram"}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.Register("telegram-main", sender)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_telegram_delivery"})
	now := time.Now().UTC()
	if err := sqliteStore.SaveTelegramHostedSetup(ctx, store.TelegramHostedSetupRecord{
		TenantID:           "ten_telegram_delivery",
		ConnectorID:        "telegram-main",
		ConnectorKind:      "telegram",
		DisplayName:        "Telegram Main",
		Status:             "healthy",
		TerminalState:      "ready",
		HostedReady:        true,
		CredentialState:    "valid",
		AllowmentState:     "valid",
		GroupBehavior:      "mention_or_command_required",
		DeliveryEligible:   true,
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveTelegramHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, runtime.Run{
		RunID:      "run_telegram_delivery",
		Entrypoint: "operator",
		Status:     runtime.RunStatusCompleted,
		Goal:       "telegram connector delivery",
	}); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	result, err := adapter.Send(ctx, DeliveryTarget{
		TargetID:   "telegram-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "telegram-main",
			ChannelID:   "telegram_chat_1",
			PeerID:      "telegram_user_1",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_telegram",
		RunID:          "run_telegram_delivery",
		PayloadPreview: "hello telegram",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(sender.replies) != 1 || sender.replies[0].ConnectorID != "telegram-main" {
		t.Fatalf("expected one Telegram outbound reply, got %+v", sender.replies)
	}
	if result.SeparationStatus != "separate_truths" || result.ConnectorDeliveryBoundaryID == "" {
		t.Fatalf("expected separate Telegram delivery truth, got %+v", result)
	}
	record, ok, err := sqliteStore.GetConnectorMessageByExternalID(ctx, "telegram-main", imtypes.DeliveryDirectionOutbound, "telegram_reply_1")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID returned ok=%v err=%v", ok, err)
	}
	if record.BackgroundDeliveryID != "delivery_telegram" || record.DeliveryBoundaryKind != "background_delivery" {
		t.Fatalf("expected Telegram background delivery boundary fields, got %+v", record)
	}
}

func TestConnectorAdapterSupportsSlackBackgroundDeliveryBoundary(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &kindedFakeConnectorSender{fakeConnectorSender: &fakeConnectorSender{externalID: "slack_reply_1"}, kind: "slack"}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.RegisterConnector("slack-main", "slack", sender)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_slack_delivery"})
	now := time.Now().UTC()
	if err := sqliteStore.SaveSlackHostedSetup(ctx, store.SlackHostedSetupRecord{
		TenantID:           "ten_slack_delivery",
		ConnectorID:        "slack-main",
		ConnectorKind:      "slack",
		DisplayName:        "Slack Main",
		Status:             "healthy",
		TerminalState:      "ready",
		OAuthState:         "grant_valid",
		RoutePolicyState:   "valid",
		DeliveryEligible:   true,
		WorkspaceBindingID: "slack_workspace_binding_delivery",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveSlackHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, runtime.Run{
		RunID:      "run_slack_delivery",
		Entrypoint: "operator",
		Status:     runtime.RunStatusCompleted,
		Goal:       "slack connector delivery",
	}); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	result, err := adapter.Send(ctx, DeliveryTarget{
		TargetID:   "slack-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "slack-main",
			ChannelID:   "slack_channel_1",
			PeerID:      "slack_user_1",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_slack",
		RunID:          "run_slack_delivery",
		PayloadPreview: "hello slack",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(sender.replies) != 1 || sender.replies[0].ConnectorID != "slack-main" {
		t.Fatalf("expected one Slack outbound reply, got %+v", sender.replies)
	}
	if result.SeparationStatus != "separate_truths" || result.ConnectorDeliveryBoundaryID == "" {
		t.Fatalf("expected separate Slack delivery truth, got %+v", result)
	}
	record, ok, err := sqliteStore.GetConnectorMessageByExternalID(ctx, "slack-main", imtypes.DeliveryDirectionOutbound, "slack_reply_1")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID returned ok=%v err=%v", ok, err)
	}
	if record.BackgroundDeliveryID != "delivery_slack" || record.DeliveryBoundaryKind != "background_delivery" {
		t.Fatalf("expected Slack background delivery boundary fields, got %+v", record)
	}
}

func TestConnectorAdapterBlocksSlackDeliveryUntilHostedSetupIsReady(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &fakeConnectorSender{externalID: "slack_reply_1"}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.RegisterConnector("slack-main", "slack", sender)

	_, err = adapter.Send(context.Background(), DeliveryTarget{
		TargetID:   "slack-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "slack-main",
			ChannelID:   "slack_channel_1",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_slack_blocked",
		RunID:          "run_slack_delivery",
		PayloadPreview: "hello slack",
	})
	if err == nil {
		t.Fatal("expected Slack delivery to be blocked before hosted setup is ready")
	}
	if len(sender.replies) != 0 {
		t.Fatalf("blocked Slack delivery should not send replies, got %+v", sender.replies)
	}
}

func TestConnectorAdapterBlocksTelegramDeliveryUntilHostedSetupIsReady(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &fakeConnectorSender{externalID: "telegram_reply_1"}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.RegisterConnector("telegram-main", "telegram", sender)

	_, err = adapter.Send(context.Background(), DeliveryTarget{
		TargetID:   "telegram-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "telegram-main",
			ChannelID:   "telegram_chat_1",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_telegram_blocked",
		RunID:          "run_telegram_delivery",
		PayloadPreview: "hello telegram",
	})
	if err == nil {
		t.Fatal("expected Telegram delivery to be blocked before hosted setup is ready")
	}
	if len(sender.replies) != 0 {
		t.Fatalf("blocked Telegram delivery should not send replies, got %+v", sender.replies)
	}
}
