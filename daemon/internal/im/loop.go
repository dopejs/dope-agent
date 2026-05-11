package im

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type ReplySender interface {
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
}

type ReplyProgressor interface {
	ReplySender
	ReplyCapabilities() imtypes.ReplyCapabilities
	SendThinking(ctx context.Context, signal imtypes.ThinkingSignal) error
	EditReply(ctx context.Context, edit imtypes.ReplyEdit) error
}

type classifiedError interface {
	ErrorClass() string
}

type MessageLoop struct {
	router      *router.SessionRouter
	runtime     *runtime.Manager
	checkpoints *checkpoints.Manager
	eventBus    *events.Bus
	store       *store.SQLiteStore
	chat        *chat.Service
}

type ProcessResult struct {
	Session    router.Session
	Run        runtime.Run
	Step       runtime.Step
	Reply      string
	Outcome    string
	ReasonCode string
	Duplicate  bool
}

func NewMessageLoop(sessionRouter *router.SessionRouter, runtimeManager *runtime.Manager, checkpointManager *checkpoints.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, chatService *chat.Service) *MessageLoop {
	return &MessageLoop{
		router:      sessionRouter,
		runtime:     runtimeManager,
		checkpoints: checkpointManager,
		eventBus:    eventBus,
		store:       sqliteStore,
		chat:        chatService,
	}
}

func (l *MessageLoop) ProcessSingleTurn(ctx context.Context, connector connectors.Connector, inbound imtypes.InboundMessage, replies ReplySender) (ProcessResult, error) {
	if l == nil || l.router == nil || l.runtime == nil || l.chat == nil {
		return ProcessResult{}, fmt.Errorf("connector message loop is not configured")
	}
	if strings.TrimSpace(inbound.ExternalMessageID) == "" {
		return ProcessResult{}, fmt.Errorf("external message id is required")
	}
	if strings.TrimSpace(inbound.Content) == "" {
		return ProcessResult{}, fmt.Errorf("content is required")
	}

	now := inbound.ReceivedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	inboundRecord := imtypes.MessageRecord{
		DeliveryID:               newDeliveryID(),
		TenantID:                 inbound.TenantID,
		ConnectorID:              connector.ConnectorID,
		Direction:                imtypes.DeliveryDirectionInbound,
		ExternalMessageID:        inbound.ExternalMessageID,
		ConnectorAccountID:       inboundConnectorAccountID(inbound),
		ChannelOrConversationID:  inboundChannelOrConversationID(inbound),
		ProviderMessageID:        inboundProviderMessageID(inbound),
		EquivalentRuleID:         inbound.EquivalentRuleID,
		ChannelID:                inbound.ChannelID,
		PeerID:                   inbound.PeerID,
		ThreadID:                 inbound.ThreadID,
		AuthorID:                 inbound.AuthorID,
		Content:                  inbound.Content,
		Status:                   imtypes.DeliveryStatusReceived,
		ReplyToExternalMessageID: inbound.ReplyToMessageID,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	persistedInbound, created, err := l.store.CreateConnectorMessageIfAbsent(ctx, inboundRecord)
	if err != nil {
		return ProcessResult{}, err
	}
	if !created {
		_ = l.recordDuplicateThreadEvidence(ctx, connector, inbound, persistedInbound)
		_, _ = l.publishConnectorEvent(ctx, events.ConnectorEventInboundDuplicateDetected, connector, router.Session{}, "", "", map[string]any{
			"tenantId":                persistedInbound.TenantID,
			"connectorId":             connector.ConnectorID,
			"connectorAccountId":      persistedInbound.ConnectorAccountID,
			"channelOrConversationId": persistedInbound.ChannelOrConversationID,
			"providerMessageId":       persistedInbound.ProviderMessageID,
			"equivalentRuleId":        persistedInbound.EquivalentRuleID,
			"existingDeliveryId":      persistedInbound.DeliveryID,
			"redactionStatus":         "redacted",
		})
		_ = l.publishMatrixRouteOutcome(ctx, connector, router.Session{}, persistedInbound, "duplicate", "duplicate_inbound")
		return ProcessResult{Outcome: "duplicate", ReasonCode: "duplicate_inbound", Duplicate: true}, nil
	}

	if blocked, err := l.blockArchivedSourceContinuation(ctx, connector, inbound, &persistedInbound); err != nil {
		return ProcessResult{}, err
	} else if blocked {
		return ProcessResult{Outcome: "blocked", ReasonCode: "thread_archived"}, nil
	}

	session, createdSession, err := l.routeSession(inbound)
	if err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		_ = l.recordRoutingOnlySourceEvidence(ctx, connector, inbound, persistedInbound, classifyRoutingOutcome(err), classifyError(err))
		return ProcessResult{}, err
	}
	if err := l.persistSession(ctx, session); err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{}, err
	}
	thread, segmentID, err := l.ensureThreadLifecycleForInbound(ctx, connector, inbound, session, persistedInbound)
	if err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{}, err
	}
	persistedInbound.SessionID = session.SessionID
	if thread.ThreadID != "" {
		persistedInbound.ThreadID = thread.ThreadID
		persistedInbound.ThreadSessionSegmentID = segmentID
	}
	persistedInbound.UpdatedAt = time.Now().UTC()
	if err := l.store.UpsertConnectorMessage(ctx, persistedInbound); err != nil {
		return ProcessResult{}, err
	}
	if err := l.publishSessionRouteEvents(ctx, connector, session, createdSession, inbound); err != nil {
		return ProcessResult{}, err
	}
	if err := l.publishMatrixRouteOutcome(ctx, connector, session, persistedInbound, "accepted", "accepted"); err != nil {
		return ProcessResult{}, err
	}
	if _, err := l.publishConnectorEvent(ctx, "connector.ingress_accepted", connector, session, "", "", map[string]any{
		"messageId": inbound.ExternalMessageID,
		"channelId": inbound.ChannelID,
		"authorId":  inbound.AuthorID,
		"direct":    inbound.Direct,
	}); err != nil {
		return ProcessResult{}, err
	}

	run, step, err := l.createRunAndStep(ctx, connector, session, inbound)
	if err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		persistedInbound.RunID = run.RunID
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{}, err
	}
	persistedInbound.RunID = run.RunID
	persistedInbound.Status = imtypes.DeliveryStatusProcessing
	persistedInbound.UpdatedAt = time.Now().UTC()
	if err := l.store.UpsertConnectorMessage(ctx, persistedInbound); err != nil {
		return ProcessResult{}, err
	}

	if _, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusPlanning}); err != nil {
		return ProcessResult{}, err
	}

	if _, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusCallingModel}); err != nil {
		return ProcessResult{}, err
	}

	scope := events.Scope{
		SessionID:   session.SessionID,
		RunID:       run.RunID,
		StepID:      step.StepID,
		ConnectorID: connector.ConnectorID,
	}
	progressor, _ := replies.(ReplyProgressor)
	capabilities := imtypes.ReplyCapabilities{}
	if progressor != nil {
		capabilities = progressor.ReplyCapabilities()
	}
	stopThinking := l.startThinkingProgress(ctx, connector, session, run.RunID, step.StepID, inbound, progressor, capabilities)
	defer stopThinking()

	queryResult, outboundRecord, sendErr := l.executeReplyPath(ctx, connector, session, run, step, inbound, persistedInbound, replies, progressor, capabilities, scope, stopThinking)
	if sendErr != nil {
		stopThinking()
		partialReply := queryResult.Dispatch.Partial || outboundRecord.Status == imtypes.DeliveryStatusPartial
		safeReason := safeReplyFailureReason(sendErr)
		errorClass := classifyError(sendErr)
		if outboundRecord.DeliveryID != "" && !partialReply {
			outboundRecord.Status = imtypes.DeliveryStatusFailed
			outboundRecord.Error = safeReason
			outboundRecord.UpdatedAt = time.Now().UTC()
			_ = l.store.UpsertConnectorMessage(ctx, outboundRecord)
		}
		_, _ = l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"reply":       queryResult.Dispatch.Output,
				"partial":     partialReply,
				"replyStatus": outboundRecord.Status,
				"reasonCode":  safeReason,
				"errorClass":  errorClass,
			},
		})
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		if partialReply {
			persistedInbound.Status = imtypes.DeliveryStatusPartial
		}
		persistedInbound.Error = safeReason
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		_ = l.recordThreadRuntimeProjections(ctx, thread, segmentID, session, run, persistedInbound, outboundRecord, string(persistedInbound.Status), safeReason)
		if !partialReply {
			_ = l.recordChannelForegroundReplyOutcome(ctx, connector, session, outboundRecord, "failed", safeReason, map[string]string{
				"errorClass": errorClass,
				"messageId":  inbound.ExternalMessageID,
			})
			payload := map[string]any{
				"messageId":                 inbound.ExternalMessageID,
				"replyMessageId":            outboundRecord.ExternalMessageID,
				"assistantExecutionOutcome": "succeeded",
				"connectorDeliveryOutcome":  "failed",
				"connectorKind":             connector.Kind,
				"reasonCode":                safeReason,
				"errorClass":                errorClass,
				"redactionStatus":           "redacted",
			}
			if connector.Kind == "discord" {
				payload["discordDeliveryOutcome"] = "failed"
			}
			_, _ = l.publishConnectorEvent(ctx, "connector.reply_failed", connector, session, run.RunID, step.StepID, payload)
		}
		if refreshedRun, ok := l.runtime.GetRun(run.RunID); ok {
			run = refreshedRun
		}
		return ProcessResult{Session: session, Run: run, Step: step, Reply: queryResult.Dispatch.Output}, sendErr
	}

	stopThinking()

	finalStep, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCompleted,
		Output: map[string]any{
			"reply":                    queryResult.Dispatch.Output,
			"replyMessageId":           outboundRecord.ExternalMessageID,
			"llmDispatchId":            queryResult.Dispatch.DispatchID,
			"llmProvider":              queryResult.Dispatch.Provider,
			"llmModel":                 queryResult.Dispatch.Model,
			"llmUsage":                 queryResult.Dispatch.Usage,
			"replyToExternalMessageId": replyToExternalMessageID(inbound),
		},
	})
	if err != nil {
		return ProcessResult{}, err
	}
	run, _ = l.runtime.GetRun(run.RunID)
	if err := l.recordThreadRuntimeProjections(ctx, thread, segmentID, session, run, persistedInbound, outboundRecord, string(run.Status), "accepted"); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Session:    session,
		Run:        run,
		Step:       finalStep,
		Reply:      queryResult.Dispatch.Output,
		Outcome:    "accepted",
		ReasonCode: "accepted",
	}, nil
}

func inboundConnectorAccountID(inbound imtypes.InboundMessage) string {
	return coalesceTrimmed(inbound.ConnectorAccountID, inbound.AccountID)
}

func inboundChannelOrConversationID(inbound imtypes.InboundMessage) string {
	return coalesceTrimmed(inbound.ChannelOrConversationID, inbound.ChannelID, inbound.PeerID)
}

func inboundProviderMessageID(inbound imtypes.InboundMessage) string {
	return coalesceTrimmed(inbound.ProviderMessageID, inbound.ExternalMessageID)
}

func coalesceTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (l *MessageLoop) blockArchivedSourceContinuation(ctx context.Context, connector connectors.Connector, inbound imtypes.InboundMessage, persistedInbound *imtypes.MessageRecord) (bool, error) {
	key, err := sourceContinuationKey(connector, inbound)
	if err != nil {
		return false, nil
	}
	current, found, err := l.store.GetCurrentThreadForSource(ctx, key)
	if err != nil || !found || current.LifecycleState != threads.LifecycleStateArchived {
		return false, err
	}
	now := time.Now().UTC()
	persistedInbound.ThreadID = current.ThreadID
	persistedInbound.ThreadSessionSegmentID = current.CurrentSessionSegmentID
	persistedInbound.Status = imtypes.DeliveryStatusFailed
	persistedInbound.Error = "thread_archived"
	persistedInbound.UpdatedAt = now
	if err := l.store.UpsertConnectorMessage(ctx, *persistedInbound); err != nil {
		return false, err
	}
	if err := l.saveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      threadSourceLinkageID(*persistedInbound, threads.RoutingOutcomeBlocked),
		ThreadID:             current.ThreadID,
		TenantID:             current.TenantID,
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          connector.ConnectorID,
		ConnectorKind:        connector.Kind,
		SourceAccountID:      key.SourceAccountID,
		SourceConversationID: key.SourceConversationID,
		SourceMessageID:      inboundProviderMessageID(inbound),
		RoutingOutcome:       threads.RoutingOutcomeBlocked,
		Current:              false,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (l *MessageLoop) ensureThreadLifecycleForInbound(ctx context.Context, connector connectors.Connector, inbound imtypes.InboundMessage, session router.Session, persistedInbound imtypes.MessageRecord) (threads.Thread, string, error) {
	key, err := sourceContinuationKey(connector, inbound)
	if err != nil {
		return threads.Thread{}, "", nil
	}
	now := time.Now().UTC()
	current, found, err := l.store.GetCurrentThreadForSource(ctx, key)
	if err != nil {
		return threads.Thread{}, "", err
	}
	segmentID := ""
	if found {
		segmentID = current.CurrentSessionSegmentID
		if segmentID == "" {
			segmentID = "seg_" + session.SessionID
			current.CurrentSessionSegmentID = segmentID
		}
		current.LastActivityAt = now
		current.UpdatedAt = now
		if err := l.store.UpsertThread(ctx, current); err != nil {
			return threads.Thread{}, "", err
		}
	} else {
		segmentID = "seg_" + session.SessionID
		current = threads.Thread{
			ThreadID:                threadIDForSource(key),
			TenantID:                key.TenantID,
			LifecycleState:          threads.LifecycleStateActive,
			CurrentSessionSegmentID: segmentID,
			SourceKind:              threads.SourceKindChannel,
			SourceSummary:           connector.DisplayName + " / " + inboundChannelOrConversationID(inbound),
			LastActivityAt:          now,
			CreatedAt:               now,
			UpdatedAt:               now,
			RetentionExpiresAt:      l.store.ThreadRetentionExpiry(ctx, key.TenantID, now),
			RedactionStatus:         threads.RedactionStatusRedacted,
		}
		if err := l.store.UpsertThread(ctx, current); err != nil {
			return threads.Thread{}, "", err
		}
	}
	if err := l.store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{
		SessionSegmentID: segmentID,
		ThreadID:         current.ThreadID,
		TenantID:         current.TenantID,
		SessionID:        session.SessionID,
		Generation:       session.Generation,
		State:            "active",
		StartedAt:        session.CreatedAt,
		LastActiveAt:     now,
		PartialEvidence:  false,
	}); err != nil {
		return threads.Thread{}, "", err
	}
	if err := l.saveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      threadSourceLinkageID(persistedInbound, threads.RoutingOutcomeAccepted),
		ThreadID:             current.ThreadID,
		TenantID:             current.TenantID,
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          connector.ConnectorID,
		ConnectorKind:        connector.Kind,
		SourceAccountID:      key.SourceAccountID,
		SourceConversationID: key.SourceConversationID,
		SourceMessageID:      inboundProviderMessageID(inbound),
		RoutingOutcome:       threads.RoutingOutcomeAccepted,
		Current:              true,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	}); err != nil {
		return threads.Thread{}, "", err
	}
	return current, segmentID, nil
}

func (l *MessageLoop) recordDuplicateThreadEvidence(ctx context.Context, connector connectors.Connector, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord) error {
	key, err := sourceContinuationKey(connector, inbound)
	if err != nil {
		return l.recordRoutingOnlySourceEvidence(ctx, connector, inbound, persistedInbound, threads.RoutingOutcomeUnknownSource, "invalid_source_key")
	}
	current, found, err := l.store.GetCurrentThreadForSource(ctx, key)
	if err != nil || !found {
		return err
	}
	return l.saveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      threadSourceLinkageID(persistedInbound, threads.RoutingOutcomeDuplicate),
		ThreadID:             current.ThreadID,
		TenantID:             current.TenantID,
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          connector.ConnectorID,
		ConnectorKind:        connector.Kind,
		SourceAccountID:      key.SourceAccountID,
		SourceConversationID: key.SourceConversationID,
		SourceMessageID:      inboundProviderMessageID(inbound),
		RoutingOutcome:       threads.RoutingOutcomeDuplicate,
		Current:              false,
		LinkedAt:             time.Now().UTC(),
		RedactionStatus:      threads.RedactionStatusRedacted,
	})
}

func (l *MessageLoop) recordRoutingOnlySourceEvidence(ctx context.Context, connector connectors.Connector, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, outcome threads.RoutingOutcome, reasonCode string) error {
	tenantID := coalesceTrimmed(inbound.TenantID, connector.TenantID)
	if tenantID == "" {
		return nil
	}
	now := time.Now().UTC()
	thread := threads.Thread{
		ThreadID:           "thr_ingress_" + shortThreadHash(persistedInbound.DeliveryID+string(outcome)),
		TenantID:           tenantID,
		LifecycleState:     threads.LifecycleStateActive,
		SourceKind:         threads.SourceKindChannel,
		SourceSummary:      connector.DisplayName + " / routing evidence",
		LastActivityAt:     now,
		CreatedAt:          now,
		UpdatedAt:          now,
		RetentionExpiresAt: l.store.ThreadRetentionExpiry(ctx, tenantID, now),
		RedactionStatus:    threads.RedactionStatusRedacted,
	}
	if err := l.store.UpsertThread(ctx, thread); err != nil {
		return err
	}
	return l.saveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      threadSourceLinkageID(persistedInbound, outcome),
		ThreadID:             thread.ThreadID,
		TenantID:             tenantID,
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          connector.ConnectorID,
		ConnectorKind:        connector.Kind,
		SourceAccountID:      inboundConnectorAccountID(inbound),
		SourceConversationID: inboundChannelOrConversationID(inbound),
		SourceMessageID:      inboundProviderMessageID(inbound),
		RoutingOutcome:       outcome,
		Current:              false,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	})
}

func (l *MessageLoop) recordThreadRuntimeProjections(ctx context.Context, thread threads.Thread, segmentID string, session router.Session, run runtime.Run, inboundRecord imtypes.MessageRecord, outboundRecord imtypes.MessageRecord, status, reasonCode string) error {
	if thread.ThreadID == "" {
		return nil
	}
	now := time.Now().UTC()
	projections := []threads.RuntimeProjection{
		threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
			ProjectionID:     "rtp_session_" + session.SessionID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionSegmentID: segmentID,
			ResourceKind:     threads.RuntimeResourceSession,
			ResourceID:       session.SessionID,
			Status:           string(session.Status),
			ReasonCode:       reasonCode,
			OccurredAt:       now,
			Route:            "/v1/sessions/" + session.SessionID,
			SafeSummary:      "Session routed",
		}),
		threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
			ProjectionID:     "rtp_connector_message_" + inboundRecord.DeliveryID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionSegmentID: segmentID,
			ResourceKind:     threads.RuntimeResourceConnectorMessage,
			ResourceID:       inboundRecord.DeliveryID,
			Status:           string(inboundRecord.Status),
			ReasonCode:       reasonCode,
			OccurredAt:       inboundRecord.CreatedAt,
			SafeSummary:      "Inbound connector message " + status,
		}),
	}
	if run.RunID != "" {
		projections = append(projections, threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
			ProjectionID:     "rtp_run_" + run.RunID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionSegmentID: segmentID,
			ResourceKind:     threads.RuntimeResourceRun,
			ResourceID:       run.RunID,
			Status:           string(run.Status),
			ReasonCode:       reasonCode,
			OccurredAt:       run.CreatedAt,
			Route:            "/v1/runs/" + run.RunID,
			SafeSummary:      "Assistant run " + string(run.Status),
		}))
	}
	if outboundRecord.DeliveryID != "" {
		projections = append(projections, threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
			ProjectionID:     "rtp_foreground_reply_" + outboundRecord.DeliveryID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionSegmentID: segmentID,
			ResourceKind:     threads.RuntimeResourceForegroundReply,
			ResourceID:       outboundRecord.DeliveryID,
			Status:           string(outboundRecord.Status),
			ReasonCode:       reasonCode,
			OccurredAt:       outboundRecord.CreatedAt,
			SafeSummary:      "Foreground reply " + string(outboundRecord.Status),
		}))
	}
	for _, projection := range projections {
		if err := l.saveThreadRuntimeProjection(ctx, projection); err != nil {
			return err
		}
	}
	return nil
}

func (l *MessageLoop) saveThreadSourceLinkage(ctx context.Context, linkage threads.SourceLinkage) error {
	if err := l.store.SaveThreadSourceLinkage(ctx, linkage); err != nil {
		return err
	}
	if l.eventBus != nil {
		l.eventBus.Publish(events.ThreadSourceLinkedEvent(linkage))
	}
	return nil
}

func (l *MessageLoop) saveThreadRuntimeProjection(ctx context.Context, projection threads.RuntimeProjection) error {
	if err := l.store.SaveThreadRuntimeProjection(ctx, projection); err != nil {
		return err
	}
	if l.eventBus != nil {
		l.eventBus.Publish(events.ThreadRuntimeProjectionEvent(projection))
	}
	return nil
}

func sourceContinuationKey(connector connectors.Connector, inbound imtypes.InboundMessage) (threads.SourceContinuationKey, error) {
	return threads.NormalizeSourceContinuationKey(threads.SourceContinuationKey{
		TenantID:             coalesceTrimmed(inbound.TenantID, connector.TenantID),
		ConnectorID:          coalesceTrimmed(connector.ConnectorID, inbound.ConnectorID),
		SourceAccountID:      inboundConnectorAccountID(inbound),
		SourceConversationID: inboundChannelOrConversationID(inbound),
	})
}

func threadIDForSource(key threads.SourceContinuationKey) string {
	return "thr_src_" + shortThreadHash(key.String())
}

func threadSourceLinkageID(record imtypes.MessageRecord, outcome threads.RoutingOutcome) string {
	return "src_" + shortThreadHash(record.DeliveryID+":"+string(outcome))
}

func shortThreadHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:24]
}

func (l *MessageLoop) executeReplyPath(ctx context.Context, connector connectors.Connector, session router.Session, run runtime.Run, step runtime.Step, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, replies ReplySender, progressor ReplyProgressor, capabilities imtypes.ReplyCapabilities, scope events.Scope, stopThinking func()) (chat.QueryResult, imtypes.MessageRecord, error) {
	if capabilities.SupportsStreaming && progressor != nil {
		return l.executeStreamingReply(ctx, connector, session, run, step, inbound, persistedInbound, progressor, capabilities, scope, stopThinking)
	}
	return l.executeFinalReply(ctx, connector, session, run, step, inbound, persistedInbound, replies, capabilities, scope, stopThinking)
}

func (l *MessageLoop) executeFinalReply(ctx context.Context, connector connectors.Connector, session router.Session, run runtime.Run, step runtime.Step, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, replies ReplySender, capabilities imtypes.ReplyCapabilities, scope events.Scope, stopThinking func()) (chat.QueryResult, imtypes.MessageRecord, error) {
	queryResult, queryErr := l.chat.Query(ctx, chat.QueryInput{
		Query:           inbound.Content,
		TenantID:        persistedInbound.TenantID,
		ThreadID:        persistedInbound.ThreadID,
		Scope:           scope,
		SourceKind:      threads.SourceKindChannel,
		SourceLinkageID: threadSourceLinkageID(persistedInbound, threads.RoutingOutcomeAccepted),
		SourceMessageID: inboundProviderMessageID(inbound),
		SourceTimestamp: &persistedInbound.CreatedAt,
		SourceEventKey:  "connector:" + persistedInbound.DeliveryID,
	})
	if queryErr != nil {
		return queryResult, imtypes.MessageRecord{}, queryErr
	}

	if _, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
		return chat.QueryResult{}, imtypes.MessageRecord{}, err
	}

	replyParts := splitReplyContent(queryResult.Dispatch.Output, capabilities.MaxMessageLength)
	if len(replyParts) == 0 {
		replyParts = []string{queryResult.Dispatch.Output}
	}

	var (
		outboundRecord  imtypes.MessageRecord
		replyMessageIDs []string
	)
	for index, replyPart := range replyParts {
		record := l.newOutboundRecord(connector, session, run, inbound, persistedInbound, replyPart)
		if err := l.store.UpsertConnectorMessage(ctx, record); err != nil {
			return chat.QueryResult{}, imtypes.MessageRecord{}, err
		}

		sentReply, sendErr := replies.SendReply(ctx, imtypes.OutboundReply{
			ConnectorID:              connector.ConnectorID,
			ChannelID:                inbound.ChannelID,
			Content:                  replyPart,
			ReplyToExternalMessageID: replyToExternalMessageID(inbound),
		})
		if sendErr != nil {
			return queryResult, record, sendErr
		}

		if stopThinking != nil {
			stopThinking()
			stopThinking = nil
		}

		record.ExternalMessageID = sentReply.ExternalMessageID
		record.Status = imtypes.DeliveryStatusReplied
		record.ForegroundOutcomeStatus = foregroundReplyOutcomeStatus(record.Status)
		record.UpdatedAt = time.Now().UTC()
		if err := l.store.UpsertConnectorMessage(ctx, record); err != nil {
			return chat.QueryResult{}, imtypes.MessageRecord{}, err
		}
		if index == 0 {
			outboundRecord = record
		}
		replyMessageIDs = append(replyMessageIDs, sentReply.ExternalMessageID)
	}

	_, _ = l.publishConnectorEvent(ctx, "connector.reply_sent", connector, session, run.RunID, step.StepID, map[string]any{
		"messageId":       inbound.ExternalMessageID,
		"replyMessageId":  outboundRecord.ExternalMessageID,
		"replyMessageIds": replyMessageIDs,
		"partCount":       len(replyMessageIDs),
	})
	_ = l.recordChannelForegroundReplyOutcome(ctx, connector, session, outboundRecord, "sent", "reply_sent", map[string]string{
		"messageId":       inbound.ExternalMessageID,
		"replyMessageId":  outboundRecord.ExternalMessageID,
		"replyMessageIds": strings.Join(replyMessageIDs, ","),
	})
	return queryResult, outboundRecord, nil
}

func (l *MessageLoop) executeStreamingReply(ctx context.Context, connector connectors.Connector, session router.Session, run runtime.Run, step runtime.Step, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, progressor ReplyProgressor, capabilities imtypes.ReplyCapabilities, scope events.Scope, stopThinking func()) (chat.QueryResult, imtypes.MessageRecord, error) {
	progress := streamReplyProgress{
		loop:           l,
		progressor:     progressor,
		connector:      connector,
		session:        session,
		runID:          run.RunID,
		stepID:         step.StepID,
		inbound:        inbound,
		responseTo:     persistedInbound,
		flushInterval:  500 * time.Millisecond,
		maxReplyLength: capabilities.MaxMessageLength,
		stopThinking:   stopThinking,
	}

	var progressErr error
	queryResult, queryErr := l.chat.Stream(ctx, chat.QueryInput{
		Query:           inbound.Content,
		TenantID:        persistedInbound.TenantID,
		ThreadID:        persistedInbound.ThreadID,
		Scope:           scope,
		SourceKind:      threads.SourceKindChannel,
		SourceLinkageID: threadSourceLinkageID(persistedInbound, threads.RoutingOutcomeAccepted),
		SourceMessageID: inboundProviderMessageID(inbound),
		SourceTimestamp: &persistedInbound.CreatedAt,
		SourceEventKey:  "connector:" + persistedInbound.DeliveryID,
	}, func(chunk chat.StreamChunk) error {
		if progressErr != nil {
			return nil
		}
		if err := progress.OnChunk(ctx, chunk.Reply); err != nil {
			progressErr = err
		}
		return nil
	})
	if queryErr != nil {
		if queryResult.Dispatch.Partial && strings.TrimSpace(queryResult.Dispatch.Output) != "" {
			if err := progress.CompletePartial(ctx, queryResult.Dispatch.Output, queryErr); err != nil {
				return queryResult, progress.record, err
			}
		}
		return queryResult, progress.record, queryErr
	}

	if _, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
		return chat.QueryResult{}, imtypes.MessageRecord{}, err
	}
	if err := progress.Complete(ctx, queryResult.Dispatch.Output); err != nil {
		return queryResult, progress.record, err
	}
	if progressErr != nil {
		return queryResult, progress.record, progressErr
	}
	return queryResult, progress.record, nil
}

func (l *MessageLoop) newOutboundRecord(connector connectors.Connector, session router.Session, run runtime.Run, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, content string) imtypes.MessageRecord {
	now := time.Now().UTC()
	return imtypes.MessageRecord{
		DeliveryID:               newDeliveryID(),
		TenantID:                 persistedInbound.TenantID,
		ConnectorID:              connector.ConnectorID,
		Direction:                imtypes.DeliveryDirectionOutbound,
		SessionID:                session.SessionID,
		RunID:                    run.RunID,
		ChannelID:                inbound.ChannelID,
		PeerID:                   inbound.PeerID,
		ThreadID:                 persistedInbound.ThreadID,
		ThreadSessionSegmentID:   persistedInbound.ThreadSessionSegmentID,
		Content:                  content,
		Status:                   imtypes.DeliveryStatusProcessing,
		ForegroundOutcomeStatus:  foregroundReplyOutcomeStatus(imtypes.DeliveryStatusProcessing),
		ResponseToDeliveryID:     persistedInbound.DeliveryID,
		ReplyToExternalMessageID: replyToExternalMessageID(inbound),
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

func foregroundReplyOutcomeStatus(status imtypes.DeliveryStatus) string {
	switch status {
	case imtypes.DeliveryStatusReplied:
		return "sent"
	case imtypes.DeliveryStatusPartial:
		return "partial"
	case imtypes.DeliveryStatusFailed:
		return "failed"
	default:
		return "processing"
	}
}

func replyToExternalMessageID(inbound imtypes.InboundMessage) string {
	if strings.TrimSpace(inbound.ReplyToMessageID) != "" {
		return strings.TrimSpace(inbound.ReplyToMessageID)
	}
	return inbound.ExternalMessageID
}

func (l *MessageLoop) startThinkingProgress(ctx context.Context, connector connectors.Connector, session router.Session, runID, stepID string, inbound imtypes.InboundMessage, progressor ReplyProgressor, capabilities imtypes.ReplyCapabilities) func() {
	if progressor == nil || !capabilities.SupportsThinking {
		return func() {}
	}

	thinkingPayload := map[string]any{
		"messageId": inbound.ExternalMessageID,
		"channelId": inbound.ChannelID,
	}
	signal := imtypes.ThinkingSignal{
		ConnectorID: connector.ConnectorID,
		ChannelID:   inbound.ChannelID,
	}
	if err := progressor.SendThinking(ctx, signal); err != nil {
		_, _ = l.publishConnectorEvent(ctx, "connector.thinking_failed", connector, session, runID, stepID, map[string]any{
			"messageId":  inbound.ExternalMessageID,
			"channelId":  inbound.ChannelID,
			"error":      err.Error(),
			"errorClass": classifyError(err),
		})
		return func() {}
	}
	_, _ = l.publishConnectorEvent(ctx, "connector.thinking_started", connector, session, runID, stepID, thinkingPayload)

	thinkingCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-thinkingCtx.Done():
				return
			case <-ticker.C:
			}

			err := progressor.SendThinking(thinkingCtx, signal)
			if err != nil {
				_, _ = l.publishConnectorEvent(ctx, "connector.thinking_failed", connector, session, runID, stepID, map[string]any{
					"messageId":  inbound.ExternalMessageID,
					"channelId":  inbound.ChannelID,
					"error":      err.Error(),
					"errorClass": classifyError(err),
				})
				return
			}
		}
	}()
	return cancel
}

func (l *MessageLoop) routeSession(inbound imtypes.InboundMessage) (router.Session, bool, error) {
	return l.router.Route(router.RouteInput{
		Kind:      inbound.Kind,
		Channel:   inbound.ConnectorKind,
		AccountID: inbound.AccountID,
		PeerID:    inbound.PeerID,
		ThreadID:  inbound.ThreadID,
	})
}

func classifyRoutingOutcome(err error) threads.RoutingOutcome {
	if err == nil {
		return threads.RoutingOutcomeAccepted
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "disabled"):
		return threads.RoutingOutcomeDisabled
	case strings.Contains(lower, "unsupported"):
		return threads.RoutingOutcomeUnsupported
	case strings.Contains(lower, "stale"):
		return threads.RoutingOutcomeStaleSource
	case strings.Contains(lower, "tenant"):
		return threads.RoutingOutcomeInaccessibleTenantBinding
	case strings.Contains(lower, "source") || strings.Contains(lower, "routing key"):
		return threads.RoutingOutcomeUnknownSource
	default:
		return threads.RoutingOutcomeFailed
	}
}

func (l *MessageLoop) createRunAndStep(ctx context.Context, connector connectors.Connector, session router.Session, inbound imtypes.InboundMessage) (runtime.Run, runtime.Step, error) {
	run, err := l.runtime.CreateRun(runtime.CreateRunInput{
		SessionID:  session.SessionID,
		Entrypoint: connector.Kind + ".message",
		Goal:       inbound.Content,
	})
	if err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if err := l.store.UpsertRun(ctx, run); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if err := l.persistCheckpoint(ctx, run.RunID); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if _, err := l.publishRuntimeEvent(ctx, "run.created", "run", run.RunID, events.Scope{
		SessionID:   session.SessionID,
		RunID:       run.RunID,
		ConnectorID: connector.ConnectorID,
	}, map[string]any{
		"entrypoint": run.Entrypoint,
		"goal":       run.Goal,
		"status":     run.Status,
		"source":     connectorSource(connector.Kind),
		"messageId":  inbound.ExternalMessageID,
	}); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}

	step, err := l.runtime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "reply to connector message",
		Kind:  "chat_query",
		Input: map[string]any{
			"messageId": inbound.ExternalMessageID,
			"content":   inbound.Content,
		},
	})
	if err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if err := l.store.UpsertStep(ctx, step); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if err := l.persistCheckpoint(ctx, run.RunID); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	if _, err := l.publishRuntimeEvent(ctx, "step.created", "step", step.StepID, events.Scope{
		SessionID:   session.SessionID,
		RunID:       run.RunID,
		StepID:      step.StepID,
		ConnectorID: connector.ConnectorID,
	}, map[string]any{
		"title":     step.Title,
		"kind":      step.Kind,
		"status":    step.Status,
		"messageId": inbound.ExternalMessageID,
	}); err != nil {
		return runtime.Run{}, runtime.Step{}, err
	}
	return run, step, nil
}

func (l *MessageLoop) updateStepStatus(ctx context.Context, runID, stepID string, input runtime.UpdateStepStatusInput) (runtime.Step, error) {
	step, runUpdate, err := l.runtime.UpdateStepStatusAndReconcileRun(runID, stepID, input)
	if err != nil {
		return runtime.Step{}, err
	}
	if err := l.store.UpsertStep(ctx, step); err != nil {
		return runtime.Step{}, err
	}
	if runUpdate != nil {
		if err := l.store.UpsertRun(ctx, *runUpdate); err != nil {
			return runtime.Step{}, err
		}
	}
	if err := l.persistCheckpoint(ctx, runID); err != nil {
		return runtime.Step{}, err
	}

	run, _ := l.runtime.GetRun(runID)
	stepPayload := map[string]any{"status": step.Status}
	if _, err := l.publishRuntimeEvent(ctx, "step.status_changed", "step", step.StepID, events.Scope{SessionID: run.SessionID, RunID: runID, StepID: stepID}, stepPayload); err != nil {
		return runtime.Step{}, err
	}
	if runUpdate != nil {
		runPayload := map[string]any{"status": runUpdate.Status}
		if _, err := l.publishRuntimeEvent(ctx, "run.status_changed", "run", runID, events.Scope{SessionID: run.SessionID, RunID: runID}, runPayload); err != nil {
			return runtime.Step{}, err
		}
	}

	return step, nil
}

func (l *MessageLoop) persistCheckpoint(ctx context.Context, runID string) error {
	if l.checkpoints == nil {
		return nil
	}
	return l.checkpoints.SaveRunCheckpoint(ctx, runID)
}

func (l *MessageLoop) persistSession(ctx context.Context, session router.Session) error {
	if l.store == nil {
		return nil
	}
	return l.store.UpsertSession(ctx, session)
}

func (l *MessageLoop) publishSessionRouteEvents(ctx context.Context, connector connectors.Connector, session router.Session, created bool, inbound imtypes.InboundMessage) error {
	if created {
		if _, err := l.publishRuntimeEvent(ctx, "session.created", "session", session.SessionID, events.Scope{SessionID: session.SessionID}, map[string]any{
			"kind":        session.Kind,
			"channel":     session.Channel,
			"routingKey":  session.RoutingKey,
			"generation":  session.Generation,
			"source":      connectorSource(connector.Kind),
			"connectorId": connector.ConnectorID,
			"messageId":   inbound.ExternalMessageID,
		}); err != nil {
			return err
		}
	}
	_, err := l.publishRuntimeEvent(ctx, "session.routed", "session", session.SessionID, events.Scope{SessionID: session.SessionID}, map[string]any{
		"kind":        session.Kind,
		"channel":     session.Channel,
		"routingKey":  session.RoutingKey,
		"generation":  session.Generation,
		"source":      connectorSource(connector.Kind),
		"connectorId": connector.ConnectorID,
		"messageId":   inbound.ExternalMessageID,
	})
	return err
}

func (l *MessageLoop) publishMatrixRouteOutcome(ctx context.Context, connector connectors.Connector, session router.Session, record imtypes.MessageRecord, outcome, reasonCode string) error {
	if connector.Kind != "matrix" {
		return nil
	}
	_, err := l.publishConnectorEvent(ctx, events.ConnectorEventRouteOutcomeRecorded, connector, session, "", "", map[string]any{
		"tenantId":                record.TenantID,
		"connectorId":             connector.ConnectorID,
		"homeserverId":            record.ConnectorAccountID,
		"conversationId":          record.ChannelOrConversationID,
		"matrixEventId":           record.ProviderMessageID,
		"outcome":                 outcome,
		"reasonCode":              reasonCode,
		"surface":                 matrixRouteSurface(record),
		"messageDeliveryId":       record.DeliveryID,
		"connectorAccountId":      record.ConnectorAccountID,
		"channelOrConversationId": record.ChannelOrConversationID,
		"providerMessageId":       record.ProviderMessageID,
		"equivalentRuleId":        record.EquivalentRuleID,
		"redactionStatus":         "redacted",
	})
	return err
}

func matrixRouteSurface(record imtypes.MessageRecord) string {
	if strings.TrimSpace(record.PeerID) != "" && strings.TrimSpace(record.ChannelID) == "" {
		return "direct"
	}
	return "room"
}

func connectorSource(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "connector"
	}
	return "connector." + kind
}

func (l *MessageLoop) publishConnectorEvent(ctx context.Context, name string, connector connectors.Connector, session router.Session, runID, stepID string, payload map[string]any) (events.Event, error) {
	scope := events.Scope{
		SessionID:   session.SessionID,
		RunID:       runID,
		StepID:      stepID,
		ConnectorID: connector.ConnectorID,
	}
	return l.publishEvent(ctx, "connector", name, "connector", connector.ConnectorID, scope, payload)
}

func (l *MessageLoop) recordChannelForegroundReplyOutcome(ctx context.Context, connector connectors.Connector, session router.Session, record imtypes.MessageRecord, status, reasonCode string, safeEvidence map[string]string) error {
	if l == nil || l.store == nil {
		return nil
	}
	tenantID := strings.TrimSpace(record.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(connector.TenantID)
	}
	if tenantID == "" {
		return nil
	}
	outcomeID := strings.TrimSpace(record.DeliveryID)
	if outcomeID == "" {
		outcomeID = newDeliveryID()
	}
	now := time.Now().UTC()
	_, err := l.store.SaveChannelForegroundReplyOutcome(ctx, connectors.ForegroundReplyOutcome{
		ReplyOutcomeID:     outcomeID,
		TenantID:           tenantID,
		ConnectorID:        connector.ConnectorID,
		Status:             status,
		ReasonCode:         reasonCode,
		OccurredAt:         now,
		SafeEvidence:       safeEvidence,
		RedactionStatus:    connectors.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	})
	return err
}

func (l *MessageLoop) publishRuntimeEvent(ctx context.Context, name, resourceKind, resourceID string, scope events.Scope, payload map[string]any) (events.Event, error) {
	category := "run"
	if resourceKind == "session" {
		category = "session"
	}
	if resourceKind == "step" {
		category = "step"
	}
	return l.publishEvent(ctx, category, name, resourceKind, resourceID, scope, payload)
}

func (l *MessageLoop) publishEvent(ctx context.Context, category, name, resourceKind, resourceID string, scope events.Scope, payload map[string]any) (events.Event, error) {
	if l.eventBus == nil {
		return events.Event{}, nil
	}
	event := events.Event{
		Category: category,
		Name:     name,
		Scope:    scope,
		Resource: events.Resource{
			Kind: resourceKind,
			ID:   resourceID,
		},
		Payload: payload,
	}
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if l.store != nil {
		persisted, err := l.store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, err
		}
		event = persisted
	}
	return l.eventBus.Publish(event), nil
}

func newDeliveryID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "delivery_fallback"
	}
	return "delivery_" + hex.EncodeToString(buf)
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}
	return "evt_" + hex.EncodeToString(buf)
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	classified, ok := err.(classifiedError)
	if !ok {
		return ""
	}
	return strings.TrimSpace(classified.ErrorClass())
}

func safeReplyFailureReason(err error) string {
	if class := classifyError(err); class != "" {
		return class
	}
	return "reply_failed"
}

type streamReplyProgress struct {
	loop           *MessageLoop
	progressor     ReplyProgressor
	connector      connectors.Connector
	session        router.Session
	runID          string
	stepID         string
	inbound        imtypes.InboundMessage
	responseTo     imtypes.MessageRecord
	record         imtypes.MessageRecord
	records        []imtypes.MessageRecord
	lastFlushed    string
	lastFlushAt    time.Time
	flushInterval  time.Duration
	maxReplyLength int
	stopThinking   func()
	partialErr     error
	err            error
}

func (p *streamReplyProgress) OnChunk(ctx context.Context, reply string) error {
	if p.err != nil {
		return nil
	}
	return p.flush(ctx, reply, streamReplyModeProgress)
}

func (p *streamReplyProgress) Complete(ctx context.Context, reply string) error {
	if p.err != nil {
		return p.err
	}
	return p.flush(ctx, reply, streamReplyModeComplete)
}

func (p *streamReplyProgress) CompletePartial(ctx context.Context, reply string, cause error) error {
	if p.err != nil {
		return p.err
	}
	p.partialErr = cause
	return p.flush(ctx, appendPartialMarker(reply), streamReplyModePartial)
}

type streamReplyMode string

const (
	streamReplyModeProgress streamReplyMode = "progress"
	streamReplyModeComplete streamReplyMode = "complete"
	streamReplyModePartial  streamReplyMode = "partial"
)

func (p *streamReplyProgress) flush(ctx context.Context, reply string, mode streamReplyMode) error {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	force := mode != streamReplyModeProgress
	if reply == p.lastFlushed && !force {
		return nil
	}
	if !force && !p.lastFlushAt.IsZero() && time.Since(p.lastFlushAt) < p.flushInterval {
		return nil
	}

	now := time.Now().UTC()
	replyParts := splitReplyContent(reply, p.maxReplyLength)
	startedNow := len(p.records) == 0
	replyMessageIDs := make([]string, 0, len(replyParts))
	for index, replyPart := range replyParts {
		if index >= len(p.records) {
			record := p.loop.newOutboundRecord(p.connector, p.session, runtime.Run{RunID: p.runID}, p.inbound, p.responseTo, replyPart)
			record.Status = imtypes.DeliveryStatusStreaming
			if err := p.loop.store.UpsertConnectorMessage(ctx, record); err != nil {
				p.err = err
				return err
			}

			sentReply, err := p.progressor.SendReply(ctx, imtypes.OutboundReply{
				ConnectorID:              p.connector.ConnectorID,
				ChannelID:                p.inbound.ChannelID,
				Content:                  replyPart,
				ReplyToExternalMessageID: replyToExternalMessageID(p.inbound),
			})
			if err != nil {
				p.err = err
				return err
			}
			if p.stopThinking != nil {
				p.stopThinking()
				p.stopThinking = nil
			}
			record.ExternalMessageID = sentReply.ExternalMessageID
			record.Content = replyPart
			record.UpdatedAt = now
			if mode == streamReplyModeComplete {
				record.Status = imtypes.DeliveryStatusReplied
			} else if mode == streamReplyModePartial {
				record.Status = imtypes.DeliveryStatusPartial
				record.Error = partialDeliveryError(p.partialErr)
			}
			if err := p.loop.store.UpsertConnectorMessage(ctx, record); err != nil {
				p.err = err
				return err
			}
			p.records = append(p.records, record)
			if index == 0 {
				p.record = record
			}
			replyMessageIDs = append(replyMessageIDs, record.ExternalMessageID)
			continue
		}

		record := p.records[index]
		replyMessageIDs = append(replyMessageIDs, record.ExternalMessageID)
		if replyPart != record.Content {
			if err := p.progressor.EditReply(ctx, imtypes.ReplyEdit{
				ConnectorID:       p.connector.ConnectorID,
				ChannelID:         p.inbound.ChannelID,
				ExternalMessageID: record.ExternalMessageID,
				Content:           replyPart,
			}); err != nil {
				p.err = err
				return err
			}
			record.Content = replyPart
			record.UpdatedAt = now
		}
		record.Status = imtypes.DeliveryStatusStreaming
		if mode == streamReplyModeComplete {
			record.Status = imtypes.DeliveryStatusReplied
			record.Error = ""
		} else if mode == streamReplyModePartial {
			record.Status = imtypes.DeliveryStatusPartial
			record.Error = partialDeliveryError(p.partialErr)
		}
		if err := p.loop.store.UpsertConnectorMessage(ctx, record); err != nil {
			p.err = err
			return err
		}
		p.records[index] = record
		if index == 0 {
			p.record = record
		}
	}

	eventName := "connector.reply_stream_updated"
	if startedNow {
		eventName = "connector.reply_stream_started"
	} else if mode == streamReplyModeComplete {
		eventName = "connector.reply_sent"
	} else if mode == streamReplyModePartial {
		eventName = "connector.reply_partial"
	}
	payload := map[string]any{
		"messageId":       p.inbound.ExternalMessageID,
		"replyMessageId":  p.record.ExternalMessageID,
		"replyMessageIds": replyMessageIDs,
		"partCount":       len(replyMessageIDs),
		"contentLength":   len(reply),
	}
	if mode == streamReplyModePartial {
		payload["error"] = partialDeliveryError(p.partialErr)
		payload["errorClass"] = classifyError(p.partialErr)
	}
	_, _ = p.loop.publishConnectorEvent(ctx, eventName, p.connector, p.session, p.runID, p.stepID, payload)
	if mode == streamReplyModeComplete {
		_ = p.loop.recordChannelForegroundReplyOutcome(ctx, p.connector, p.session, p.record, "sent", "reply_sent", map[string]string{
			"messageId":       p.inbound.ExternalMessageID,
			"replyMessageId":  p.record.ExternalMessageID,
			"replyMessageIds": strings.Join(replyMessageIDs, ","),
		})
	} else if mode == streamReplyModePartial {
		_ = p.loop.recordChannelForegroundReplyOutcome(ctx, p.connector, p.session, p.record, "partial", safeReplyFailureReason(p.partialErr), map[string]string{
			"errorClass": classifyError(p.partialErr),
			"messageId":  p.inbound.ExternalMessageID,
		})
	}
	p.lastFlushed = reply
	p.lastFlushAt = now
	return nil
}

func appendPartialMarker(reply string) string {
	const suffix = "\n\n[response interrupted]"
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return reply
	}
	if strings.HasSuffix(reply, suffix) {
		return reply
	}
	return reply + suffix
}

func partialDeliveryError(err error) string {
	if err == nil {
		return ""
	}
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) {
		if strings.TrimSpace(providerErr.Message) != "" {
			return providerErr.Message
		}
		return providerErr.Code
	}
	return err.Error()
}

func splitReplyContent(reply string, maxMessageLength int) []string {
	if maxMessageLength <= 0 {
		return []string{reply}
	}
	runes := []rune(reply)
	if len(runes) <= maxMessageLength {
		return []string{reply}
	}

	parts := make([]string, 0, (len(runes)+maxMessageLength-1)/maxMessageLength)
	for start := 0; start < len(runes); start += maxMessageLength {
		end := start + maxMessageLength
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
