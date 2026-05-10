package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type connectorReplySender interface {
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
}

type connectorKindReporter interface {
	ConnectorKind() string
}

type ConnectorAdapter struct {
	sqliteStore *store.SQLiteStore
	mu          sync.RWMutex
	senders     map[string]connectorReplySender
	kinds       map[string]string
}

func NewConnectorAdapter(sqliteStore *store.SQLiteStore) *ConnectorAdapter {
	return &ConnectorAdapter{
		sqliteStore: sqliteStore,
		senders:     map[string]connectorReplySender{},
		kinds:       map[string]string{},
	}
}

func (a *ConnectorAdapter) Register(connectorID string, sender connectorReplySender) {
	a.RegisterConnector(connectorID, "", sender)
}

func (a *ConnectorAdapter) RegisterConnector(connectorID, connectorKind string, sender connectorReplySender) {
	if a == nil || strings.TrimSpace(connectorID) == "" || sender == nil {
		return
	}
	if strings.TrimSpace(connectorKind) == "" {
		if reporter, ok := sender.(connectorKindReporter); ok {
			connectorKind = reporter.ConnectorKind()
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	connectorID = strings.TrimSpace(connectorID)
	a.senders[connectorID] = sender
	if strings.TrimSpace(connectorKind) != "" {
		a.kinds[connectorID] = strings.TrimSpace(connectorKind)
	}
}

func (a *ConnectorAdapter) Supports(kind TargetKind) bool {
	return kind == TargetKindConnectorRoute
}

func (a *ConnectorAdapter) Send(ctx context.Context, target DeliveryTarget, outcome DeliveryOutcome) (SendResult, error) {
	if a == nil {
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, fmt.Errorf("connector-backed delivery adapter is not configured")
	}
	if target.ConnectorBinding == nil || strings.TrimSpace(target.ConnectorBinding.ConnectorID) == "" {
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, fmt.Errorf("connector-backed delivery target %s is missing connector binding", target.TargetID)
	}
	if strings.TrimSpace(target.ConnectorBinding.ChannelID) == "" {
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, fmt.Errorf("connector-backed delivery target %s is missing channel id", target.TargetID)
	}
	sender, connectorKind, ok := a.senderFor(target.ConnectorBinding.ConnectorID)
	if !ok {
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, fmt.Errorf("connector %s is not available for delivery", target.ConnectorBinding.ConnectorID)
	}
	if err := a.requireConnectorDeliveryReady(ctx, connectorKind, target.ConnectorBinding.ConnectorID); err != nil {
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, err
	}

	record := imtypes.MessageRecord{
		DeliveryID:           outcome.DeliveryID,
		ConnectorID:          target.ConnectorBinding.ConnectorID,
		Direction:            imtypes.DeliveryDirectionOutbound,
		RunID:                outcome.RunID,
		ChannelID:            target.ConnectorBinding.ChannelID,
		PeerID:               target.ConnectorBinding.PeerID,
		ThreadID:             target.ConnectorBinding.ThreadID,
		Content:              outcome.PayloadPreview,
		Status:               imtypes.DeliveryStatusProcessing,
		ResponseToDeliveryID: outcome.DeliveryID,
		BackgroundDeliveryID: outcome.DeliveryID,
		DeliveryBoundaryKind: "background_delivery",
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	if a.sqliteStore != nil {
		if err := a.sqliteStore.UpsertConnectorMessage(ctx, record); err != nil {
			return SendResult{TransportKind: string(TargetKindConnectorRoute)}, err
		}
	}

	sent, err := sender.SendReply(ctx, imtypes.OutboundReply{
		ConnectorID: target.ConnectorBinding.ConnectorID,
		ChannelID:   target.ConnectorBinding.ChannelID,
		Content:     outcome.PayloadPreview,
	})
	record.UpdatedAt = time.Now().UTC()
	if err != nil {
		record.Status = imtypes.DeliveryStatusFailed
		record.Error = err.Error()
		if a.sqliteStore != nil {
			_ = a.sqliteStore.UpsertConnectorMessage(ctx, record)
		}
		return SendResult{TransportKind: string(TargetKindConnectorRoute)}, err
	}
	record.ExternalMessageID = sent.ExternalMessageID
	record.Status = imtypes.DeliveryStatusReplied
	if a.sqliteStore != nil {
		if err := a.sqliteStore.UpsertConnectorMessage(ctx, record); err != nil {
			return SendResult{TransportKind: string(TargetKindConnectorRoute)}, err
		}
		boundaryID := "boundary_" + record.DeliveryID
		boundary := store.ConnectorDeliveryBoundaryRecord{
			BoundaryID:               boundaryID,
			ConnectorID:              record.ConnectorID,
			ForegroundReplyOutcomeID: outcome.DeliveryID,
			BackgroundDeliveryID:     record.DeliveryID,
			TransportKind:            string(TargetKindConnectorRoute),
			SeparationStatus:         "separate_truths",
			CreatedAt:                record.UpdatedAt,
		}
		if document, err := json.Marshal(map[string]any{
			"boundaryId":               boundary.BoundaryID,
			"connectorId":              boundary.ConnectorID,
			"foregroundReplyOutcomeId": boundary.ForegroundReplyOutcomeID,
			"backgroundDeliveryId":     boundary.BackgroundDeliveryID,
			"transportKind":            boundary.TransportKind,
			"separationStatus":         boundary.SeparationStatus,
			"connectorMessageId":       record.DeliveryID,
			"externalMessageId":        record.ExternalMessageID,
		}); err == nil {
			boundary.Document = document
		}
		if err := a.sqliteStore.SaveConnectorDeliveryBoundary(ctx, boundary); err != nil {
			return SendResult{TransportKind: string(TargetKindConnectorRoute)}, err
		}
	}
	return SendResult{
		TransportKind:               string(TargetKindConnectorRoute),
		ReceiptSummary:              "connector reply persisted",
		ConnectorMessageDeliveryID:  record.DeliveryID,
		ConnectorDeliveryBoundaryID: "boundary_" + record.DeliveryID,
		SeparationStatus:            "separate_truths",
	}, nil
}

func (a *ConnectorAdapter) senderFor(connectorID string) (connectorReplySender, string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	connectorID = strings.TrimSpace(connectorID)
	sender, ok := a.senders[connectorID]
	return sender, a.kinds[connectorID], ok
}

func (a *ConnectorAdapter) requireConnectorDeliveryReady(ctx context.Context, connectorKind, connectorID string) error {
	if a == nil || a.sqliteStore == nil {
		return nil
	}
	connectorKind = strings.TrimSpace(connectorKind)
	if connectorKind != "telegram" && connectorKind != "slack" && connectorKind != matrixConnectorKind {
		return nil
	}
	tenantID := ""
	if binding, ok := a.sqliteStore.ResolveActiveTenantBinding(ctx).(string); ok {
		tenantID = binding
	}
	if strings.TrimSpace(tenantID) == "" {
		if defaultTenant, err := a.sqliteStore.ResolveDefaultPersonalTenantID(ctx); err == nil {
			tenantID = defaultTenant
		}
	}
	if connectorKind == "slack" {
		setup, ok, err := a.sqliteStore.GetSlackHostedSetup(ctx, tenantID, connectorID)
		if err != nil {
			return err
		}
		if !ok || setup.TerminalState != "ready" || !setup.DeliveryEligible {
			return fmt.Errorf("slack connector %s is not delivery eligible", connectorID)
		}
		return nil
	}
	if connectorKind == matrixConnectorKind {
		setup, ok, err := a.sqliteStore.GetMatrixHostedSetup(ctx, tenantID, connectorID)
		if err != nil {
			return err
		}
		if !ok || setup.TerminalState != "ready" || !setup.DeliveryEligible {
			return fmt.Errorf("matrix connector %s is not delivery eligible", connectorID)
		}
		return nil
	}
	setup, ok, err := a.sqliteStore.GetTelegramHostedSetup(ctx, tenantID, connectorID)
	if err != nil {
		return err
	}
	if !ok || !setup.HostedReady || !setup.DeliveryEligible {
		return fmt.Errorf("telegram connector %s is not delivery eligible", connectorID)
	}
	return nil
}
