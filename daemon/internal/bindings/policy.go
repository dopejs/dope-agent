package bindings

import (
	"errors"
	"strings"
	"unicode"
)

var (
	// ErrInvalidBinding is the sentinel wrapped by all binding validation failures.
	ErrInvalidBinding = errors.New("binding validation failed")
	// ErrExplicitActorRequired guards mutations that must name an authorized actor.
	ErrExplicitActorRequired = errors.New("binding mutation requires an explicit authorized actor")
)

// ValidationError carries a safe, user-visible reason code (FR-009).
type ValidationError struct {
	ReasonCode string
}

func (e ValidationError) Error() string {
	if strings.TrimSpace(e.ReasonCode) == "" {
		return ErrInvalidBinding.Error()
	}
	return ErrInvalidBinding.Error() + ": " + e.ReasonCode
}

func (e ValidationError) Unwrap() error { return ErrInvalidBinding }

// InvalidBindingReason builds a validation error with a safe reason code.
func InvalidBindingReason(reasonCode string) error {
	reasonCode = strings.TrimSpace(reasonCode)
	if reasonCode == "" {
		reasonCode = "binding_validation_failed"
	}
	return ValidationError{ReasonCode: reasonCode}
}

// ValidationReasonCode extracts the safe reason code from a validation error.
func ValidationReasonCode(err error) string {
	var validationErr ValidationError
	if errors.As(err, &validationErr) && strings.TrimSpace(validationErr.ReasonCode) != "" {
		return strings.TrimSpace(validationErr.ReasonCode)
	}
	if errors.Is(err, ErrInvalidBinding) {
		return "binding_validation_failed"
	}
	return ""
}

// WorkspaceMutationInput is the validated shape for creating/updating a workspace.
type WorkspaceMutationInput struct {
	DisplayName string
	Status      WorkspaceStatus
}

// ValidateWorkspaceMutation enforces safe, bounded workspace fields (FR-002, FR-032).
func ValidateWorkspaceMutation(input WorkspaceMutationInput) error {
	name := strings.TrimSpace(input.DisplayName)
	if name == "" {
		return InvalidBindingReason("workspace_display_name_required")
	}
	if overLimit(name, 120) {
		return InvalidBindingReason("workspace_display_name_too_long")
	}
	if containsUnsafe(name) {
		return InvalidBindingReason("unsafe_workspace_content")
	}
	switch input.Status {
	case "", WorkspaceActive, WorkspaceArchived, WorkspaceDisabled:
	default:
		return InvalidBindingReason("workspace_status_invalid")
	}
	return nil
}

// BindingMutationInput is the validated shape for creating/updating a binding rule.
// CrossTenant, ChannelAvailable, AccountConnected, ProfileSelectable, and
// WorkspaceSelectable carry the caller's tenant-scoped validation findings so policy
// can reject cross-tenant and unavailable references with safe reasons (FR-009).
type BindingMutationInput struct {
	ScopeKind               ScopeKind
	ScopeRef                string
	SelectedProfileID       string
	SelectedWorkspaceID     string
	CrossTenant             bool
	ScopeRefAvailable       bool
	ScopeConnectorSupported bool
	ProfileSelectable       bool
	WorkspaceSelectable     bool
}

// ValidateBindingMutation rejects cross-tenant resources, unavailable channels,
// disconnected accounts, archived/disabled profiles, unavailable workspaces,
// unsupported connector surfaces, malformed values, and policy-conflicting selections
// with safe user-visible reasons (FR-009).
func ValidateBindingMutation(input BindingMutationInput) error {
	if input.CrossTenant {
		return InvalidBindingReason("cross_tenant_reference_denied")
	}
	switch input.ScopeKind {
	case ScopeChannel, ScopeIntegrationAccount:
	default:
		return InvalidBindingReason("binding_scope_kind_invalid")
	}
	ref := strings.TrimSpace(input.ScopeRef)
	if ref == "" || overLimit(ref, 256) || containsUnsafe(ref) || !identifierLike(ref) {
		return InvalidBindingReason("binding_scope_ref_malformed")
	}
	if !input.ScopeRefAvailable {
		if input.ScopeKind == ScopeChannel {
			return InvalidBindingReason("channel_unavailable")
		}
		return InvalidBindingReason("integration_account_disconnected")
	}
	if !input.ScopeConnectorSupported {
		return InvalidBindingReason("connector_binding_unsupported")
	}

	profileID := strings.TrimSpace(input.SelectedProfileID)
	workspaceID := strings.TrimSpace(input.SelectedWorkspaceID)

	// Integration-account defaults supply profile only; they must not carry a workspace.
	if input.ScopeKind == ScopeIntegrationAccount && workspaceID != "" {
		return InvalidBindingReason("account_binding_workspace_not_allowed")
	}
	// A binding must select at least one of profile/workspace to be meaningful.
	if profileID == "" && workspaceID == "" {
		return InvalidBindingReason("binding_selects_nothing")
	}
	if profileID != "" {
		if overLimit(profileID, 256) || containsUnsafe(profileID) || !identifierLike(profileID) {
			return InvalidBindingReason("selected_profile_malformed")
		}
		if !input.ProfileSelectable {
			return InvalidBindingReason("selected_profile_unavailable")
		}
	}
	if workspaceID != "" {
		if overLimit(workspaceID, 256) || containsUnsafe(workspaceID) || !identifierLike(workspaceID) {
			return InvalidBindingReason("selected_workspace_malformed")
		}
		if !input.WorkspaceSelectable {
			return InvalidBindingReason("selected_workspace_unavailable")
		}
	}
	return nil
}

// CapabilityVisibilityMutationInput is the validated shape for setting visibility.
type CapabilityVisibilityMutationInput struct {
	ScopeKind    VisibilityScopeKind
	ScopeRef     string
	CapabilityID string
	Visibility   Visibility
}

// ValidateCapabilityVisibilityMutation enforces that visibility is only user-edited at
// profile and workspace scope this phase (FR-018) with safe, known values.
func ValidateCapabilityVisibilityMutation(input CapabilityVisibilityMutationInput) error {
	switch input.ScopeKind {
	case VisibilityScopeProfile, VisibilityScopeWorkspace:
	default:
		return InvalidBindingReason("visibility_scope_not_editable")
	}
	ref := strings.TrimSpace(input.ScopeRef)
	if ref == "" || overLimit(ref, 256) || containsUnsafe(ref) || !identifierLike(ref) {
		return InvalidBindingReason("visibility_scope_ref_malformed")
	}
	cap := strings.TrimSpace(input.CapabilityID)
	if cap == "" || overLimit(cap, 256) || containsUnsafe(cap) || !identifierLike(cap) {
		return InvalidBindingReason("capability_id_malformed")
	}
	switch input.Visibility {
	case VisibilityVisible, VisibilityHidden, VisibilityDisabled, VisibilityDefaultEnabled:
	default:
		return InvalidBindingReason("visibility_value_invalid")
	}
	return nil
}

// RepairStatusForReferences derives the repair status of a binding from the
// availability of the resources it references (FR-022). Healthy only when every
// referenced resource is present and the binding is active.
func RepairStatusForReferences(status BindingStatus, profileSelectable, workspaceSelectable, scopeRefAvailable, connectorSupported bool) RepairStatus {
	if status == BindingDisabled {
		return RepairDisabled
	}
	if !connectorSupported {
		return RepairUnsupported
	}
	if !scopeRefAvailable {
		return RepairStale
	}
	if !profileSelectable || !workspaceSelectable {
		return RepairNeedsRepair
	}
	return RepairHealthy
}

// --- local safe-value helpers (kept package-local to preserve the bindings boundary) ---

func overLimit(value string, limit int) bool {
	return len(strings.TrimSpace(value)) > limit
}

func identifierLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.', ':', '/', '@', '#':
			continue
		default:
			return false
		}
	}
	return true
}

func containsUnsafe(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "secret=") || strings.Contains(lower, "token=") || strings.Contains(lower, "api_key")
}
