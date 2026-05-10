package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	slackconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/slack"
	telegramconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/telegram"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type LiveValidationAttemptResource = livevalidation.Attempt
type LiveValidationAttemptListResponse struct {
	TenantID         string                          `json:"tenantId,omitempty"`
	EnvironmentScope string                          `json:"environmentScope,omitempty"`
	Items            []LiveValidationAttemptResource `json:"items"`
}

type LiveValidationSupportMatrixResponse struct {
	EnvironmentScope string                     `json:"environmentScope,omitempty"`
	Version          string                     `json:"version"`
	Items            []livevalidation.MatrixRow `json:"items"`
}

type LiveValidationDiscordConformanceResponse struct {
	TenantID    string                             `json:"tenantId"`
	ConnectorID string                             `json:"connectorId"`
	Items       []baseconnectors.ConformanceResult `json:"items"`
}

type CreateLiveValidationRequest = livevalidation.StartInput
type CreateLiveValidationResponse = livevalidation.StartResult
type ResolveLiveValidationReconciliationRequest struct {
	Resolution   livevalidation.ReconciliationResolutionValue `json:"resolution"`
	Reason       string                                       `json:"reason"`
	EvidenceRefs []string                                     `json:"evidenceRefs,omitempty"`
}
type UpdateLiveValidationKillSwitchRequest struct {
	Scope     livevalidation.KillSwitchScope `json:"scope"`
	TenantID  string                         `json:"tenantId,omitempty"`
	Enabled   bool                           `json:"enabled"`
	Reason    string                         `json:"reason"`
	ExpiresAt *time.Time                     `json:"expiresAt,omitempty"`
}

type RecordTelegramSmokeRequest struct {
	ConnectorID    string            `json:"connectorId"`
	Status         string            `json:"status"`
	CredentialMode string            `json:"credentialMode"`
	Owner          string            `json:"owner,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	RemainingRisk  string            `json:"remainingRisk,omitempty"`
	ValidatedAt    *time.Time        `json:"validatedAt,omitempty"`
	SafeEvidence   map[string]string `json:"safeEvidence,omitempty"`
}

type RecordSlackSmokeRequest struct {
	ConnectorID        string            `json:"connectorId"`
	WorkspaceBindingID string            `json:"workspaceBindingId,omitempty"`
	Status             string            `json:"status"`
	AuthorizationMode  string            `json:"authorizationMode"`
	Owner              string            `json:"owner,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	RemainingRisk      string            `json:"remainingRisk,omitempty"`
	ValidatedAt        *time.Time        `json:"validatedAt,omitempty"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type RecordMatrixSmokeRequest struct {
	ConnectorID         string            `json:"connectorId"`
	HomeserverBindingID string            `json:"homeserverBindingId,omitempty"`
	Status              string            `json:"status"`
	AuthorizationMode   string            `json:"authorizationMode"`
	Owner               string            `json:"owner,omitempty"`
	Reason              string            `json:"reason,omitempty"`
	RemainingRisk       string            `json:"remainingRisk,omitempty"`
	ValidatedAt         *time.Time        `json:"validatedAt,omitempty"`
	SafeEvidence        map[string]string `json:"safeEvidence,omitempty"`
}

type matrixSmokeExecutor interface {
	ExecuteMatrixSmoke(ctx context.Context, tenantID string, input RecordMatrixSmokeRequest) (store.MatrixSmokeEvidenceRecord, error)
}

func handleLiveValidationRoutes(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, matrixExecutors ...matrixSmokeExecutor) {
	var matrixExecutor matrixSmokeExecutor
	if len(matrixExecutors) > 0 {
		matrixExecutor = matrixExecutors[0]
	}
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "live validation manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/live-validations")
	path = strings.Trim(path, "/")
	if path == "" {
		handleLiveValidationCollection(manager, eventBus, sqliteStore, w, r)
		return
	}
	if path == "support-matrix" {
		handleLiveValidationSupportMatrix(manager, w, r)
		return
	}
	if path == "kill-switches" {
		handleLiveValidationKillSwitches(manager, eventBus, sqliteStore, w, r)
		return
	}
	if path == "discord-smoke" {
		handleLiveValidationDiscordSmoke(sqliteStore, w, r)
		return
	}
	if path == "discord-conformance" {
		handleLiveValidationDiscordConformance(sqliteStore, w, r)
		return
	}
	if path == "telegram-smoke" {
		handleLiveValidationTelegramSmoke(sqliteStore, w, r)
		return
	}
	if path == "telegram-conformance" {
		handleLiveValidationConnectorConformance(sqliteStore, w, r)
		return
	}
	if path == "slack-conformance" {
		handleLiveValidationConnectorConformance(sqliteStore, w, r)
		return
	}
	if path == "slack-smoke" {
		handleLiveValidationSlackSmoke(sqliteStore, w, r)
		return
	}
	if path == "matrix-conformance" {
		handleLiveValidationConnectorConformance(sqliteStore, w, r)
		return
	}
	if path == "matrix-smoke" {
		handleLiveValidationMatrixSmoke(sqliteStore, matrixExecutor, w, r)
		return
	}
	handleLiveValidationItem(manager, eventBus, sqliteStore, path, w, r)
}

func handleLiveValidationDiscordConformance(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	handleLiveValidationConnectorConformance(sqliteStore, w, r)
}

func handleLiveValidationConnectorConformance(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "connector conformance store is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if _, reason := requireHostedCredentialReadAny(r, identity.PermissionConnectorsManage); reason != "" {
		writeCredentialDenial(w, http.StatusForbidden, reason)
		return
	}
	connectorID := strings.TrimSpace(r.URL.Query().Get("connectorId"))
	if connectorID == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	items, err := sqliteStore.ListConnectorConformanceResults(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(items) == 0 {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, LiveValidationDiscordConformanceResponse{
		TenantID:    tenantContext.TenantID,
		ConnectorID: connectorID,
		Items:       items,
	})
}

func handleLiveValidationDiscordSmoke(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "connector smoke evidence store is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if _, reason := requireHostedCredentialReadAny(r, identity.PermissionConnectorsManage); reason != "" {
		writeCredentialDenial(w, http.StatusForbidden, reason)
		return
	}
	connectorID := strings.TrimSpace(r.URL.Query().Get("connectorId"))
	if connectorID == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	evidence, found, err := sqliteStore.LatestDiscordSmokeEvidence(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func handleLiveValidationTelegramSmoke(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "connector smoke evidence store is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		recordTelegramSmokeEvidence(sqliteStore, w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if _, reason := requireHostedCredentialReadAny(r, identity.PermissionConnectorsManage); reason != "" {
		writeCredentialDenial(w, http.StatusForbidden, reason)
		return
	}
	connectorID := strings.TrimSpace(r.URL.Query().Get("connectorId"))
	if connectorID == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	evidence, found, err := sqliteStore.LatestTelegramSmokeEvidence(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func handleLiveValidationSlackSmoke(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "connector smoke evidence store is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		recordSlackSmokeEvidence(sqliteStore, w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if _, reason := requireHostedCredentialReadAny(r, identity.PermissionConnectorsManage); reason != "" {
		writeCredentialDenial(w, http.StatusForbidden, reason)
		return
	}
	connectorID := strings.TrimSpace(r.URL.Query().Get("connectorId"))
	if connectorID == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	evidence, found, err := sqliteStore.LatestSlackSmokeEvidence(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, projectSlackSmokeEvidenceResource(evidence))
}

func handleLiveValidationMatrixSmoke(sqliteStore *store.SQLiteStore, executor matrixSmokeExecutor, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "connector smoke evidence store is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		recordMatrixSmokeEvidence(sqliteStore, executor, w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if _, reason := requireHostedCredentialReadAny(r, identity.PermissionConnectorsManage); reason != "" {
		writeCredentialDenial(w, http.StatusForbidden, reason)
		return
	}
	connectorID := strings.TrimSpace(r.URL.Query().Get("connectorId"))
	if connectorID == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	evidence, found, err := sqliteStore.LatestMatrixSmokeEvidence(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func recordMatrixSmokeEvidence(sqliteStore *store.SQLiteStore, executor matrixSmokeExecutor, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionLiveValidationExecute) {
		writeCredentialDenial(w, http.StatusForbidden, "live_validation_execute_required")
		return
	}
	var input RecordMatrixSmokeRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		input.ConnectorID = strings.TrimSpace(r.URL.Query().Get("connectorId"))
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	var record store.MatrixSmokeEvidenceRecord
	var err error
	if matrixconnector.SmokeAuthorizationMode(strings.TrimSpace(input.AuthorizationMode)) == matrixconnector.SmokeAuthorizationSafeLive {
		if executor == nil {
			writeError(w, http.StatusBadRequest, "matrix safe-live smoke executor is not configured")
			return
		}
		record, err = executor.ExecuteMatrixSmoke(r.Context(), tenantContext.TenantID, input)
	} else {
		record, err = matrixSmokeRecordFromRequest(tenantContext.TenantID, input)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.SaveMatrixSmokeEvidence(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func matrixSmokeRecordFromRequest(tenantID string, input RecordMatrixSmokeRequest) (store.MatrixSmokeEvidenceRecord, error) {
	mode := matrixconnector.SmokeAuthorizationMode(strings.TrimSpace(input.AuthorizationMode))
	status := matrixconnector.SmokeStatus(strings.TrimSpace(input.Status))
	if status == "" {
		status = matrixconnector.SmokeSkipped
	}
	if mode == "" {
		mode = matrixconnector.SmokeAuthorizationUnavailable
	}
	switch status {
	case matrixconnector.SmokeSkipped:
		if mode != matrixconnector.SmokeAuthorizationUnavailable {
			return store.MatrixSmokeEvidenceRecord{}, errors.New("skipped Matrix smoke must use unavailable authorization mode")
		}
	case matrixconnector.SmokePassed, matrixconnector.SmokeFailed:
		if mode != matrixconnector.SmokeAuthorizationFakeMatrix && mode != matrixconnector.SmokeAuthorizationSafeLive {
			return store.MatrixSmokeEvidenceRecord{}, errors.New("passed or failed Matrix smoke requires fake_matrix or safe_live authorization mode")
		}
	default:
		return store.MatrixSmokeEvidenceRecord{}, errors.New("status must be passed, failed, or skipped")
	}
	validatedAt := time.Now().UTC()
	if input.ValidatedAt != nil {
		validatedAt = input.ValidatedAt.UTC()
	}
	connectorID := strings.TrimSpace(input.ConnectorID)
	bindingID := firstNonEmptyString(input.HomeserverBindingID, "matrix_homeserver_"+connectorID)
	owner := firstNonEmptyString(input.Owner, "operator")
	reason := firstNonEmptyString(input.Reason, "safe_matrix_authorization_unavailable")
	remainingRisk := input.RemainingRisk
	if strings.TrimSpace(remainingRisk) == "" && status == matrixconnector.SmokeSkipped {
		remainingRisk = "No live Matrix hosted smoke was run; release review must consume this structured skip."
	}
	return store.MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     "matrix_smoke_" + connectorID,
		TenantID:            tenantID,
		ConnectorID:         connectorID,
		HomeserverBindingID: bindingID,
		Status:              string(status),
		AuthorizationMode:   string(mode),
		Owner:               owner,
		Reason:              reason,
		RemainingRisk:       remainingRisk,
		ValidatedAt:         validatedAt,
		RetentionExpiresAt:  validatedAt.Add(90 * 24 * time.Hour),
		RedactionStatus:     string(baseconnectors.RedactionStatusRedacted),
		SafeEvidence:        input.SafeEvidence,
	}, nil
}

func recordSlackSmokeEvidence(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionLiveValidationExecute) {
		writeCredentialDenial(w, http.StatusForbidden, "live_validation_execute_required")
		return
	}
	var input RecordSlackSmokeRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		input.ConnectorID = strings.TrimSpace(r.URL.Query().Get("connectorId"))
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	smokeInput, err := slackSmokeInputFromRequest(tenantContext.TenantID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	evidence := slackconnector.BuildSmokeEvidence(smokeInput)
	record := store.SlackSmokeEvidenceRecord{
		SmokeEvidenceID:    evidence.SmokeEvidenceID,
		TenantID:           evidence.TenantID,
		ConnectorID:        evidence.ConnectorID,
		WorkspaceBindingID: evidence.WorkspaceBindingID,
		Status:             string(evidence.Status),
		AuthorizationMode:  string(evidence.AuthorizationMode),
		Owner:              evidence.Owner,
		Reason:             evidence.Reason,
		RemainingRisk:      evidence.RemainingRisk,
		ValidatedAt:        evidence.ValidatedAt,
		RetentionExpiresAt: evidence.RetentionExpiresAt,
		RedactionStatus:    string(evidence.RedactionStatus),
		SafeEvidence:       evidence.SafeEvidence,
	}
	if err := sqliteStore.SaveSlackSmokeEvidence(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, projectSlackSmokeEvidenceResource(record))
}

func slackSmokeInputFromRequest(tenantID string, input RecordSlackSmokeRequest) (slackconnector.SmokeInput, error) {
	mode := slackconnector.AuthorizationMode(strings.TrimSpace(input.AuthorizationMode))
	status := slackconnector.SmokeStatus(strings.TrimSpace(input.Status))
	if status == "" {
		status = slackconnector.SmokeSkipped
	}
	if mode == "" {
		mode = slackconnector.AuthorizationModeUnavailable
	}
	switch status {
	case slackconnector.SmokeSkipped:
		if mode != slackconnector.AuthorizationModeUnavailable {
			return slackconnector.SmokeInput{}, errors.New("skipped Slack smoke must use unavailable authorization mode")
		}
	case slackconnector.SmokePassed, slackconnector.SmokeFailed:
		if mode != slackconnector.AuthorizationModeFakeOAuth && mode != slackconnector.AuthorizationModeSafeLive {
			return slackconnector.SmokeInput{}, errors.New("passed or failed Slack smoke requires fake_oauth or safe_live authorization mode")
		}
	default:
		return slackconnector.SmokeInput{}, errors.New("status must be passed, failed, or skipped")
	}
	validatedAt := time.Time{}
	if input.ValidatedAt != nil {
		validatedAt = *input.ValidatedAt
	}
	return slackconnector.SmokeInput{
		TenantID:           tenantID,
		ConnectorID:        input.ConnectorID,
		WorkspaceBindingID: input.WorkspaceBindingID,
		SafeLiveApproved:   mode == slackconnector.AuthorizationModeSafeLive,
		FakeOAuth:          mode == slackconnector.AuthorizationModeFakeOAuth,
		Passed:             status == slackconnector.SmokePassed,
		Owner:              input.Owner,
		Reason:             input.Reason,
		RemainingRisk:      input.RemainingRisk,
		ValidatedAt:        validatedAt,
		SafeEvidence:       input.SafeEvidence,
	}, nil
}

func recordTelegramSmokeEvidence(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionLiveValidationExecute) {
		writeCredentialDenial(w, http.StatusForbidden, "live_validation_execute_required")
		return
	}
	var input RecordTelegramSmokeRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		input.ConnectorID = strings.TrimSpace(r.URL.Query().Get("connectorId"))
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		writeError(w, http.StatusBadRequest, "connectorId is required")
		return
	}
	smokeInput, err := telegramSmokeInputFromRequest(tenantContext.TenantID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	evidence := telegramconnector.BuildSmokeEvidence(smokeInput)
	record := store.TelegramSmokeEvidenceRecord{
		SmokeEvidenceID:    evidence.SmokeEvidenceID,
		TenantID:           evidence.TenantID,
		ConnectorID:        evidence.ConnectorID,
		Status:             string(evidence.Status),
		CredentialMode:     string(evidence.CredentialMode),
		Owner:              evidence.Owner,
		Reason:             evidence.Reason,
		RemainingRisk:      evidence.RemainingRisk,
		ValidatedAt:        evidence.ValidatedAt,
		RetentionExpiresAt: evidence.RetentionExpiresAt,
		RedactionStatus:    string(evidence.RedactionStatus),
		SafeEvidence:       evidence.SafeEvidence,
	}
	if err := sqliteStore.SaveTelegramSmokeEvidence(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func telegramSmokeInputFromRequest(tenantID string, input RecordTelegramSmokeRequest) (telegramconnector.SmokeInput, error) {
	mode := telegramconnector.CredentialMode(strings.TrimSpace(input.CredentialMode))
	status := telegramconnector.SmokeStatus(strings.TrimSpace(input.Status))
	if status == "" {
		status = telegramconnector.SmokeSkipped
	}
	if mode == "" {
		mode = telegramconnector.CredentialModeUnavailable
	}
	switch status {
	case telegramconnector.SmokeSkipped:
		if mode != telegramconnector.CredentialModeUnavailable {
			return telegramconnector.SmokeInput{}, errors.New("skipped Telegram smoke must use unavailable credential mode")
		}
	case telegramconnector.SmokePassed, telegramconnector.SmokeFailed:
		if mode != telegramconnector.CredentialModeFake && mode != telegramconnector.CredentialModeSafeLive {
			return telegramconnector.SmokeInput{}, errors.New("passed or failed Telegram smoke requires fake or safe_live credential mode")
		}
	default:
		return telegramconnector.SmokeInput{}, errors.New("status must be passed, failed, or skipped")
	}
	validatedAt := time.Time{}
	if input.ValidatedAt != nil {
		validatedAt = *input.ValidatedAt
	}
	return telegramconnector.SmokeInput{
		TenantID:       tenantID,
		ConnectorID:    input.ConnectorID,
		SafeCredential: mode == telegramconnector.CredentialModeSafeLive,
		FakeSafePass:   mode == telegramconnector.CredentialModeFake,
		Passed:         status == telegramconnector.SmokePassed,
		Owner:          input.Owner,
		Reason:         input.Reason,
		RemainingRisk:  input.RemainingRisk,
		ValidatedAt:    validatedAt,
		SafeEvidence:   input.SafeEvidence,
	}, nil
}

func handleLiveValidationKillSwitches(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tenantID := r.URL.Query().Get("tenantId")
		if tenantID == "" {
			if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
				tenantID = tenantContext.TenantID
			}
		}
		items, err := manager.ListKillSwitches(r.Context(), livevalidation.KillSwitchFilter{
			TenantID: tenantID,
			Scope:    livevalidation.KillSwitchScope(r.URL.Query().Get("scope")),
			Limit:    queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenantId": tenantID, "items": items})
	case http.MethodPost:
		var input UpdateLiveValidationKillSwitchRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := manager.SetKillSwitch(r.Context(), livevalidation.KillSwitch{
			Scope:     input.Scope,
			TenantID:  input.TenantID,
			Enabled:   input.Enabled,
			Reason:    input.Reason,
			ExpiresAt: input.ExpiresAt,
		})
		if err != nil {
			if errors.Is(err, livevalidation.ErrKillSwitchPermissionDenied) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationAttemptEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationKillSwitchChangedName, livevalidation.Attempt{TenantID: item.TenantID, ValidationID: item.KillSwitchID, Status: livevalidation.AttemptStatusAborted, UpdatedAt: item.ChangedAt}, nil)
		writeJSON(w, http.StatusOK, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleLiveValidationSupportMatrix(manager *livevalidation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	matrix, err := manager.SupportMatrix()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, LiveValidationSupportMatrixResponse{
		EnvironmentScope: manager.EnvironmentScope(),
		Version:          "v1",
		Items:            matrix.Rows(),
	})
}

func handleLiveValidationCollection(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input CreateLiveValidationRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := manager.Start(r.Context(), input)
		if err != nil {
			if errors.Is(err, livevalidation.ErrLiveValidationDisabled) {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			if errors.Is(err, livevalidation.ErrLiveValidationBlocked) {
				publishLiveValidationStartEvent(r.Context(), eventBus, sqliteStore, result)
				recordLiveValidationAudit(r.Context(), sqliteStore, result, identity.AuditOutcomeDenied)
				writeJSON(w, http.StatusConflict, result)
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationStartEvent(r.Context(), eventBus, sqliteStore, result)
		recordLiveValidationAudit(r.Context(), sqliteStore, result, identity.AuditOutcomeSucceeded)
		writeJSON(w, http.StatusAccepted, result)
	case http.MethodGet:
		tenantID := ""
		if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
			tenantID = tenantContext.TenantID
		}
		items, err := manager.ListAttempts(r.Context(), livevalidation.AttemptFilter{
			TenantID:    tenantID,
			CandidateID: r.URL.Query().Get("candidateId"),
			Status:      livevalidation.AttemptStatus(r.URL.Query().Get("status")),
			Limit:       queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, LiveValidationAttemptListResponse{TenantID: tenantID, EnvironmentScope: manager.EnvironmentScope(), Items: items})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleLiveValidationItem(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, ok, err := manager.GetAttempt(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "live validation not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "ledger" && r.Method == http.MethodGet {
		tenantID := ""
		if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
			tenantID = tenantContext.TenantID
		}
		items, err := manager.ListLedgerEntries(r.Context(), livevalidation.LedgerFilter{
			TenantID:     tenantID,
			ValidationID: parts[0],
			ToolClass:    livevalidation.ToolClass(r.URL.Query().Get("toolClass")),
			Outcome:      livevalidation.LedgerOutcome(r.URL.Query().Get("outcome")),
			Limit:        queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"validationId": parts[0], "tenantId": tenantID, "items": items})
		return
	}
	if len(parts) == 2 && parts[1] == "abort" && r.Method == http.MethodPost {
		item, err := manager.Abort(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationAttemptEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationAbortedName, item, nil)
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "retention" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, manager.DefaultRetentionPolicy(r.Context()))
		return
	}
	if len(parts) == 2 && parts[1] == "compare" && r.Method == http.MethodPost {
		comparison, err := manager.CreateComparison(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if eventBus != nil {
			_, _ = publishEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationComparisonEvent(comparison))
		}
		writeJSON(w, http.StatusAccepted, comparison)
		return
	}
	if len(parts) == 4 && parts[1] == "reconciliations" && parts[3] == "resolve" && r.Method == http.MethodPost {
		var input ResolveLiveValidationReconciliationRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		resolution, err := manager.ResolveReconciliation(r.Context(), livevalidation.ReconciliationResolution{
			AmbiguousCommitID: parts[2],
			Resolution:        input.Resolution,
			Reason:            input.Reason,
			EvidenceRefs:      input.EvidenceRefs,
		})
		if err != nil {
			if errors.Is(err, livevalidation.ErrReconciliationPermissionDenied) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if eventBus != nil {
			_, _ = publishEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationReconciliationEvent(resolution))
		}
		writeJSON(w, http.StatusOK, resolution)
		return
	}
	writeError(w, http.StatusNotFound, "live validation route not found")
}

func publishLiveValidationStartEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, result livevalidation.StartResult) {
	switch result.Attempt.Status {
	case livevalidation.AttemptStatusBlocked:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationBlockedName, result.Attempt, result.Denials)
	case livevalidation.AttemptStatusAwaitingApproval:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationAwaitingApprovalName, result.Attempt, nil)
	default:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationStartedName, result.Attempt, nil)
	}
}

func publishLiveValidationAttemptEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, attempt livevalidation.Attempt, denials []livevalidation.Denial) {
	if eventBus == nil {
		return
	}
	_, _ = publishEvent(ctx, eventBus, sqliteStore, events.LiveValidationAttemptEvent(name, attempt, denials))
}

func recordLiveValidationAudit(ctx context.Context, sqliteStore *store.SQLiteStore, result livevalidation.StartResult, outcome string) {
	if sqliteStore == nil || result.Attempt.TenantID == "" {
		return
	}
	reasonCode := "live_validation_started"
	if outcome == identity.AuditOutcomeDenied && len(result.Denials) > 0 {
		reasonCode = result.Denials[0].ReasonCode
	}
	_, _ = sqliteStore.AppendTenantAuditEvent(ctx, audit.BuildLiveValidationAuditEvent(audit.LiveValidationAuditInput{
		Attempt:    result.Attempt,
		Denials:    result.Denials,
		Action:     "live_validation.start",
		Outcome:    outcome,
		ReasonCode: reasonCode,
		CreatedAt:  result.Attempt.UpdatedAt,
	}))
}
