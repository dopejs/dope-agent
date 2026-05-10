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

func TestConnectorAdapterSupportsMatrixBackgroundDeliveryBoundary(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &kindedFakeConnectorSender{fakeConnectorSender: &fakeConnectorSender{externalID: "matrix_reply_1"}, kind: matrixConnectorKind}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.RegisterConnector("matrix-main", matrixConnectorKind, sender)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_matrix_delivery"})
	now := time.Now().UTC()
	if err := sqliteStore.SaveMatrixHostedSetup(ctx, store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_delivery",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "healthy",
		TerminalState:       "ready",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    true,
		HomeserverBindingID: "matrix_hs_delivery",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, runtime.Run{
		RunID:      "run_matrix_delivery",
		Entrypoint: "operator",
		Status:     runtime.RunStatusCompleted,
		Goal:       "matrix connector delivery",
	}); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	result, err := adapter.Send(ctx, DeliveryTarget{
		TargetID:   "matrix-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "matrix-main",
			ChannelID:   "!room:example.org",
			PeerID:      "@alice:example.org",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_matrix",
		RunID:          "run_matrix_delivery",
		PayloadPreview: "hello matrix",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(sender.replies) != 1 || sender.replies[0].ConnectorID != "matrix-main" {
		t.Fatalf("expected one Matrix outbound reply, got %+v", sender.replies)
	}
	if result.SeparationStatus != "separate_truths" || result.ConnectorDeliveryBoundaryID == "" {
		t.Fatalf("expected separate Matrix delivery truth, got %+v", result)
	}
	record, ok, err := sqliteStore.GetConnectorMessageByExternalID(ctx, "matrix-main", imtypes.DeliveryDirectionOutbound, "matrix_reply_1")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID returned ok=%v err=%v", ok, err)
	}
	if record.BackgroundDeliveryID != "delivery_matrix" || record.DeliveryBoundaryKind != "background_delivery" {
		t.Fatalf("expected Matrix background delivery boundary fields, got %+v", record)
	}
}

func TestConnectorAdapterBlocksMatrixDeliveryUntilHostedSetupIsReady(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	sender := &fakeConnectorSender{externalID: "matrix_reply_1"}
	adapter := NewConnectorAdapter(sqliteStore)
	adapter.RegisterConnector("matrix-main", matrixConnectorKind, sender)

	_, err = adapter.Send(context.Background(), DeliveryTarget{
		TargetID:   "matrix-target",
		TargetKind: TargetKindConnectorRoute,
		ConnectorBinding: &ConnectorBinding{
			ConnectorID: "matrix-main",
			ChannelID:   "!room:example.org",
		},
	}, DeliveryOutcome{
		DeliveryID:     "delivery_matrix_blocked",
		RunID:          "run_matrix_delivery",
		PayloadPreview: "hello matrix",
	})
	if err == nil {
		t.Fatal("expected Matrix delivery to be blocked before hosted setup is ready")
	}
	if len(sender.replies) != 0 {
		t.Fatalf("blocked Matrix delivery should not send replies, got %+v", sender.replies)
	}
}
