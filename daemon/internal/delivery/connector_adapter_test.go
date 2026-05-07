package delivery

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type fakeConnectorSender struct {
	replies []imtypes.OutboundReply
}

func (s *fakeConnectorSender) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	s.replies = append(s.replies, reply)
	return imtypes.SentReply{ExternalMessageID: "discord_reply_1"}, nil
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
