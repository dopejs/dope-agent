package setupwizard

import (
	"context"
	"errors"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

type DefaultDiagnosticProbe struct {
	Secrets SecretManager
}

func (p DefaultDiagnosticProbe) ProbeSetup(ctx context.Context, session SetupSession, operation SetupOperation) (SetupDiagnosticProbeResult, error) {
	stage := diagnosticStageForOperation(operation)
	state := StateReady
	reason := ReasonHealthy
	owner := OwnerNoneRequired
	retry := RetryNoActionNeeded
	switch operation {
	case OperationSubmitSecret:
		if p.Secrets != nil {
			secretRef := resourceRefID(session.ResourceRefs, "tenant_secret")
			if secretRef == "" {
				state, owner, retry = ClassifyDiagnosticReason(ReasonCredentialMissing)
				reason = ReasonCredentialMissing
				break
			}
			if _, err := p.Secrets.Get(ctx, session.TenantID, secretRef); err != nil {
				state, owner, retry = ClassifyDiagnosticReason(ReasonCredentialMissing)
				reason = ReasonCredentialMissing
				if !errors.Is(err, secrets.ErrSecretNotFound) {
					state, owner, retry = ClassifyDiagnosticReason(ReasonProviderUnavailable)
					reason = ReasonProviderUnavailable
				}
			}
		}
	case OperationOAuthCallback:
		if resourceRefID(session.ResourceRefs, "provider_auth_state") == "" {
			state, owner, retry = ClassifyDiagnosticReason(ReasonTokenMissing)
			reason = ReasonTokenMissing
		}
	}
	return SetupDiagnosticProbeResult{
		State:              state,
		ReasonCode:         reason,
		RetrySafety:        retry,
		RemediationOwner:   owner,
		DiagnosticResultID: defaultDiagnosticResultID(session, operation),
		DiagnosticRunID:    defaultDiagnosticRunID(session, operation),
		DiagnosticStage:    stage,
		DiagnosticSource: DiagnosticSource{
			Kind: defaultDiagnosticSourceKind(session),
			ID:   session.TargetID,
		},
	}, nil
}

func resourceRefID(items []ResourceRef, kind string) string {
	for _, item := range items {
		if item.Kind == kind {
			return item.ID
		}
	}
	return ""
}

func (s *Service) probeReadiness(ctx context.Context, session SetupSession, operation SetupOperation) (SetupSession, SetupState, string, error) {
	if s.diagnostics == nil {
		return SetupSession{}, "", "", ErrDiagnosticLinkNeeded
	}
	result, err := s.diagnostics.ProbeSetup(ctx, session, operation)
	if err != nil {
		state, owner, retry := ClassifyDiagnosticReason(ReasonProviderUnavailable)
		result = SetupDiagnosticProbeResult{
			State:              state,
			ReasonCode:         ReasonProviderUnavailable,
			RemediationOwner:   owner,
			RetrySafety:        retry,
			DiagnosticResultID: defaultDiagnosticResultID(session, operation),
			DiagnosticRunID:    defaultDiagnosticRunID(session, operation),
			DiagnosticStage:    diagnosticStageForOperation(operation),
			DiagnosticSource:   DiagnosticSource{Kind: defaultDiagnosticSourceKind(session), ID: session.TargetID},
		}
	}
	result = normalizeProbeResult(session, operation, result)
	session.DiagnosticResultID = result.DiagnosticResultID
	session.DiagnosticRunID = result.DiagnosticRunID
	session.DiagnosticStage = result.DiagnosticStage
	session.DiagnosticSourceKind = result.DiagnosticSource.Kind
	session.DiagnosticSourceID = result.DiagnosticSource.ID
	session.DiagnosticAllowedUse = append([]string(nil), result.AllowedCapabilities...)
	session.AllowedCapabilities = append([]string(nil), result.AllowedCapabilities...)
	session.RemediationOwner = result.RemediationOwner
	return session, result.State, result.ReasonCode, nil
}

func normalizeProbeResult(session SetupSession, operation SetupOperation, result SetupDiagnosticProbeResult) SetupDiagnosticProbeResult {
	if result.ReasonCode == "" {
		if result.State == StateReady {
			result.ReasonCode = ReasonHealthy
		} else {
			result.ReasonCode = ReasonProviderUnavailable
		}
	}
	if result.State == "" || result.RemediationOwner == "" || result.RetrySafety == "" {
		state, owner, retry := ClassifyDiagnosticReason(result.ReasonCode)
		if result.State == "" {
			result.State = state
		}
		if result.RemediationOwner == "" {
			result.RemediationOwner = owner
		}
		if result.RetrySafety == "" {
			result.RetrySafety = retry
		}
	}
	if result.DiagnosticResultID == "" {
		result.DiagnosticResultID = defaultDiagnosticResultID(session, operation)
	}
	if result.DiagnosticRunID == "" {
		result.DiagnosticRunID = defaultDiagnosticRunID(session, operation)
	}
	if result.DiagnosticStage == "" {
		result.DiagnosticStage = diagnosticStageForOperation(operation)
	}
	if result.DiagnosticSource.Kind == "" {
		result.DiagnosticSource.Kind = defaultDiagnosticSourceKind(session)
	}
	if result.DiagnosticSource.ID == "" {
		result.DiagnosticSource.ID = session.TargetID
	}
	return result
}

func diagnosticStageForOperation(operation SetupOperation) string {
	switch operation {
	case OperationSubmitSecret:
		return "credential_probe"
	case OperationOAuthCallback:
		return "oauth_probe"
	case OperationDiagnosticProbe:
		return "diagnostic_probe"
	default:
		return string(operation)
	}
}

func defaultDiagnosticSourceKind(session SetupSession) string {
	switch session.TargetKind {
	case TargetKindProvider:
		return "provider_check"
	case TargetKindIntegration:
		return "integration_diagnostic"
	default:
		return "setup_probe"
	}
}

func defaultDiagnosticResultID(session SetupSession, operation SetupOperation) string {
	return fmt.Sprintf("diag_%s_%s", sanitizeID(session.SetupSessionID), sanitizeID(string(operation)))
}

func defaultDiagnosticRunID(session SetupSession, operation SetupOperation) string {
	return fmt.Sprintf("diag_run_%s_%s", sanitizeID(session.SetupSessionID), sanitizeID(string(operation)))
}
