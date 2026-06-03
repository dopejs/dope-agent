package bindings

// Request/response DTOs shared by the store and API layers. Responses surface only
// safe, redacted fields (FR-028); scope references are presented via SafeLabel.

// CreateWorkspaceRequest is the body for POST /v1/workspaces.
type CreateWorkspaceRequest struct {
	DisplayName string `json:"displayName"`
}

// UpdateWorkspaceRequest is the body for PATCH /v1/workspaces/{id}.
type UpdateWorkspaceRequest struct {
	Status     WorkspaceStatus `json:"status"`
	ReasonCode string          `json:"reasonCode,omitempty"`
}

// CreateBindingRequest is the body for POST /v1/bindings.
type CreateBindingRequest struct {
	ScopeKind           ScopeKind `json:"scopeKind"`
	ScopeRef            string    `json:"scopeRef"`
	SelectedProfileID   string    `json:"selectedProfileId,omitempty"`
	SelectedWorkspaceID string    `json:"selectedWorkspaceId,omitempty"`
	ReasonCode          string    `json:"reasonCode,omitempty"`
}

// UpdateBindingRequest is the body for PATCH /v1/bindings/{id}.
type UpdateBindingRequest struct {
	SelectedProfileID   string `json:"selectedProfileId,omitempty"`
	SelectedWorkspaceID string `json:"selectedWorkspaceId,omitempty"`
	Disable             bool   `json:"disable,omitempty"`
	ReasonCode          string `json:"reasonCode,omitempty"`
}

// SetVisibilityRequest is the body for PUT /v1/capability-visibility.
type SetVisibilityRequest struct {
	ScopeKind    VisibilityScopeKind `json:"scopeKind"`
	ScopeRef     string              `json:"scopeRef"`
	CapabilityID string              `json:"capabilityId"`
	Visibility   Visibility          `json:"visibility"`
	ReasonCode   string              `json:"reasonCode,omitempty"`
}

// WorkspaceResource is the safe JSON view of a workspace.
type WorkspaceResource struct {
	WorkspaceID     string          `json:"workspaceId"`
	TenantID        string          `json:"tenantId"`
	DisplayName     string          `json:"displayName"`
	Status          WorkspaceStatus `json:"status"`
	IsDefault       bool            `json:"isDefault"`
	RepairStatus    RepairStatus    `json:"repairStatus"`
	RedactionStatus RedactionStatus `json:"redactionStatus"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

// BindingResource is the safe JSON view of a binding rule.
type BindingResource struct {
	BindingID                 string           `json:"bindingId"`
	TenantID                  string           `json:"tenantId"`
	ScopeKind                 ScopeKind        `json:"scopeKind"`
	ScopeLabel                string           `json:"scopeLabel"`
	SelectedProfileID         string           `json:"selectedProfileId,omitempty"`
	SelectedProfileVersionID  string           `json:"selectedProfileVersionId,omitempty"`
	SelectedWorkspaceID       string           `json:"selectedWorkspaceId,omitempty"`
	Status                    BindingStatus    `json:"status"`
	RepairStatus              RepairStatus     `json:"repairStatus"`
	ValidationStatus          ValidationStatus `json:"validationStatus"`
	ResultingSelectionSummary string           `json:"resultingSelectionSummary,omitempty"`
	LastMaterialChangeAt      string           `json:"lastMaterialChangeAt"`
	RedactionStatus           RedactionStatus  `json:"redactionStatus"`
}

// CapabilityVisibilityResource is the safe JSON view of a visibility policy.
type CapabilityVisibilityResource struct {
	PolicyID         string              `json:"policyId"`
	TenantID         string              `json:"tenantId"`
	ScopeKind        VisibilityScopeKind `json:"scopeKind"`
	ScopeRef         string              `json:"scopeRef"`
	CapabilityID     string              `json:"capabilityId"`
	Visibility       Visibility          `json:"visibility"`
	ValidationStatus ValidationStatus    `json:"validationStatus"`
}

// BindingRuntimeEvidenceResource is the safe JSON view of runtime binding evidence.
type BindingRuntimeEvidenceResource struct {
	ProjectionID             string                       `json:"projectionId"`
	ResourceKind             string                       `json:"resourceKind"`
	ResourceID               string                       `json:"resourceId"`
	SelectedProfileID        string                       `json:"selectedProfileId,omitempty"`
	SelectedProfileVersionID string                       `json:"selectedProfileVersionId,omitempty"`
	SelectedWorkspaceID      string                       `json:"selectedWorkspaceId,omitempty"`
	BindingScope             BindingRuntimeScope          `json:"bindingScope"`
	BindingID                string                       `json:"bindingId,omitempty"`
	Classification           Classification               `json:"classification"`
	SelectionReason          string                       `json:"selectionReason"`
	CapabilityVisibility     []CapabilityDecisionResource `json:"capabilityVisibilitySummary"`
	OccurredAt               string                       `json:"occurredAt"`
	RedactionStatus          RedactionStatus              `json:"redactionStatus"`
}

// CapabilityDecisionResource is the safe JSON view of one capability decision.
type CapabilityDecisionResource struct {
	CapabilityID   string              `json:"capabilityId"`
	Effective      EffectiveVisibility `json:"effective"`
	DefaultEnabled bool                `json:"defaultEnabled"`
	Offered        bool                `json:"offered"`
	Executable     bool                `json:"executable"`
	Reason         string              `json:"reason"`
	Scope          string              `json:"scope"`
}

const rfc3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

// ToWorkspaceResource maps a Workspace to its safe JSON view.
func ToWorkspaceResource(ws Workspace) WorkspaceResource {
	return WorkspaceResource{
		WorkspaceID:     ws.WorkspaceID,
		TenantID:        ws.TenantID,
		DisplayName:     SafeLabel(ws.DisplayName),
		Status:          ws.Status,
		IsDefault:       ws.IsDefault,
		RepairStatus:    ws.RepairStatus,
		RedactionStatus: ws.RedactionStatus,
		CreatedAt:       ws.CreatedAt.Format(rfc3339Nano),
		UpdatedAt:       ws.UpdatedAt.Format(rfc3339Nano),
	}
}

// ToBindingResource maps a BindingRule to its safe JSON view.
func ToBindingResource(b BindingRule) BindingResource {
	return BindingResource{
		BindingID:                 b.BindingID,
		TenantID:                  b.TenantID,
		ScopeKind:                 b.ScopeKind,
		ScopeLabel:                SafeLabel(b.ScopeRef),
		SelectedProfileID:         b.SelectedProfileID,
		SelectedProfileVersionID:  b.SelectedProfileVersionID,
		SelectedWorkspaceID:       b.SelectedWorkspaceID,
		Status:                    b.Status,
		RepairStatus:              b.RepairStatus,
		ValidationStatus:          b.ValidationStatus,
		ResultingSelectionSummary: SafeLabel(b.ResultingSelectionSummary),
		LastMaterialChangeAt:      b.UpdatedAt.Format(rfc3339Nano),
		RedactionStatus:           b.RedactionStatus,
	}
}

// ToCapabilityVisibilityResource maps a policy to its safe JSON view.
func ToCapabilityVisibilityResource(p CapabilityVisibilityPolicy) CapabilityVisibilityResource {
	return CapabilityVisibilityResource{
		PolicyID:         p.PolicyID,
		TenantID:         p.TenantID,
		ScopeKind:        p.ScopeKind,
		ScopeRef:         SafeLabel(p.ScopeRef),
		CapabilityID:     SafeLabel(p.CapabilityID),
		Visibility:       p.Visibility,
		ValidationStatus: p.ValidationStatus,
	}
}

// ToRuntimeEvidenceResource maps runtime evidence to its safe JSON view.
func ToRuntimeEvidenceResource(e RuntimeBindingEvidence) BindingRuntimeEvidenceResource {
	decisions := make([]CapabilityDecisionResource, 0, len(e.CapabilityVisibility))
	for _, d := range e.CapabilityVisibility {
		decisions = append(decisions, CapabilityDecisionResource{
			CapabilityID:   SafeLabel(d.CapabilityID),
			Effective:      d.Effective,
			DefaultEnabled: d.DefaultEnabled,
			Offered:        d.Offered,
			Executable:     d.Executable,
			Reason:         SafeReason(d.Reason),
			Scope:          d.Scope,
		})
	}
	return BindingRuntimeEvidenceResource{
		ProjectionID:             e.ProjectionID,
		ResourceKind:             e.ResourceKind,
		ResourceID:               e.ResourceID,
		SelectedProfileID:        e.SelectedProfileID,
		SelectedProfileVersionID: e.SelectedProfileVersionID,
		SelectedWorkspaceID:      e.SelectedWorkspaceID,
		BindingScope:             e.BindingScope,
		BindingID:                e.BindingID,
		Classification:           e.Classification,
		SelectionReason:          SafeReason(e.SelectionReason),
		CapabilityVisibility:     decisions,
		OccurredAt:               e.OccurredAt.Format(rfc3339Nano),
		RedactionStatus:          e.RedactionStatus,
	}
}
