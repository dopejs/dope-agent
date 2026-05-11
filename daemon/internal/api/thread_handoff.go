package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type threadHandoffDestinationRequest struct {
	Surface              string `json:"surface"`
	ConnectorID          string `json:"connectorId,omitempty"`
	SourceAccountID      string `json:"sourceAccountId,omitempty"`
	SourceConversationID string `json:"sourceConversationId,omitempty"`
	ConversationShape    string `json:"conversationShape,omitempty"`
}

type threadHandoffRequest struct {
	Destination threadHandoffDestinationRequest `json:"destination"`
	ReasonCode  string                          `json:"reasonCode"`
}

func isThreadHandoffRoute(path string) bool {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, "/v1/threads/"), "/"), "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "handoffs"
}

func handleThreadHandoffNotImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, "thread handoff creation is not enabled yet")
}

func handleThreadHandoffCreate(sqliteStore *store.SQLiteStore, eventBus *events.Bus, w http.ResponseWriter, r *http.Request, sourceThreadID string) {
	tenantContext, ok := requireThreadPermission(r, identity.PermissionConnectorsManage)
	if !ok {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if sqliteStore == nil {
		http.NotFound(w, r)
		return
	}
	var input threadHandoffRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	link, err := createThreadHandoff(r.Context(), sqliteStore, tenantContext, sourceThreadID, input, time.Now().UTC())
	if err != nil {
		handleThreadHandoffError(w, err)
		return
	}
	if eventBus != nil {
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.ThreadHandoffLinkedEvent(link)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, link)
}

func createThreadHandoff(ctx context.Context, sqliteStore *store.SQLiteStore, tenantContext identity.TenantContext, sourceThreadID string, input threadHandoffRequest, now time.Time) (threads.HandoffLink, error) {
	sourceThread, found, err := sqliteStore.GetThreadForTenant(ctx, tenantContext.TenantID, sourceThreadID)
	if err != nil {
		return threads.HandoffLink{}, err
	}
	if !found {
		return threads.HandoffLink{}, errThreadHandoffNotFound
	}
	sourceShape, hasSourceShape, err := sqliteStore.GetConversationShapeForThread(ctx, tenantContext.TenantID, sourceThreadID)
	if err != nil {
		return threads.HandoffLink{}, err
	}
	if !hasSourceShape || sourceShape.ShapeEvidenceStatus != threads.ShapeEvidenceStatusProven || sourceShape.Shape == threads.ConversationShapeUnknown || sourceShape.Shape == threads.ConversationShapeUnsupported {
		return threads.HandoffLink{}, threads.ErrHandoffNotEligible
	}
	sourceEligible, sourcePermissionAllowed, err := validateHandoffSource(ctx, sqliteStore, tenantContext.TenantID, sourceThread, sourceShape)
	if err != nil {
		return threads.HandoffLink{}, err
	}
	destinationThread, destinationShape, destinationPermissionAllowed, err := ensureHandoffDestinationThread(ctx, sqliteStore, tenantContext.TenantID, input.Destination, now)
	if err != nil {
		return threads.HandoffLink{}, err
	}
	link := threads.HandoffLink{
		TenantID:                     tenantContext.TenantID,
		SourceThreadID:               sourceThread.ThreadID,
		SourceSessionSegmentID:       sourceThread.CurrentSessionSegmentID,
		DestinationThreadID:          destinationThread.ThreadID,
		DestinationSessionSegmentID:  destinationThread.CurrentSessionSegmentID,
		SourceConversationShape:      sourceShape.Shape,
		DestinationConversationShape: destinationShape.Shape,
		SourceKind:                   sourceThread.SourceKind,
		DestinationKind:              destinationThread.SourceKind,
		SourceConnectorID:            sourceShape.ConnectorID,
		DestinationConnectorID:       destinationShape.ConnectorID,
		SourceConversationID:         sourceShape.SourceConversationID,
		DestinationConversationID:    destinationShape.SourceConversationID,
		ActorPrincipalID:             tenantContext.PrincipalID,
		PermissionGate:               "connectors.manage",
		Status:                       threads.HandoffStatusSucceeded,
		ReasonCode:                   coalesceReason(input.ReasonCode, "user_requested_handoff"),
		SourceReferenceStatus:        threads.HandoffSourceReferenceAvailable,
		CreatedAt:                    now,
		RedactionStatus:              threads.RedactionStatusRedacted,
	}
	if err := threads.ValidateHandoff(threads.HandoffValidationInput{
		Link:                         link,
		HasMutationPermission:        true,
		SourceEligible:               sourceEligible,
		DestinationEligible:          destinationShape.ShapeEvidenceStatus == threads.ShapeEvidenceStatusProven,
		SourcePermissionAllowed:      sourcePermissionAllowed,
		DestinationPermissionAllowed: destinationPermissionAllowed,
	}); err != nil {
		return threads.HandoffLink{}, err
	}
	saved, err := sqliteStore.SaveHandoffLink(ctx, link)
	if err != nil {
		return threads.HandoffLink{}, err
	}
	turns, err := sqliteStore.ListContinuityTurns(ctx, store.ContinuityLookupQuery{
		TenantID:         tenantContext.TenantID,
		ThreadID:         sourceThread.ThreadID,
		SessionSegmentID: sourceThread.CurrentSessionSegmentID,
		Now:              now,
	})
	if err != nil {
		return threads.HandoffLink{}, err
	}
	refs := threads.BuildHandoffSourceReferences(saved, turns, now)
	if len(refs) == 0 {
		saved.SourceReferenceStatus = threads.HandoffSourceReferenceNone
		return sqliteStore.SaveHandoffLink(ctx, saved)
	}
	if _, err := sqliteStore.SaveHandoffSourceReferences(ctx, refs); err != nil {
		return threads.HandoffLink{}, err
	}
	return saved, nil
}

func ensureHandoffDestinationThread(ctx context.Context, sqliteStore *store.SQLiteStore, tenantID string, destination threadHandoffDestinationRequest, now time.Time) (threads.Thread, threads.ConversationShapeEvidence, bool, error) {
	switch strings.TrimSpace(destination.Surface) {
	case "web":
		threadID := "thr_handoff_web_" + shortHandoffHash(tenantID+":"+now.Format(time.RFC3339Nano))
		segmentID := "seg_" + threadID
		thread := threads.Thread{
			ThreadID:                threadID,
			TenantID:                tenantID,
			LifecycleState:          threads.LifecycleStateActive,
			CurrentSessionSegmentID: segmentID,
			SourceKind:              threads.SourceKindShell,
			SourceSummary:           "Web handoff destination",
			LastActivityAt:          now,
			CreatedAt:               now,
			UpdatedAt:               now,
			RetentionExpiresAt:      sqliteStore.ThreadRetentionExpiry(ctx, tenantID, now),
			RedactionStatus:         threads.RedactionStatusRedacted,
		}
		if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
		}
		if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: segmentID, ThreadID: threadID, TenantID: tenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
		}
		shape := threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
			TenantID:                  tenantID,
			ThreadID:                  threadID,
			SessionSegmentID:          segmentID,
			SourceKind:                threads.SourceKindShell,
			SourceConversationSummary: "Web handoff destination",
			ClaimedShape:              threads.ConversationShapeWeb,
			Now:                       now,
		})
		shape, err := sqliteStore.SaveConversationShapeEvidence(ctx, shape)
		return thread, shape, true, err
	case "channel":
		shapeValue, err := threads.NormalizeConversationShape(threads.ConversationShape(destination.ConversationShape))
		if err != nil || (shapeValue != threads.ConversationShapeGroup && shapeValue != threads.ConversationShapeRoom && shapeValue != threads.ConversationShapeDirectMessage) {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, threads.ErrHandoffNotEligible
		}
		destinationEligible, destinationPermissionAllowed, err := validateChannelHandoffEndpoint(ctx, sqliteStore, tenantID, destination.ConnectorID, destination.SourceConversationID, connectors.HandoffSurfaceDestinationSupport)
		if err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
		}
		if !destinationPermissionAllowed {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, threads.ErrHandoffPermissionDenied
		}
		if !destinationEligible {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, destinationPermissionAllowed, threads.ErrHandoffNotEligible
		}
		destinationConnector, _, err := findConnectorForTenant(ctx, sqliteStore, tenantID, destination.ConnectorID)
		if err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
		}
		key, err := threads.NormalizeSourceContinuationKey(threads.SourceContinuationKey{
			TenantID:             tenantID,
			ConnectorID:          destination.ConnectorID,
			SourceAccountID:      destination.SourceAccountID,
			SourceConversationID: destination.SourceConversationID,
		})
		if err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, threads.ErrHandoffNotEligible
		}
		current, found, err := sqliteStore.GetCurrentThreadForSource(ctx, key)
		if err != nil {
			return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
		}
		if !found {
			threadID := "thr_handoff_channel_" + shortHandoffHash(key.String())
			segmentID := "seg_" + threadID
			current = threads.Thread{
				ThreadID:                threadID,
				TenantID:                tenantID,
				LifecycleState:          threads.LifecycleStateActive,
				CurrentSessionSegmentID: segmentID,
				SourceKind:              threads.SourceKindChannel,
				SourceSummary:           destination.ConnectorID + " / " + destination.SourceConversationID,
				LastActivityAt:          now,
				CreatedAt:               now,
				UpdatedAt:               now,
				RetentionExpiresAt:      sqliteStore.ThreadRetentionExpiry(ctx, tenantID, now),
				RedactionStatus:         threads.RedactionStatusRedacted,
			}
			if err := sqliteStore.UpsertThread(ctx, current); err != nil {
				return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
			}
			if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: segmentID, ThreadID: threadID, TenantID: tenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
				return threads.Thread{}, threads.ConversationShapeEvidence{}, false, err
			}
		}
		shape := threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
			TenantID:                  tenantID,
			ThreadID:                  current.ThreadID,
			SessionSegmentID:          current.CurrentSessionSegmentID,
			SourceKind:                threads.SourceKindChannel,
			ConnectorID:               destination.ConnectorID,
			ConnectorKind:             destinationConnector.Kind,
			SourceAccountID:           destination.SourceAccountID,
			SourceConversationID:      destination.SourceConversationID,
			SourceConversationSummary: current.SourceSummary,
			ClaimedShape:              shapeValue,
			Now:                       now,
		})
		shape, err = sqliteStore.SaveConversationShapeEvidence(ctx, shape)
		return current, shape, destinationPermissionAllowed, err
	default:
		return threads.Thread{}, threads.ConversationShapeEvidence{}, false, threads.ErrHandoffNotEligible
	}
}

func validateHandoffSource(ctx context.Context, sqliteStore *store.SQLiteStore, tenantID string, sourceThread threads.Thread, sourceShape threads.ConversationShapeEvidence) (bool, bool, error) {
	if sourceThread.LifecycleState == threads.LifecycleStateArchived {
		return false, true, nil
	}
	if sourceShape.SourceKind != threads.SourceKindChannel {
		return true, true, nil
	}
	return validateChannelHandoffEndpoint(ctx, sqliteStore, tenantID, sourceShape.ConnectorID, sourceShape.SourceConversationID, connectors.HandoffSurfaceSourceSupport)
}

func validateChannelHandoffEndpoint(ctx context.Context, sqliteStore *store.SQLiteStore, tenantID, connectorID, sourceConversationID, capabilityKey string) (bool, bool, error) {
	if sqliteStore == nil {
		return false, false, nil
	}
	connector, found, err := findConnectorForTenant(ctx, sqliteStore, tenantID, connectorID)
	if err != nil {
		return false, false, err
	}
	if !found {
		return false, false, nil
	}
	if connector.Status == connectors.StatusDisabled || connector.Status == connectors.StatusFailed || connector.Status == connectors.StatusBackingOff {
		return false, false, nil
	}
	if connectorHandoffCapabilityUnsupported(connector, capabilityKey) {
		return false, true, nil
	}
	policy, found, err := sqliteStore.GetChannelRoutePolicy(ctx, tenantID, connectorID)
	if err != nil {
		return false, false, err
	}
	if !found || !connectors.RoutePolicyIsValid(policy) {
		return false, false, nil
	}
	return connectors.RoutePolicyAllowsConversation(policy, sourceConversationID), true, nil
}

func connectorHandoffCapabilityUnsupported(connector connectors.Connector, capabilityKey string) bool {
	if len(connector.CapabilityProfile) == 0 || strings.TrimSpace(capabilityKey) == "" {
		return false
	}
	value, ok := connector.CapabilityProfile[capabilityKey]
	if !ok {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(value)) != string(connectors.SurfaceSupported) &&
		strings.TrimSpace(fmt.Sprint(value)) != string(connectors.SurfaceLimited)
}

func findConnectorForTenant(ctx context.Context, sqliteStore *store.SQLiteStore, tenantID, connectorID string) (connectors.Connector, bool, error) {
	items, err := sqliteStore.ListConnectors(ctx)
	if err != nil {
		return connectors.Connector{}, false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.ConnectorID) != strings.TrimSpace(connectorID) {
			continue
		}
		if strings.TrimSpace(item.TenantID) != "" && strings.TrimSpace(item.TenantID) != strings.TrimSpace(tenantID) {
			continue
		}
		return item, true, nil
	}
	return connectors.Connector{}, false, nil
}

var errThreadHandoffNotFound = errors.New("thread not found")

func handleThreadHandoffError(w http.ResponseWriter, err error) {
	switch err {
	case threads.ErrHandoffPermissionDenied:
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
	case threads.ErrHandoffSameThread, threads.ErrHandoffNotEligible:
		writeError(w, http.StatusConflict, err.Error())
	case errThreadHandoffNotFound:
		writeError(w, http.StatusNotFound, "thread not found")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func shortHandoffHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:24]
}
