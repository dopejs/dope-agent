package api

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type MatrixSetupProjection struct {
	TenantID            string                           `json:"tenantId,omitempty"`
	ConnectorID         string                           `json:"connectorId"`
	ConnectorKind       string                           `json:"connectorKind"`
	DisplayName         string                           `json:"displayName"`
	Status              string                           `json:"status"`
	TerminalState       string                           `json:"terminalState"`
	BotCredentialState  string                           `json:"botCredentialState"`
	HomeserverState     string                           `json:"homeserverState"`
	RoutePolicyState    string                           `json:"routePolicyState"`
	DeliveryEligible    bool                             `json:"deliveryEligible"`
	HomeserverBindingID string                           `json:"homeserverBindingId"`
	ReasonCode          string                           `json:"reasonCode,omitempty"`
	RedactionStatus     string                           `json:"redactionStatus"`
	CreatedAt           time.Time                        `json:"createdAt"`
	UpdatedAt           time.Time                        `json:"updatedAt"`
	ValidatedAt         time.Time                        `json:"validatedAt,omitempty"`
	RetentionExpiresAt  time.Time                        `json:"retentionExpiresAt"`
	HomeserverBinding   *matrixHomeserverBindingResource `json:"homeserverBinding,omitempty"`
	RoutePolicy         *matrixRoutePolicyResource       `json:"routePolicy,omitempty"`
}

type matrixHomeserverBindingResource struct {
	HomeserverURL             string            `json:"homeserverUrl"`
	HomeserverName            string            `json:"homeserverName,omitempty"`
	BotUserID                 string            `json:"botUserId"`
	BotDeviceID               string            `json:"botDeviceId,omitempty"`
	AuthorizationState        string            `json:"authorizationState"`
	HomeserverCapabilityState string            `json:"homeserverCapabilityState"`
	ValidatedAt               time.Time         `json:"validatedAt"`
	RedactionStatus           string            `json:"redactionStatus"`
	SafeEvidence              map[string]string `json:"safeEvidence,omitempty"`
}

type matrixRoutePolicyResource struct {
	ConnectorID         string                            `json:"connectorId"`
	HomeserverBindingID string                            `json:"homeserverBindingId"`
	SelectedRooms       []matrixConversationRouteResource `json:"selectedRooms"`
	AllowedDirectUsers  []string                          `json:"allowedDirectUsers"`
	RoomInvocationGate  string                            `json:"roomInvocationGate"`
	ConfiguredCommands  []string                          `json:"configuredCommands"`
	EncryptedRoomPolicy string                            `json:"encryptedRoomPolicy"`
	ValidationState     string                            `json:"validationState"`
	ReasonCode          string                            `json:"reasonCode,omitempty"`
	ValidatedAt         time.Time                         `json:"validatedAt"`
	RedactionStatus     string                            `json:"redactionStatus"`
	SafeEvidence        map[string]string                 `json:"safeEvidence,omitempty"`
}

type matrixConversationRouteResource struct {
	ConversationID     string            `json:"conversationId"`
	ConversationType   string            `json:"conversationType"`
	RoomSelectionState string            `json:"roomSelectionState"`
	ValidationState    string            `json:"validationState"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	RedactionStatus    string            `json:"redactionStatus,omitempty"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

func projectMatrixSetup(record store.MatrixHostedSetupRecord) MatrixSetupProjection {
	return MatrixSetupProjection{
		TenantID:            record.TenantID,
		ConnectorID:         record.ConnectorID,
		ConnectorKind:       record.ConnectorKind,
		DisplayName:         record.DisplayName,
		Status:              record.Status,
		TerminalState:       record.TerminalState,
		BotCredentialState:  record.BotCredentialState,
		HomeserverState:     record.HomeserverState,
		RoutePolicyState:    record.RoutePolicyState,
		DeliveryEligible:    record.DeliveryEligible,
		HomeserverBindingID: record.HomeserverBindingID,
		ReasonCode:          record.ReasonCode,
		RedactionStatus:     record.RedactionStatus,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
		ValidatedAt:         record.ValidatedAt,
		RetentionExpiresAt:  record.RetentionExpiresAt,
		HomeserverBinding:   projectMatrixHomeserverBinding(record.HomeserverBinding),
		RoutePolicy:         projectMatrixRoutePolicy(record.RoutePolicy),
	}
}

func projectMatrixHomeserverBinding(record *store.MatrixHomeserverBindingRecord) *matrixHomeserverBindingResource {
	if record == nil {
		return nil
	}
	return &matrixHomeserverBindingResource{
		HomeserverURL:             record.HomeserverURL,
		HomeserverName:            record.HomeserverName,
		BotUserID:                 record.BotUserID,
		BotDeviceID:               record.BotDeviceID,
		AuthorizationState:        record.AuthorizationState,
		HomeserverCapabilityState: record.HomeserverCapabilityState,
		ValidatedAt:               record.ValidatedAt,
		RedactionStatus:           record.RedactionStatus,
		SafeEvidence:              record.SafeEvidence,
	}
}

func projectMatrixRoutePolicy(record *store.MatrixRoutePolicyRecord) *matrixRoutePolicyResource {
	if record == nil {
		return nil
	}
	rooms := make([]matrixConversationRouteResource, 0, len(record.SelectedRooms))
	for _, room := range record.SelectedRooms {
		rooms = append(rooms, matrixConversationRouteResource{
			ConversationID:     room.ConversationID,
			ConversationType:   room.ConversationType,
			RoomSelectionState: room.RoomSelectionState,
			ValidationState:    room.ValidationState,
			ReasonCode:         room.ReasonCode,
			RedactionStatus:    room.RedactionStatus,
			SafeEvidence:       room.SafeEvidence,
		})
	}
	return &matrixRoutePolicyResource{
		ConnectorID:         record.ConnectorID,
		HomeserverBindingID: record.HomeserverBindingID,
		SelectedRooms:       rooms,
		AllowedDirectUsers:  append([]string(nil), record.AllowedDirectUsers...),
		RoomInvocationGate:  record.RoomInvocationGate,
		ConfiguredCommands:  append([]string(nil), record.ConfiguredCommands...),
		EncryptedRoomPolicy: record.EncryptedRoomPolicy,
		ValidationState:     record.ValidationState,
		ReasonCode:          record.ReasonCode,
		ValidatedAt:         record.ValidatedAt,
		RedactionStatus:     record.RedactionStatus,
		SafeEvidence:        record.SafeEvidence,
	}
}
