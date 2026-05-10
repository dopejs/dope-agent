package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type matrixSmokeExecutorImpl struct {
	store   *store.SQLiteStore
	secrets *secrets.Manager
	cfg     config.MatrixConnectorConfig
}

func newMatrixSmokeExecutor(sqliteStore *store.SQLiteStore, secretManager *secrets.Manager, cfg config.MatrixConnectorConfig) *matrixSmokeExecutorImpl {
	return &matrixSmokeExecutorImpl{store: sqliteStore, secrets: secretManager, cfg: cfg}
}

func (e *matrixSmokeExecutorImpl) ExecuteMatrixSmoke(ctx context.Context, tenantID string, input RecordMatrixSmokeRequest) (store.MatrixSmokeEvidenceRecord, error) {
	if e == nil || e.store == nil {
		return store.MatrixSmokeEvidenceRecord{}, errors.New("matrix smoke executor is not configured")
	}
	connectorID := strings.TrimSpace(input.ConnectorID)
	setup, ok, err := e.store.GetMatrixHostedSetup(ctx, tenantID, connectorID)
	if err != nil {
		return store.MatrixSmokeEvidenceRecord{}, err
	}
	if !ok || setup.TerminalState != string(matrixconnector.TerminalReady) || !setup.DeliveryEligible {
		return store.MatrixSmokeEvidenceRecord{}, errors.New("matrix connector is not delivery eligible for safe-live smoke")
	}
	token, err := e.matrixAccessToken(ctx, tenantID, connectorID)
	if err != nil {
		return store.MatrixSmokeEvidenceRecord{}, err
	}
	homeserverURL := firstNonEmptyString(e.cfg.HomeserverURL)
	if setup.HomeserverBinding != nil {
		homeserverURL = firstNonEmptyString(setup.HomeserverBinding.HomeserverURL, homeserverURL)
	}
	transport, err := matrixconnector.NewClientTransport(matrixconnector.ClientTransportConfig{
		ConnectorID:    connectorID,
		HomeserverURL:  homeserverURL,
		BotAccessToken: token,
	})
	if err != nil {
		return store.MatrixSmokeEvidenceRecord{}, err
	}
	binding := matrixBindingFromSetup(setup, e.cfg)
	policy := matrixPolicyFromSetup(setup)
	evidence, err := matrixconnector.ExecuteSafeLiveSmoke(ctx, matrixconnector.SafeLiveSmokeInput{
		TenantID:    tenantID,
		ConnectorID: connectorID,
		Owner:       firstNonEmptyString(input.Owner, "operator"),
		Now:         valueOrZeroTime(input.ValidatedAt),
		Transport:   transport,
		Binding:     binding,
		RoutePolicy: policy,
		SmokeRoomID: firstMatrixSmokeRoomID(policy),
	})
	if err != nil {
		return store.MatrixSmokeEvidenceRecord{}, err
	}
	return store.MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     evidence.SmokeEvidenceID,
		TenantID:            evidence.TenantID,
		ConnectorID:         evidence.ConnectorID,
		HomeserverBindingID: evidence.HomeserverBindingID,
		Status:              string(evidence.Status),
		AuthorizationMode:   string(evidence.AuthorizationMode),
		Owner:               evidence.Owner,
		Reason:              evidence.Reason,
		RemainingRisk:       evidence.RemainingRisk,
		ValidatedAt:         evidence.ValidatedAt,
		RetentionExpiresAt:  evidence.RetentionExpiresAt,
		RedactionStatus:     string(evidence.RedactionStatus),
		SafeEvidence:        evidence.SafeEvidence,
	}, nil
}

func (e *matrixSmokeExecutorImpl) matrixAccessToken(ctx context.Context, tenantID, connectorID string) (string, error) {
	if strings.TrimSpace(e.cfg.BotAccessToken) != "" {
		return strings.TrimSpace(e.cfg.BotAccessToken), nil
	}
	if e.secrets == nil {
		return "", errors.New("matrix bot access token is not configured")
	}
	ref := "matrix/" + strings.TrimSpace(connectorID) + "/bot_access_token"
	resolved, err := e.secrets.Resolve(ctx, secrets.ResolveInput{TenantID: tenantID, SecretRef: ref})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved.Value) == "" {
		return "", errors.New("matrix bot access token is not configured")
	}
	return strings.TrimSpace(resolved.Value), nil
}

func matrixBindingFromSetup(setup store.MatrixHostedSetupRecord, cfg config.MatrixConnectorConfig) matrixconnector.HomeserverBinding {
	binding := matrixconnector.HomeserverBinding{
		TenantID:            setup.TenantID,
		ConnectorID:         setup.ConnectorID,
		HomeserverBindingID: setup.HomeserverBindingID,
		HomeserverURL:       cfg.HomeserverURL,
		BotUserID:           cfg.BotUserID,
		AuthorizationState:  matrixconnector.AuthorizationValid,
		CapabilityState:     matrixconnector.HomeserverCapabilityValid,
	}
	if setup.HomeserverBinding != nil {
		binding.HomeserverBindingID = firstNonEmptyString(setup.HomeserverBinding.HomeserverBindingID, binding.HomeserverBindingID)
		binding.HomeserverURL = firstNonEmptyString(setup.HomeserverBinding.HomeserverURL, binding.HomeserverURL)
		binding.HomeserverName = setup.HomeserverBinding.HomeserverName
		binding.BotUserID = firstNonEmptyString(setup.HomeserverBinding.BotUserID, binding.BotUserID)
	}
	return binding
}

func matrixPolicyFromSetup(setup store.MatrixHostedSetupRecord) matrixconnector.RoutePolicy {
	policy := matrixconnector.RoutePolicy{
		TenantID:            setup.TenantID,
		ConnectorID:         setup.ConnectorID,
		HomeserverBindingID: setup.HomeserverBindingID,
		ValidationState:     matrixconnector.RoutePolicyValid,
	}
	if setup.RoutePolicy == nil {
		return policy
	}
	policy.HomeserverBindingID = setup.RoutePolicy.HomeserverBindingID
	policy.AllowedDirectUsers = append([]string(nil), setup.RoutePolicy.AllowedDirectUsers...)
	policy.RoomInvocationGate = setup.RoutePolicy.RoomInvocationGate
	policy.ConfiguredCommands = append([]string(nil), setup.RoutePolicy.ConfiguredCommands...)
	policy.EncryptedRoomPolicy = setup.RoutePolicy.EncryptedRoomPolicy
	policy.ValidationState = matrixconnector.RoutePolicyState(setup.RoutePolicy.ValidationState)
	for _, room := range setup.RoutePolicy.SelectedRooms {
		policy.SelectedRooms = append(policy.SelectedRooms, matrixconnector.ConversationRoute{
			ConversationID:     room.ConversationID,
			ConversationType:   matrixconnector.ConversationType(room.ConversationType),
			RoomSelectionState: matrixconnector.RoomSelectionState(room.RoomSelectionState),
			ValidationState:    matrixconnector.RoutePolicyState(room.ValidationState),
		})
	}
	return policy
}

func firstMatrixSmokeRoomID(policy matrixconnector.RoutePolicy) string {
	for _, room := range policy.SelectedRooms {
		if strings.TrimSpace(room.ConversationID) != "" {
			return strings.TrimSpace(room.ConversationID)
		}
	}
	return ""
}

func valueOrZeroTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}
