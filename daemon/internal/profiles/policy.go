package profiles

import (
	"errors"
	"path"
	"strings"
	"unicode"
)

var (
	ErrInvalidProfile        = errors.New("profile validation failed")
	ErrProfileNotActivatable = errors.New("profile is not activatable")
	ErrScopedBindingDeferred = errors.New("profile scoped binding is deferred to roadmap 58")
	ErrExplicitActorRequired = errors.New("profile mutation requires an explicit authorized actor")
)

type ValidationError struct {
	ReasonCode string
}

func (e ValidationError) Error() string {
	if strings.TrimSpace(e.ReasonCode) == "" {
		return ErrInvalidProfile.Error()
	}
	return ErrInvalidProfile.Error() + ": " + e.ReasonCode
}

func (e ValidationError) Unwrap() error {
	return ErrInvalidProfile
}

func InvalidProfileReason(reasonCode string) error {
	reasonCode = strings.TrimSpace(reasonCode)
	if reasonCode == "" {
		reasonCode = "profile_validation_failed"
	}
	return ValidationError{ReasonCode: reasonCode}
}

func ValidationReasonCode(err error) string {
	var validationErr ValidationError
	if errors.As(err, &validationErr) && strings.TrimSpace(validationErr.ReasonCode) != "" {
		return strings.TrimSpace(validationErr.ReasonCode)
	}
	if errors.Is(err, ErrInvalidProfile) {
		return "profile_validation_failed"
	}
	return ""
}

func ValidateMutation(input MutationInput) error {
	if strings.TrimSpace(input.DisplayName) == "" {
		return InvalidProfileReason("display_name_required")
	}
	if len(strings.TrimSpace(input.DisplayName)) > 120 {
		return InvalidProfileReason("display_name_too_long")
	}
	if overLimit(input.DisplayIdentity.Name, 120) || overLimit(input.DisplayIdentity.Description, 2000) || overLimit(input.DisplayIdentity.SafeSummary, 160) || overLimit(input.Persona.Tone, 80) || overLimit(input.Persona.Instructions, 4000) || overLimit(input.Persona.SafeSummary, 160) {
		return InvalidProfileReason("profile_field_too_long")
	}
	if containsUnsafe(input.DisplayName) || containsUnsafe(input.DisplayIdentity.Name) || containsUnsafe(input.DisplayIdentity.Description) || containsUnsafe(input.DisplayIdentity.SafeSummary) || containsUnsafe(input.Persona.Tone) || containsUnsafe(input.Persona.Instructions) || containsUnsafe(input.Persona.SafeSummary) {
		return InvalidProfileReason("unsafe_profile_content")
	}
	if err := validateProviderPreference(input.DefaultProviderPreference); err != nil {
		return err
	}
	if err := validateSafetyDefaults(input.SafetyDefaults); err != nil {
		return err
	}
	if len(input.OverlayReferences) > 20 {
		return InvalidProfileReason("too_many_overlay_references")
	}
	if len(input.LegacyMappingEvidence) > 20 {
		return InvalidProfileReason("too_many_legacy_mapping_records")
	}
	for _, evidence := range input.LegacyMappingEvidence {
		if err := validateLegacyMappingEvidence(evidence); err != nil {
			return err
		}
	}
	for _, overlay := range input.OverlayReferences {
		if err := validateOverlayInput(overlay); err != nil {
			return err
		}
	}
	return nil
}

func validateLegacyMappingEvidence(evidence LegacyMappingEvidence) error {
	if overLimit(evidence.SourceKind, 80) || overLimit(evidence.ReasonCode, 160) || overLimit(evidence.SafeSummary, 160) {
		return InvalidProfileReason("legacy_mapping_too_long")
	}
	if containsUnsafe(evidence.SourceKind) || containsUnsafe(evidence.ReasonCode) || containsUnsafe(evidence.SafeSummary) {
		return InvalidProfileReason("unsafe_legacy_mapping")
	}
	if !identifierLike(evidence.SourceKind) || !identifierLike(evidence.ReasonCode) {
		return InvalidProfileReason("legacy_mapping_malformed")
	}
	if !validValidationState(evidence.MappingState) {
		return InvalidProfileReason("legacy_mapping_state_invalid")
	}
	switch evidence.RedactionStatus {
	case "", RedactionRedacted, RedactionSuppressed:
		return nil
	default:
		return InvalidProfileReason("legacy_mapping_redaction_invalid")
	}
}

func validateProviderPreference(preference DefaultProviderPreference) error {
	if overLimit(preference.ProviderID, 160) || overLimit(preference.Model, 160) || overLimit(preference.ReasoningLevel, 40) || overLimit(preference.FailureReasonCode, 160) {
		return InvalidProfileReason("provider_preference_too_long")
	}
	if containsUnsafe(preference.ProviderID) || containsUnsafe(preference.Model) || containsUnsafe(preference.ReasoningLevel) || containsUnsafe(preference.FailureReasonCode) {
		return InvalidProfileReason("unsafe_provider_preference")
	}
	if !identifierLike(preference.ProviderID) || !identifierLike(preference.Model) {
		return InvalidProfileReason("provider_preference_malformed")
	}
	switch strings.TrimSpace(preference.ReasoningLevel) {
	case "", "low", "medium", "high", "xhigh":
	default:
		return InvalidProfileReason("reasoning_level_unsupported")
	}
	if !validValidationState(preference.ValidationState) {
		return InvalidProfileReason("provider_validation_state_unknown")
	}
	if invalidValidationState(preference.ValidationState) || strings.TrimSpace(preference.FailureReasonCode) != "" {
		return InvalidProfileReason("provider_preference_not_allowed")
	}
	return nil
}

func validateSafetyDefaults(defaults SafetyDefaults) error {
	if overLimit(defaults.ApprovalPosture, 80) || overLimit(defaults.RiskTolerance, 80) || overLimit(defaults.FailureReasonCode, 160) {
		return InvalidProfileReason("safety_defaults_too_long")
	}
	if containsUnsafe(defaults.ApprovalPosture) || containsUnsafe(defaults.RiskTolerance) || containsUnsafe(defaults.FailureReasonCode) {
		return InvalidProfileReason("unsafe_safety_defaults")
	}
	switch strings.TrimSpace(defaults.ApprovalPosture) {
	case "", "ask", "ask_for_risky_changes", "always_ask", "manual", "auto_read_only":
	default:
		return InvalidProfileReason("approval_posture_unsupported")
	}
	switch strings.TrimSpace(defaults.RiskTolerance) {
	case "", "low", "medium", "high":
	default:
		return InvalidProfileReason("risk_tolerance_unsupported")
	}
	if !validValidationState(defaults.ValidationState) {
		return InvalidProfileReason("safety_validation_state_unknown")
	}
	if invalidValidationState(defaults.ValidationState) || strings.TrimSpace(defaults.FailureReasonCode) != "" {
		return InvalidProfileReason("safety_defaults_not_allowed")
	}
	return nil
}

func validateOverlayInput(overlay OverlayReferenceInput) error {
	if strings.TrimSpace(overlay.ReferenceURI) == "" || containsUnsafe(overlay.ReferenceURI) || overLimit(overlay.ReferenceURI, 1024) || overLimit(overlay.ReferenceKind, 80) {
		return InvalidProfileReason("overlay_reference_invalid")
	}
	if strings.HasPrefix(strings.TrimSpace(overlay.ReferenceURI), "/") || strings.Contains(path.Clean(strings.ReplaceAll(overlay.ReferenceURI, "\\", "/")), "../") || strings.Contains(strings.ReplaceAll(overlay.ReferenceURI, "\\", "/"), "/../") {
		return InvalidProfileReason("overlay_reference_out_of_scope")
	}
	if !identifierLike(overlay.ReferenceKind) {
		return InvalidProfileReason("overlay_reference_kind_malformed")
	}
	if scope := strings.TrimSpace(overlay.Scope); scope != "" && scope != "profile" {
		return ErrScopedBindingDeferred
	}
	return nil
}

func validValidationState(state OverlayValidationState) bool {
	switch state {
	case "", OverlayValid, OverlayPartial:
		return true
	case OverlayMissing, OverlayPermissionDenied, OverlayOutOfScope, OverlayTooLarge, OverlayUnsafeContent, OverlayRedactionFailed:
		return true
	default:
		return false
	}
}

func invalidValidationState(state OverlayValidationState) bool {
	switch state {
	case OverlayMissing, OverlayPermissionDenied, OverlayOutOfScope, OverlayTooLarge, OverlayUnsafeContent, OverlayRedactionFailed:
		return true
	default:
		return false
	}
}

func overLimit(value string, limit int) bool {
	return len(strings.TrimSpace(value)) > limit
}

func identifierLike(value string) bool {
	value = strings.TrimSpace(value)
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.', ':', '/', '@':
			continue
		default:
			return false
		}
	}
	return true
}

func CanActivate(profile AgentProfile, version ProfileVersion) error {
	if profile.Status == StatusArchived {
		return ErrProfileNotActivatable
	}
	if profile.Status == StatusDisabled {
		return ErrProfileNotActivatable
	}
	if version.RollbackEligibility != "" && version.RollbackEligibility != RollbackEligible {
		return ErrProfileNotActivatable
	}
	if RollbackEligibilityFor(profile, version) != RollbackEligible {
		return ErrProfileNotActivatable
	}
	return nil
}

func RollbackEligibilityFor(profile AgentProfile, version ProfileVersion) RollbackEligibility {
	if profile.Status == StatusArchived {
		return RollbackProfileArchived
	}
	if profile.Status == StatusDisabled {
		return RollbackProfileDisabled
	}
	if version.RedactionStatus == RedactionFailed {
		return RollbackRedactionFailed
	}
	if invalidValidationState(version.Snapshot.DefaultProviderPreference.ValidationState) || strings.TrimSpace(version.Snapshot.DefaultProviderPreference.FailureReasonCode) != "" {
		return RollbackInvalidProvider
	}
	if invalidValidationState(version.Snapshot.SafetyDefaults.ValidationState) || strings.TrimSpace(version.Snapshot.SafetyDefaults.FailureReasonCode) != "" {
		return RollbackPolicyBlocked
	}
	return RollbackEligible
}

func containsUnsafe(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "secret=") || strings.Contains(lower, "token=") || strings.Contains(lower, "api_key")
}
