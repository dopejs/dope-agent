package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type matrixSetupWizardIntegration struct {
	store *store.SQLiteStore
	cfg   config.MatrixConnectorConfig

	mu    sync.Mutex
	cache map[string]matrixSetupProbeRecord
}

type matrixSetupProbeRecord struct {
	Binding matrixconnector.HomeserverBinding
	Policy  matrixconnector.RoutePolicy
	Reason  string
	State   setupwizard.SetupState
}

func newMatrixSetupWizardIntegration(sqliteStore *store.SQLiteStore, cfg config.MatrixConnectorConfig) *matrixSetupWizardIntegration {
	return &matrixSetupWizardIntegration{
		store: sqliteStore,
		cfg:   cfg,
		cache: map[string]matrixSetupProbeRecord{},
	}
}

func (i *matrixSetupWizardIntegration) ProbeSubmittedSecret(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) (setupwizard.SetupDiagnosticProbeResult, error) {
	record := i.validate(ctx, session, input)
	i.mu.Lock()
	i.cache[session.SetupSessionID] = record
	i.mu.Unlock()
	state, owner, retry := setupwizard.ClassifyDiagnosticReason(record.Reason)
	return setupwizard.SetupDiagnosticProbeResult{
		State:              state,
		ReasonCode:         record.Reason,
		RetrySafety:        retry,
		RemediationOwner:   owner,
		DiagnosticResultID: "diag_" + telegramSetupID(session.SetupSessionID) + "_matrix_submitted_secret",
		DiagnosticRunID:    "diag_run_" + telegramSetupID(session.SetupSessionID) + "_matrix_submitted_secret",
		DiagnosticStage:    "credential_probe",
		DiagnosticSource:   setupwizard.DiagnosticSource{Kind: "matrix_homeserver", ID: firstNonEmptyString(i.cfg.HomeserverID, i.cfg.HomeserverURL, "matrix")},
	}, nil
}

func (i *matrixSetupWizardIntegration) RecordSubmittedSecretSetup(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) error {
	if i == nil || i.store == nil || session.TargetID != setupwizard.TargetMatrixConnector {
		return nil
	}
	i.mu.Lock()
	record, ok := i.cache[session.SetupSessionID]
	i.mu.Unlock()
	if !ok {
		record = i.validate(ctx, session, input)
	}
	now := time.Now().UTC()
	connectorID := firstNonEmptyString(i.cfg.ConnectorID, "matrix-main")
	credential := matrixconnector.BotCredentialInvalid
	if strings.TrimSpace(input.Value) != "" && record.Binding.AuthorizationState == matrixconnector.AuthorizationValid {
		credential = matrixconnector.BotCredentialValid
	}
	setup := matrixconnector.EvaluateHostedSetup(matrixconnector.HostedSetupInput{
		TenantID:           session.TenantID,
		ConnectorID:        connectorID,
		DisplayName:        firstNonEmptyString(i.cfg.DisplayName, "Matrix connector"),
		BotCredentialState: credential,
		HomeserverBinding:  record.Binding,
		RoutePolicy:        record.Policy,
		ProviderAvailable:  true,
		NetworkAvailable:   true,
		ConformancePassed:  true,
		StartedAt:          session.CreatedAt,
		ValidatedAt:        now,
	})
	hosted := matrixHostedSetupRecord(setup)
	if err := i.store.SaveMatrixHostedSetup(ctx, hosted); err != nil {
		return err
	}
	if hosted.RoutePolicy != nil {
		return i.store.SaveMatrixRoutePolicy(ctx, *hosted.RoutePolicy)
	}
	return nil
}

func (i *matrixSetupWizardIntegration) validate(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) matrixSetupProbeRecord {
	now := time.Now().UTC()
	connectorID := firstNonEmptyString(i.cfg.ConnectorID, "matrix-main")
	binding := matrixconnector.NormalizeHomeserverBinding(session.TenantID, connectorID, matrixconnector.HomeserverBinding{
		HomeserverBindingID: "matrix_homeserver_" + connectorID,
		HomeserverURL:       i.cfg.HomeserverURL,
		HomeserverName:      firstNonEmptyString(i.cfg.HomeserverID, i.cfg.HomeserverURL),
		BotUserID:           i.cfg.BotUserID,
		AuthorizationState:  matrixconnector.AuthorizationValid,
		CapabilityState:     matrixconnector.HomeserverCapabilityValid,
		ValidatedAt:         now,
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
		SafeEvidence:        map[string]string{"source": "setup_wizard"},
	})
	policy := matrixRoutePolicyFromSetupRefs(session.TenantID, connectorID, binding.HomeserverBindingID, i.cfg, append(session.ResourceRefs, input.ResourceRefs...), now)
	record := matrixSetupProbeRecord{Binding: binding, Policy: policy, Reason: setupwizard.ReasonHealthy, State: setupwizard.StateReady}
	if strings.TrimSpace(input.Value) == "" || strings.TrimSpace(i.cfg.HomeserverURL) == "" || strings.TrimSpace(i.cfg.BotUserID) == "" {
		record.Binding.AuthorizationState = matrixconnector.AuthorizationMissing
		record.Reason = setupwizard.ReasonCredentialMissing
		record.State = setupwizard.StateActionRequired
		return record
	}
	transport, err := matrixconnector.NewClientTransport(matrixconnector.ClientTransportConfig{
		ConnectorID:    connectorID,
		HomeserverURL:  i.cfg.HomeserverURL,
		BotAccessToken: input.Value,
	})
	if err != nil {
		record.Binding.AuthorizationState = matrixconnector.AuthorizationMissing
		record.Reason = setupwizard.ReasonCredentialMissing
		record.State = setupwizard.StateActionRequired
		return record
	}
	validatedBinding, err := transport.ValidateHomeserverBinding(ctx, binding)
	record.Binding = matrixconnector.NormalizeHomeserverBinding(session.TenantID, connectorID, validatedBinding)
	if err != nil {
		record.Reason = setupWizardReasonForMatrixValidationError(err, record.Binding)
		record.State, _, _ = setupwizard.ClassifyDiagnosticReason(record.Reason)
		return record
	}
	if !matrixconnector.HasReadyRoutePolicy(policy) {
		record.Reason = setupwizard.ReasonMatrixRoutePolicyMissing
		record.State = setupwizard.StateActionRequired
		return record
	}
	validatedPolicy, err := transport.ValidateRoutePolicy(ctx, policy)
	record.Policy = validatedPolicy
	if err != nil || !matrixconnector.HasReadyRoutePolicy(validatedPolicy) {
		record.Reason = setupwizard.ReasonMatrixRoutePolicyInvalid
		record.State = setupwizard.StateActionRequired
		return record
	}
	return record
}

func setupWizardReasonForMatrixValidationError(err error, binding matrixconnector.HomeserverBinding) string {
	if binding.AuthorizationState == matrixconnector.AuthorizationOwnershipMismatch {
		return setupwizard.ReasonMatrixOwnershipMismatch
	}
	if binding.AuthorizationState == matrixconnector.AuthorizationMissing || binding.AuthorizationState == matrixconnector.AuthorizationRevoked {
		return setupwizard.ReasonCredentialMissing
	}
	if binding.AuthorizationState == matrixconnector.AuthorizationNetworkFailed {
		return setupwizard.ReasonNetworkFailed
	}
	if binding.AuthorizationState == matrixconnector.AuthorizationProviderUnavailable {
		return setupwizard.ReasonProviderUnavailable
	}
	if binding.CapabilityState == matrixconnector.HomeserverCapabilityRateLimited {
		return setupwizard.ReasonRateLimited
	}
	var apiError matrixconnector.ClientAPIError
	if errors.As(err, &apiError) {
		switch {
		case apiError.StatusCode == http.StatusTooManyRequests:
			return setupwizard.ReasonRateLimited
		case apiError.StatusCode >= 500:
			return setupwizard.ReasonProviderUnavailable
		}
	}
	return setupwizard.ReasonProviderUnavailable
}

func matrixRoutePolicyFromSetupRefs(tenantID, connectorID, homeserverBindingID string, cfg config.MatrixConnectorConfig, refs []setupwizard.ResourceRef, now time.Time) matrixconnector.RoutePolicy {
	selectedRooms := make([]matrixconnector.ConversationRoute, 0)
	for _, id := range cfg.SelectedRoomIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		selectedRooms = append(selectedRooms, matrixSelectedRoomRoute(strings.TrimSpace(id), now))
	}
	allowedDirectUsers := append([]string(nil), cfg.AllowedDirectUserIDs...)
	for _, ref := range refs {
		if ref.Kind != "matrix_route_policy_validation" || strings.TrimSpace(ref.ID) == "" {
			continue
		}
		id := strings.TrimSpace(ref.ID)
		if prefix, value, ok := strings.Cut(id, ":"); ok {
			switch prefix {
			case "room":
				selectedRooms = append(selectedRooms, matrixSelectedRoomRoute(strings.TrimSpace(value), now))
			case "direct":
				allowedDirectUsers = append(allowedDirectUsers, strings.TrimSpace(value))
			}
		}
	}
	state := matrixconnector.RoutePolicyBlocked
	if len(selectedRooms) > 0 || len(allowedDirectUsers) > 0 {
		state = matrixconnector.RoutePolicyValid
	}
	return matrixconnector.NormalizeRoutePolicy(matrixconnector.RoutePolicy{
		TenantID:            tenantID,
		ConnectorID:         connectorID,
		HomeserverBindingID: homeserverBindingID,
		SelectedRooms:       dedupeMatrixRooms(selectedRooms),
		AllowedDirectUsers:  dedupeStrings(allowedDirectUsers),
		RoomInvocationGate:  "bot_mention_or_command_required",
		ConfiguredCommands:  append([]string(nil), cfg.ConfiguredCommands...),
		EncryptedRoomPolicy: "unsupported",
		ValidationState:     state,
		ReasonCode:          setupwizard.ReasonHealthy,
		ValidatedAt:         now,
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
		SafeEvidence:        map[string]string{"source": "setup_wizard"},
	}, now)
}

func matrixSelectedRoomRoute(id string, _ time.Time) matrixconnector.ConversationRoute {
	return matrixconnector.ConversationRoute{
		ConversationID:     id,
		ConversationType:   matrixconnector.ConversationRoom,
		RoomSelectionState: matrixconnector.RoomSelected,
		ValidationState:    matrixconnector.RoutePolicyValid,
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		SafeEvidence:       map[string]string{"source": "setup_wizard"},
	}
}

func matrixHostedSetupRecord(setup matrixconnector.HostedSetup) store.MatrixHostedSetupRecord {
	record := store.MatrixHostedSetupRecord{
		TenantID:            setup.TenantID,
		ConnectorID:         setup.ConnectorID,
		ConnectorKind:       setup.ConnectorKind,
		DisplayName:         setup.DisplayName,
		Status:              string(setup.Status),
		TerminalState:       string(setup.TerminalState),
		BotCredentialState:  string(setup.BotCredentialState),
		HomeserverState:     string(setup.HomeserverState),
		RoutePolicyState:    string(setup.RoutePolicyState),
		DeliveryEligible:    setup.DeliveryEligible,
		HomeserverBindingID: setup.HomeserverBindingID,
		ReasonCode:          setup.ReasonCode,
		RedactionStatus:     string(setup.RedactionStatus),
		CreatedAt:           setup.CreatedAt,
		UpdatedAt:           setup.UpdatedAt,
		ValidatedAt:         setup.ValidatedAt,
		RetentionExpiresAt:  setup.RetentionExpiresAt,
	}
	if strings.TrimSpace(setup.HomeserverBinding.HomeserverBindingID) != "" {
		record.HomeserverBinding = &store.MatrixHomeserverBindingRecord{
			HomeserverBindingID:       setup.HomeserverBinding.HomeserverBindingID,
			HomeserverURL:             setup.HomeserverBinding.HomeserverURL,
			HomeserverName:            setup.HomeserverBinding.HomeserverName,
			BotUserID:                 setup.HomeserverBinding.BotUserID,
			BotDeviceID:               setup.HomeserverBinding.BotDeviceID,
			AuthorizationState:        string(setup.HomeserverBinding.AuthorizationState),
			HomeserverCapabilityState: string(setup.HomeserverBinding.CapabilityState),
			ValidatedAt:               setup.HomeserverBinding.ValidatedAt,
			RedactionStatus:           string(setup.HomeserverBinding.RedactionStatus),
			SafeEvidence:              setup.HomeserverBinding.SafeEvidence,
		}
	}
	record.RoutePolicy = matrixRoutePolicyRecord(setup.RoutePolicy)
	return record
}

func matrixRoutePolicyRecord(policy matrixconnector.RoutePolicy) *store.MatrixRoutePolicyRecord {
	if policy.ConnectorID == "" {
		return nil
	}
	rooms := make([]store.MatrixConversationRouteRecord, 0, len(policy.SelectedRooms))
	for _, room := range policy.SelectedRooms {
		rooms = append(rooms, store.MatrixConversationRouteRecord{
			ConversationID:     room.ConversationID,
			ConversationType:   string(room.ConversationType),
			RoomSelectionState: string(room.RoomSelectionState),
			ValidationState:    string(room.ValidationState),
			ReasonCode:         room.ReasonCode,
			RedactionStatus:    string(room.RedactionStatus),
			SafeEvidence:       room.SafeEvidence,
		})
	}
	return &store.MatrixRoutePolicyRecord{
		TenantID:            policy.TenantID,
		ConnectorID:         policy.ConnectorID,
		HomeserverBindingID: policy.HomeserverBindingID,
		SelectedRooms:       rooms,
		AllowedDirectUsers:  append([]string(nil), policy.AllowedDirectUsers...),
		RoomInvocationGate:  policy.RoomInvocationGate,
		ConfiguredCommands:  append([]string(nil), policy.ConfiguredCommands...),
		EncryptedRoomPolicy: policy.EncryptedRoomPolicy,
		ValidationState:     string(policy.ValidationState),
		ReasonCode:          policy.ReasonCode,
		ValidatedAt:         policy.ValidatedAt,
		RedactionStatus:     string(policy.RedactionStatus),
		SafeEvidence:        policy.SafeEvidence,
	}
}

func dedupeMatrixRooms(rooms []matrixconnector.ConversationRoute) []matrixconnector.ConversationRoute {
	seen := map[string]bool{}
	out := make([]matrixconnector.ConversationRoute, 0, len(rooms))
	for _, room := range rooms {
		id := strings.TrimSpace(room.ConversationID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		room.ConversationID = id
		out = append(out, room)
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
