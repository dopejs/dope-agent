package im

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type ReplySender interface {
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
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
	Session   router.Session
	Run       runtime.Run
	Step      runtime.Step
	Reply     string
	Duplicate bool
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
		ConnectorID:              connector.ConnectorID,
		Direction:                imtypes.DeliveryDirectionInbound,
		ExternalMessageID:        inbound.ExternalMessageID,
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
		return ProcessResult{Duplicate: true}, nil
	}

	session, createdSession, err := l.routeSession(inbound)
	if err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{}, err
	}
	if err := l.persistSession(ctx, session); err != nil {
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = err.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{}, err
	}
	persistedInbound.SessionID = session.SessionID
	persistedInbound.UpdatedAt = time.Now().UTC()
	if err := l.store.UpsertConnectorMessage(ctx, persistedInbound); err != nil {
		return ProcessResult{}, err
	}
	if err := l.publishSessionRouteEvents(ctx, connector, session, createdSession, inbound); err != nil {
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

	queryResult, queryErr := l.chat.Query(ctx, chat.QueryInput{
		Query: inbound.Content,
		Scope: events.Scope{
			SessionID:   session.SessionID,
			RunID:       run.RunID,
			StepID:      step.StepID,
			ConnectorID: connector.ConnectorID,
		},
	})
	if queryErr != nil {
		_, _ = l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"error": queryErr.Error(),
			},
		})
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		persistedInbound.Error = queryErr.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		return ProcessResult{Session: session, Run: run, Step: step}, queryErr
	}

	if _, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
		return ProcessResult{}, err
	}

	outboundRecord := imtypes.MessageRecord{
		DeliveryID:               newDeliveryID(),
		ConnectorID:              connector.ConnectorID,
		Direction:                imtypes.DeliveryDirectionOutbound,
		SessionID:                session.SessionID,
		RunID:                    run.RunID,
		ChannelID:                inbound.ChannelID,
		PeerID:                   inbound.PeerID,
		ThreadID:                 inbound.ThreadID,
		Content:                  queryResult.Dispatch.Output,
		Status:                   imtypes.DeliveryStatusProcessing,
		ResponseToDeliveryID:     persistedInbound.DeliveryID,
		ReplyToExternalMessageID: inbound.ExternalMessageID,
		CreatedAt:                time.Now().UTC(),
		UpdatedAt:                time.Now().UTC(),
	}
	if err := l.store.UpsertConnectorMessage(ctx, outboundRecord); err != nil {
		return ProcessResult{}, err
	}

	sentReply, sendErr := replies.SendReply(ctx, imtypes.OutboundReply{
		ConnectorID:              connector.ConnectorID,
		ChannelID:                inbound.ChannelID,
		Content:                  queryResult.Dispatch.Output,
		ReplyToExternalMessageID: inbound.ExternalMessageID,
	})
	if sendErr != nil {
		outboundRecord.Status = imtypes.DeliveryStatusFailed
		outboundRecord.Error = sendErr.Error()
		outboundRecord.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, outboundRecord)
		_, _ = l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"reply": queryResult.Dispatch.Output,
				"error": sendErr.Error(),
			},
		})
		_, _ = l.publishConnectorEvent(ctx, "connector.reply_failed", connector, session, run.RunID, step.StepID, map[string]any{
			"messageId":  inbound.ExternalMessageID,
			"error":      sendErr.Error(),
			"errorClass": classifyError(sendErr),
		})
		return ProcessResult{Session: session, Run: run, Step: step, Reply: queryResult.Dispatch.Output}, sendErr
	}

	outboundRecord.ExternalMessageID = sentReply.ExternalMessageID
	outboundRecord.Status = imtypes.DeliveryStatusReplied
	outboundRecord.UpdatedAt = time.Now().UTC()
	if err := l.store.UpsertConnectorMessage(ctx, outboundRecord); err != nil {
		return ProcessResult{}, err
	}

	finalStep, err := l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCompleted,
		Output: map[string]any{
			"reply":                    queryResult.Dispatch.Output,
			"replyMessageId":           sentReply.ExternalMessageID,
			"llmDispatchId":            queryResult.Dispatch.DispatchID,
			"llmProvider":              queryResult.Dispatch.Provider,
			"llmModel":                 queryResult.Dispatch.Model,
			"llmUsage":                 queryResult.Dispatch.Usage,
			"replyToExternalMessageId": inbound.ExternalMessageID,
		},
	})
	if err != nil {
		return ProcessResult{}, err
	}
	_, _ = l.publishConnectorEvent(ctx, "connector.reply_sent", connector, session, run.RunID, finalStep.StepID, map[string]any{
		"messageId":      inbound.ExternalMessageID,
		"replyMessageId": sentReply.ExternalMessageID,
	})

	run, _ = l.runtime.GetRun(run.RunID)
	return ProcessResult{
		Session: session,
		Run:     run,
		Step:    finalStep,
		Reply:   queryResult.Dispatch.Output,
	}, nil
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
		"source":     "connector.discord",
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
			"source":      "connector.discord",
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
		"source":      "connector.discord",
		"connectorId": connector.ConnectorID,
		"messageId":   inbound.ExternalMessageID,
	})
	return err
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
