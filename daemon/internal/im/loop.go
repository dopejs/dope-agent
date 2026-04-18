package im

import (
	"context"
	"crypto/rand"
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
		if outboundRecord.DeliveryID != "" && !partialReply {
			outboundRecord.Status = imtypes.DeliveryStatusFailed
			outboundRecord.Error = sendErr.Error()
			outboundRecord.UpdatedAt = time.Now().UTC()
			_ = l.store.UpsertConnectorMessage(ctx, outboundRecord)
		}
		_, _ = l.updateStepStatus(ctx, run.RunID, step.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"reply":       queryResult.Dispatch.Output,
				"partial":     partialReply,
				"replyStatus": outboundRecord.Status,
				"error":       sendErr.Error(),
			},
		})
		persistedInbound.Status = imtypes.DeliveryStatusFailed
		if partialReply {
			persistedInbound.Status = imtypes.DeliveryStatusPartial
		}
		persistedInbound.Error = sendErr.Error()
		persistedInbound.UpdatedAt = time.Now().UTC()
		_ = l.store.UpsertConnectorMessage(ctx, persistedInbound)
		if !partialReply {
			_, _ = l.publishConnectorEvent(ctx, "connector.reply_failed", connector, session, run.RunID, step.StepID, map[string]any{
				"messageId":      inbound.ExternalMessageID,
				"replyMessageId": outboundRecord.ExternalMessageID,
				"error":          sendErr.Error(),
				"errorClass":     classifyError(sendErr),
			})
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
			"replyToExternalMessageId": inbound.ExternalMessageID,
		},
	})
	if err != nil {
		return ProcessResult{}, err
	}
	run, _ = l.runtime.GetRun(run.RunID)
	return ProcessResult{
		Session: session,
		Run:     run,
		Step:    finalStep,
		Reply:   queryResult.Dispatch.Output,
	}, nil
}

func (l *MessageLoop) executeReplyPath(ctx context.Context, connector connectors.Connector, session router.Session, run runtime.Run, step runtime.Step, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, replies ReplySender, progressor ReplyProgressor, capabilities imtypes.ReplyCapabilities, scope events.Scope, stopThinking func()) (chat.QueryResult, imtypes.MessageRecord, error) {
	if capabilities.SupportsStreaming && progressor != nil {
		return l.executeStreamingReply(ctx, connector, session, run, step, inbound, persistedInbound, progressor, capabilities, scope, stopThinking)
	}
	return l.executeFinalReply(ctx, connector, session, run, step, inbound, persistedInbound, replies, capabilities, scope, stopThinking)
}

func (l *MessageLoop) executeFinalReply(ctx context.Context, connector connectors.Connector, session router.Session, run runtime.Run, step runtime.Step, inbound imtypes.InboundMessage, persistedInbound imtypes.MessageRecord, replies ReplySender, capabilities imtypes.ReplyCapabilities, scope events.Scope, stopThinking func()) (chat.QueryResult, imtypes.MessageRecord, error) {
	queryResult, queryErr := l.chat.Query(ctx, chat.QueryInput{
		Query: inbound.Content,
		Scope: scope,
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
		record := l.newOutboundRecord(connector, session, run, inbound, persistedInbound.DeliveryID, replyPart)
		if err := l.store.UpsertConnectorMessage(ctx, record); err != nil {
			return chat.QueryResult{}, imtypes.MessageRecord{}, err
		}

		sentReply, sendErr := replies.SendReply(ctx, imtypes.OutboundReply{
			ConnectorID:              connector.ConnectorID,
			ChannelID:                inbound.ChannelID,
			Content:                  replyPart,
			ReplyToExternalMessageID: inbound.ExternalMessageID,
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
		responseToID:   persistedInbound.DeliveryID,
		flushInterval:  500 * time.Millisecond,
		maxReplyLength: capabilities.MaxMessageLength,
		stopThinking:   stopThinking,
	}

	var progressErr error
	queryResult, queryErr := l.chat.Stream(ctx, chat.QueryInput{
		Query: inbound.Content,
		Scope: scope,
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

func (l *MessageLoop) newOutboundRecord(connector connectors.Connector, session router.Session, run runtime.Run, inbound imtypes.InboundMessage, responseToDeliveryID, content string) imtypes.MessageRecord {
	now := time.Now().UTC()
	return imtypes.MessageRecord{
		DeliveryID:               newDeliveryID(),
		ConnectorID:              connector.ConnectorID,
		Direction:                imtypes.DeliveryDirectionOutbound,
		SessionID:                session.SessionID,
		RunID:                    run.RunID,
		ChannelID:                inbound.ChannelID,
		PeerID:                   inbound.PeerID,
		ThreadID:                 inbound.ThreadID,
		Content:                  content,
		Status:                   imtypes.DeliveryStatusProcessing,
		ResponseToDeliveryID:     responseToDeliveryID,
		ReplyToExternalMessageID: inbound.ExternalMessageID,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
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

type streamReplyProgress struct {
	loop           *MessageLoop
	progressor     ReplyProgressor
	connector      connectors.Connector
	session        router.Session
	runID          string
	stepID         string
	inbound        imtypes.InboundMessage
	responseToID   string
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
			record := p.loop.newOutboundRecord(p.connector, p.session, runtime.Run{RunID: p.runID}, p.inbound, p.responseToID, replyPart)
			record.Status = imtypes.DeliveryStatusStreaming
			if err := p.loop.store.UpsertConnectorMessage(ctx, record); err != nil {
				p.err = err
				return err
			}

			sentReply, err := p.progressor.SendReply(ctx, imtypes.OutboundReply{
				ConnectorID:              p.connector.ConnectorID,
				ChannelID:                p.inbound.ChannelID,
				Content:                  replyPart,
				ReplyToExternalMessageID: p.inbound.ExternalMessageID,
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
