package api

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	telegramconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/telegram"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type setupWizardDiagnosticProbe struct {
	Default  setupwizard.DefaultDiagnosticProbe
	Telegram *telegramSetupWizardIntegration
	Matrix   *matrixSetupWizardIntegration
}

func (p setupWizardDiagnosticProbe) ProbeSetup(ctx context.Context, session setupwizard.SetupSession, operation setupwizard.SetupOperation) (setupwizard.SetupDiagnosticProbeResult, error) {
	return p.Default.ProbeSetup(ctx, session, operation)
}

func (p setupWizardDiagnosticProbe) ProbeSubmittedSecret(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) (setupwizard.SetupDiagnosticProbeResult, error) {
	if session.TargetID == setupwizard.TargetTelegramConnector && p.Telegram != nil {
		return p.Telegram.ProbeSubmittedSecret(ctx, session, input)
	}
	if session.TargetID == setupwizard.TargetMatrixConnector && p.Matrix != nil {
		return p.Matrix.ProbeSubmittedSecret(ctx, session, input)
	}
	return p.Default.ProbeSetup(ctx, session, setupwizard.OperationSubmitSecret)
}

type setupWizardSubmittedSecretRecorders []setupwizard.SubmittedSecretRecorder

func (r setupWizardSubmittedSecretRecorders) RecordSubmittedSecretSetup(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) error {
	for _, recorder := range r {
		if recorder == nil {
			continue
		}
		if err := recorder.RecordSubmittedSecretSetup(ctx, session, input); err != nil {
			return err
		}
	}
	return nil
}

type telegramSetupWizardIntegration struct {
	store *store.SQLiteStore
	cfg   config.TelegramConnectorConfig

	mu    sync.Mutex
	cache map[string]telegramSetupProbeRecord
}

type telegramSetupProbeRecord struct {
	Binding telegramconnector.AccountBinding
	Reason  string
	State   setupwizard.SetupState
}

func newTelegramSetupWizardIntegration(sqliteStore *store.SQLiteStore, cfg config.TelegramConnectorConfig) *telegramSetupWizardIntegration {
	return &telegramSetupWizardIntegration{
		store: sqliteStore,
		cfg:   cfg,
		cache: map[string]telegramSetupProbeRecord{},
	}
}

func (i *telegramSetupWizardIntegration) ProbeSubmittedSecret(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) (setupwizard.SetupDiagnosticProbeResult, error) {
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
		DiagnosticResultID: "diag_" + telegramSetupID(session.SetupSessionID) + "_telegram_submitted_secret",
		DiagnosticRunID:    "diag_run_" + telegramSetupID(session.SetupSessionID) + "_telegram_submitted_secret",
		DiagnosticStage:    "credential_probe",
		DiagnosticSource:   setupwizard.DiagnosticSource{Kind: "telegram_bot_api", ID: firstNonEmptyString(i.cfg.ConnectorID, "telegram-main")},
	}, nil
}

func (i *telegramSetupWizardIntegration) RecordSubmittedSecretSetup(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) error {
	if session.TargetID != setupwizard.TargetTelegramConnector || i == nil || i.store == nil {
		return nil
	}
	i.mu.Lock()
	record, ok := i.cache[session.SetupSessionID]
	i.mu.Unlock()
	if !ok {
		record = i.validate(ctx, session, input)
	}
	now := time.Now().UTC()
	credential := telegramconnector.CredentialInvalid
	if strings.TrimSpace(record.Binding.ConnectorAccountID) != "" {
		credential = telegramconnector.CredentialValid
	}
	setup := telegramconnector.EvaluateHostedSetup(telegramconnector.HostedSetupInput{
		TenantID:       session.TenantID,
		ConnectorID:    firstNonEmptyString(i.cfg.ConnectorID, "telegram-main"),
		DisplayName:    firstNonEmptyString(i.cfg.DisplayName, "Telegram connector"),
		Credential:     credential,
		AccountBinding: record.Binding,
		Allowments:     telegramAllowmentsFromSetupRefs(session.TenantID, firstNonEmptyString(i.cfg.ConnectorID, "telegram-main"), session.ResourceRefs, now),
		GroupBehavior:  telegramconnector.GroupBehaviorMentionOrCommandRequired,
		StartedAt:      session.CreatedAt,
		ValidatedAt:    now,
	})
	stored := store.TelegramHostedSetupRecord{
		TenantID:           setup.TenantID,
		ConnectorID:        setup.ConnectorID,
		ConnectorKind:      setup.ConnectorKind,
		DisplayName:        setup.DisplayName,
		Status:             string(setup.Status),
		TerminalState:      string(setup.TerminalState),
		HostedReady:        setup.HostedReady,
		CredentialState:    string(setup.CredentialState),
		AllowmentState:     string(setup.AllowmentState),
		GroupBehavior:      string(setup.GroupBehavior),
		DeliveryEligible:   setup.DeliveryEligible,
		ReasonCode:         setup.ReasonCode,
		RedactionStatus:    string(setup.RedactionStatus),
		CreatedAt:          setup.CreatedAt,
		UpdatedAt:          setup.UpdatedAt,
		ValidatedAt:        setup.ValidatedAt,
		RetentionExpiresAt: setup.RetentionExpiresAt,
	}
	if strings.TrimSpace(setup.AccountBinding.ConnectorAccountID) != "" {
		stored.AccountBinding = &store.ConnectorAccountBindingSummary{
			TenantID:            setup.AccountBinding.TenantID,
			ConnectorID:         setup.AccountBinding.ConnectorID,
			ConnectorAccountID:  setup.AccountBinding.ConnectorAccountID,
			DisplayName:         setup.AccountBinding.ProviderAccountLabel,
			ProviderAccountHint: setup.AccountBinding.ProviderAccountLabel,
			RedactionStatus:     string(setup.AccountBinding.RedactionStatus),
			UpdatedAt:           setup.AccountBinding.ValidatedAt,
		}
	}
	if err := i.store.SaveTelegramHostedSetup(ctx, stored); err != nil {
		return err
	}
	for _, allowment := range setup.Allowments {
		if err := i.store.SaveTelegramAllowment(ctx, store.TelegramAllowmentRecord{
			TenantID:        allowment.TenantID,
			ConnectorID:     allowment.ConnectorID,
			AllowmentID:     allowment.AllowmentID,
			ScopeType:       string(allowment.ScopeType),
			ScopeID:         allowment.ScopeID,
			ProviderLabel:   allowment.ProviderLabel,
			Enabled:         allowment.Enabled,
			GroupGate:       string(allowment.GroupGate),
			ValidationState: string(allowment.ValidationState),
			ReasonCode:      allowment.ReasonCode,
			ValidatedAt:     allowment.ValidatedAt,
			RedactionStatus: string(allowment.RedactionStatus),
			SafeEvidence:    allowment.SafeEvidence,
		}); err != nil {
			return err
		}
	}
	if !setup.HostedReady {
		diagnostic, err := telegramconnector.BuildDiagnosticState(setup.TenantID, setup.ConnectorID, setup.AccountBinding.ConnectorAccountID, connectorReasonForTelegramSetup(setup.ReasonCode), map[string]string{
			"setupSessionId": session.SetupSessionID,
			"stage":          "submitted_secret",
		}, now)
		if err == nil {
			_ = i.store.SaveConnectorDiagnosticState(ctx, diagnostic)
		}
	}
	return nil
}

func (i *telegramSetupWizardIntegration) validate(ctx context.Context, session setupwizard.SetupSession, input setupwizard.SubmitSecretInput) telegramSetupProbeRecord {
	record := telegramSetupProbeRecord{Reason: setupwizard.ReasonTelegramAllowmentMissing, State: setupwizard.StateActionRequired}
	transport, err := telegramconnector.NewBotAPITransport(telegramconnector.BotAPITransportConfig{
		ConnectorID: firstNonEmptyString(i.cfg.ConnectorID, "telegram-main"),
		BotToken:    input.Value,
		BaseURL:     i.cfg.BotAPIBaseURL,
	})
	if err != nil {
		record.Reason = setupwizard.ReasonCredentialMissing
		return record
	}
	binding, err := transport.ValidateCredential(ctx)
	if err != nil {
		record.Reason = setupWizardReasonForTelegramError(err)
		return record
	}
	binding.TenantID = session.TenantID
	binding.ConnectorID = firstNonEmptyString(i.cfg.ConnectorID, "telegram-main")
	record.Binding = binding
	if !hasTelegramAllowmentValidation(input.ResourceRefs) && !hasTelegramAllowmentValidation(session.ResourceRefs) {
		record.Reason = setupwizard.ReasonTelegramAllowmentMissing
		return record
	}
	record.Reason = setupwizard.ReasonHealthy
	record.State = setupwizard.StateReady
	return record
}

func setupWizardReasonForTelegramError(err error) string {
	switch telegramconnector.DiagnosticReasonForError(err) {
	case baseconnectors.DiagnosticAuthMissing:
		return setupwizard.ReasonCredentialMissing
	case baseconnectors.DiagnosticRateLimited:
		return setupwizard.ReasonRateLimited
	case baseconnectors.DiagnosticNetworkFailed:
		return setupwizard.ReasonNetworkFailed
	case baseconnectors.DiagnosticProviderUnavailable:
		return setupwizard.ReasonProviderUnavailable
	default:
		return setupwizard.ReasonProviderUnavailable
	}
}

func hasTelegramAllowmentValidation(refs []setupwizard.ResourceRef) bool {
	for _, ref := range refs {
		if ref.Kind == "telegram_allowment_validation" && strings.TrimSpace(ref.ID) != "" {
			return true
		}
	}
	return false
}

func telegramAllowmentsFromSetupRefs(tenantID, connectorID string, refs []setupwizard.ResourceRef, now time.Time) []telegramconnector.AllowmentValidation {
	items := make([]telegramconnector.AllowmentValidation, 0)
	for _, ref := range refs {
		if ref.Kind != "telegram_allowment_validation" || strings.TrimSpace(ref.ID) == "" {
			continue
		}
		scopeType := telegramconnector.ScopeDirectChat
		scopeID := strings.TrimSpace(ref.ID)
		if prefix, value, ok := strings.Cut(scopeID, ":"); ok {
			scopeID = strings.TrimSpace(value)
			switch prefix {
			case "user":
				scopeType = telegramconnector.ScopeUser
			case "group":
				scopeType = telegramconnector.ScopeGroup
			default:
				scopeType = telegramconnector.ScopeDirectChat
			}
		}
		groupGate := telegramconnector.GroupGateNotApplicable
		if scopeType == telegramconnector.ScopeGroup {
			groupGate = telegramconnector.GroupGateMentionOrCommandRequired
		}
		items = append(items, telegramconnector.AllowmentValidation{
			TenantID:        tenantID,
			ConnectorID:     connectorID,
			AllowmentID:     "allow_" + telegramSetupID(scopeID),
			ScopeType:       scopeType,
			ScopeID:         scopeID,
			Enabled:         true,
			GroupGate:       groupGate,
			ValidationState: telegramconnector.AllowmentValid,
			ReasonCode:      setupwizard.ReasonHealthy,
			ValidatedAt:     now,
			RedactionStatus: baseconnectors.RedactionStatusRedacted,
			SafeEvidence:    map[string]string{"source": "setup_wizard"},
		})
	}
	return items
}

func connectorReasonForTelegramSetup(reason string) baseconnectors.DiagnosticReasonCode {
	switch reason {
	case setupwizard.ReasonCredentialMissing:
		return baseconnectors.DiagnosticAuthMissing
	case setupwizard.ReasonTelegramAllowmentMissing, setupwizard.ReasonTelegramAllowmentInvalid:
		return baseconnectors.DiagnosticBlockedRoute
	case setupwizard.ReasonRateLimited:
		return baseconnectors.DiagnosticRateLimited
	case setupwizard.ReasonNetworkFailed:
		return baseconnectors.DiagnosticNetworkFailed
	case setupwizard.ReasonProviderUnavailable:
		return baseconnectors.DiagnosticProviderUnavailable
	default:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	}
}

func telegramSetupID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", ":", "_", ".", "_")
	return replacer.Replace(value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
