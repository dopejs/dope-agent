package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

// ErrBindingNotFound is returned when a binding id is absent for the tenant.
var ErrBindingNotFound = errors.New("binding not found")

type bindingAuditRow struct {
	AuditEventID              string
	TenantID                  string
	BindingID                 string
	WorkspaceID               string
	ActorPrincipalID          string
	EventKind                 string
	Outcome                   string
	PermissionGate            string
	ReasonCode                string
	SafeSummary               string
	PreviousSelectionSummary  string
	ResultingSelectionSummary string
	OccurredAt                time.Time
}

func insertBindingAuditTx(ctx context.Context, tx *sql.Tx, row bindingAuditRow) error {
	doc, err := json.Marshal(map[string]any{
		"auditEventId":              row.AuditEventID,
		"tenantId":                  row.TenantID,
		"bindingId":                 row.BindingID,
		"workspaceId":               row.WorkspaceID,
		"actorPrincipalId":          row.ActorPrincipalID,
		"eventKind":                 row.EventKind,
		"outcome":                   row.Outcome,
		"permissionGate":            row.PermissionGate,
		"reasonCode":                row.ReasonCode,
		"safeSummary":               bindings.SafeLabel(row.SafeSummary),
		"previousSelectionSummary":  bindings.SafeLabel(row.PreviousSelectionSummary),
		"resultingSelectionSummary": bindings.SafeLabel(row.ResultingSelectionSummary),
		"occurredAt":                row.OccurredAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO binding_audit_events (audit_event_id, tenant_id, binding_id, workspace_id, actor_principal_id, event_kind, outcome, permission_gate, reason_code, safe_summary, occurred_at, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.AuditEventID, row.TenantID, nullableString(row.BindingID), nullableString(row.WorkspaceID), row.ActorPrincipalID, row.EventKind, row.Outcome, row.PermissionGate, row.ReasonCode, bindings.SafeLabel(row.SafeSummary), row.OccurredAt.Format(time.RFC3339Nano), string(bindings.RedactionRedacted), doc)
	return err
}

// CreateBindingRule validates and persists a channel or integration-account binding,
// auditing the change in the same transaction so it fails closed (FR-008, FR-010, FR-011).
func (s *SQLiteStore) CreateBindingRule(ctx context.Context, actor identity.TenantContext, req bindings.CreateBindingRequest) (bindings.BindingRule, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.BindingRule{}, "", bindings.ErrExplicitActorRequired
	}
	profileSelectable, err := s.profileSelectable(ctx, actor.TenantID, req.SelectedProfileID)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	workspaceSelectable, err := s.workspaceSelectable(ctx, actor.TenantID, req.SelectedWorkspaceID)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	scopeAvailable, err := s.bindingScopeAvailable(ctx, actor.TenantID, req.ScopeKind, req.ScopeRef)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	mutation := bindings.BindingMutationInput{
		ScopeKind:               req.ScopeKind,
		ScopeRef:                req.ScopeRef,
		SelectedProfileID:       req.SelectedProfileID,
		SelectedWorkspaceID:     req.SelectedWorkspaceID,
		ScopeRefAvailable:       scopeAvailable,
		ScopeConnectorSupported: true,
		ProfileSelectable:       profileSelectable || strings.TrimSpace(req.SelectedProfileID) == "",
		WorkspaceSelectable:     workspaceSelectable || strings.TrimSpace(req.SelectedWorkspaceID) == "",
	}
	if err := bindings.ValidateBindingMutation(mutation); err != nil {
		return bindings.BindingRule{}, "", err
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	rule := bindings.BindingRule{
		BindingID:                 newStoreID("bnd"),
		TenantID:                  actor.TenantID,
		ScopeKind:                 req.ScopeKind,
		ScopeRef:                  strings.TrimSpace(req.ScopeRef),
		SelectedProfileID:         strings.TrimSpace(req.SelectedProfileID),
		SelectedWorkspaceID:       strings.TrimSpace(req.SelectedWorkspaceID),
		Status:                    bindings.BindingActive,
		RepairStatus:              bindings.RepairHealthy,
		ValidationStatus:          bindings.ValidationValid,
		ActorPrincipalID:          actor.PrincipalID,
		AuditEventID:              auditID,
		ResultingSelectionSummary: bindingSelectionSummary(req.SelectedProfileID, req.SelectedWorkspaceID),
		RedactionStatus:           bindings.RedactionRedacted,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertBindingRuleTx(ctx, tx, rule); err != nil {
		if isUniqueConstraintErr(err) {
			return bindings.BindingRule{}, "", bindings.InvalidBindingReason("active_binding_already_exists")
		}
		return bindings.BindingRule{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, BindingID: rule.BindingID, ActorPrincipalID: actor.PrincipalID, EventKind: "binding.created", Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: defaultReason(req.ReasonCode, "user_created_binding"), SafeSummary: "Binding created", ResultingSelectionSummary: rule.ResultingSelectionSummary, OccurredAt: now}); err != nil {
		return bindings.BindingRule{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.BindingRule{}, "", err
	}
	return rule, auditID, nil
}

// UpdateBindingRule updates the selection or disables a binding (FR-004, FR-008).
func (s *SQLiteStore) UpdateBindingRule(ctx context.Context, actor identity.TenantContext, bindingID string, req bindings.UpdateBindingRequest) (bindings.BindingRule, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.BindingRule{}, "", bindings.ErrExplicitActorRequired
	}
	rule, found, err := s.GetBindingRule(ctx, actor.TenantID, bindingID)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	if !found {
		return bindings.BindingRule{}, "", ErrBindingNotFound
	}
	previousSummary := rule.ResultingSelectionSummary
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	eventKind := "binding.updated"
	if req.Disable {
		rule.Status = bindings.BindingDisabled
		rule.DisabledAt = &now
		rule.RepairStatus = bindings.RepairDisabled
		eventKind = "binding.disabled"
	} else {
		if strings.TrimSpace(req.SelectedProfileID) != "" {
			rule.SelectedProfileID = strings.TrimSpace(req.SelectedProfileID)
		}
		if strings.TrimSpace(req.SelectedWorkspaceID) != "" {
			rule.SelectedWorkspaceID = strings.TrimSpace(req.SelectedWorkspaceID)
		}
		profileSelectable, perr := s.profileSelectable(ctx, actor.TenantID, rule.SelectedProfileID)
		if perr != nil {
			return bindings.BindingRule{}, "", perr
		}
		workspaceSelectable, werr := s.workspaceSelectable(ctx, actor.TenantID, rule.SelectedWorkspaceID)
		if werr != nil {
			return bindings.BindingRule{}, "", werr
		}
		mutation := bindings.BindingMutationInput{
			ScopeKind:               rule.ScopeKind,
			ScopeRef:                rule.ScopeRef,
			SelectedProfileID:       rule.SelectedProfileID,
			SelectedWorkspaceID:     rule.SelectedWorkspaceID,
			ScopeRefAvailable:       true,
			ScopeConnectorSupported: true,
			ProfileSelectable:       profileSelectable || rule.SelectedProfileID == "",
			WorkspaceSelectable:     workspaceSelectable || rule.SelectedWorkspaceID == "",
		}
		if err := bindings.ValidateBindingMutation(mutation); err != nil {
			return bindings.BindingRule{}, "", err
		}
		rule.ResultingSelectionSummary = bindingSelectionSummary(rule.SelectedProfileID, rule.SelectedWorkspaceID)
	}
	rule.PreviousSelectionSummary = previousSummary
	rule.UpdatedAt = now
	rule.ActorPrincipalID = actor.PrincipalID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateBindingRuleTx(ctx, tx, rule); err != nil {
		return bindings.BindingRule{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, BindingID: rule.BindingID, ActorPrincipalID: actor.PrincipalID, EventKind: eventKind, Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: defaultReason(req.ReasonCode, "user_updated_binding"), SafeSummary: "Binding updated", PreviousSelectionSummary: previousSummary, ResultingSelectionSummary: rule.ResultingSelectionSummary, OccurredAt: now}); err != nil {
		return bindings.BindingRule{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.BindingRule{}, "", err
	}
	return rule, auditID, nil
}

// RemoveBindingRule deletes a binding while preserving historical evidence (FR-012).
func (s *SQLiteStore) RemoveBindingRule(ctx context.Context, actor identity.TenantContext, bindingID string) (string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return "", bindings.ErrExplicitActorRequired
	}
	rule, found, err := s.GetBindingRule(ctx, actor.TenantID, bindingID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrBindingNotFound
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM binding_rules WHERE tenant_id = ? AND binding_id = ?`, actor.TenantID, rule.BindingID); err != nil {
		return "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, BindingID: rule.BindingID, ActorPrincipalID: actor.PrincipalID, EventKind: "binding.removed", Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: "user_removed_binding", SafeSummary: "Binding removed", OccurredAt: now}); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return auditID, nil
}

// RepairBindingRule recomputes the repair status from current references and, when all
// references are healthy, returns the binding to active (FR-022, FR-031 recovery path).
func (s *SQLiteStore) RepairBindingRule(ctx context.Context, actor identity.TenantContext, bindingID string) (bindings.BindingRule, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.BindingRule{}, "", bindings.ErrExplicitActorRequired
	}
	rule, found, err := s.GetBindingRule(ctx, actor.TenantID, bindingID)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	if !found {
		return bindings.BindingRule{}, "", ErrBindingNotFound
	}
	repair, err := s.repairStatusFor(ctx, rule)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	rule.RepairStatus = repair
	rule.UpdatedAt = now
	rule.ActorPrincipalID = actor.PrincipalID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.BindingRule{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateBindingRuleTx(ctx, tx, rule); err != nil {
		return bindings.BindingRule{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, BindingID: rule.BindingID, ActorPrincipalID: actor.PrincipalID, EventKind: "binding.repaired", Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: "user_repaired_binding", SafeSummary: "Binding repair evaluated", OccurredAt: now}); err != nil {
		return bindings.BindingRule{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.BindingRule{}, "", err
	}
	return rule, auditID, nil
}

// ListBindingRules returns tenant bindings with freshly computed repair status (FR-022).
func (s *SQLiteStore) ListBindingRules(ctx context.Context, tenantID string, limit int) ([]bindings.BindingRule, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM binding_rules WHERE tenant_id = ? ORDER BY updated_at DESC, binding_id DESC LIMIT ?`, strings.TrimSpace(tenantID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]bindings.BindingRule, 0)
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var rule bindings.BindingRule
		if err := json.Unmarshal(doc, &rule); err != nil {
			return nil, err
		}
		repair, err := s.repairStatusFor(ctx, rule)
		if err != nil {
			return nil, err
		}
		rule.RepairStatus = repair
		items = append(items, rule)
	}
	return items, rows.Err()
}

// GetBindingRule returns one binding by id within the tenant, with fresh repair status.
func (s *SQLiteStore) GetBindingRule(ctx context.Context, tenantID, bindingID string) (bindings.BindingRule, bool, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM binding_rules WHERE tenant_id = ? AND binding_id = ?`, strings.TrimSpace(tenantID), strings.TrimSpace(bindingID)).Scan(&doc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bindings.BindingRule{}, false, nil
		}
		return bindings.BindingRule{}, false, err
	}
	var rule bindings.BindingRule
	if err := json.Unmarshal(doc, &rule); err != nil {
		return bindings.BindingRule{}, false, err
	}
	repair, err := s.repairStatusFor(ctx, rule)
	if err != nil {
		return bindings.BindingRule{}, false, err
	}
	rule.RepairStatus = repair
	return rule, true, nil
}

// ResolveChannelBinding returns the active channel binding for a scope ref, if any.
func (s *SQLiteStore) ResolveChannelBinding(ctx context.Context, tenantID, scopeRef string) (*bindings.BindingRule, error) {
	return s.resolveActiveBinding(ctx, tenantID, bindings.ScopeChannel, scopeRef)
}

// ResolveAccountBinding returns the active integration-account default, if any.
func (s *SQLiteStore) ResolveAccountBinding(ctx context.Context, tenantID, scopeRef string) (*bindings.BindingRule, error) {
	return s.resolveActiveBinding(ctx, tenantID, bindings.ScopeIntegrationAccount, scopeRef)
}

func (s *SQLiteStore) resolveActiveBinding(ctx context.Context, tenantID string, kind bindings.ScopeKind, scopeRef string) (*bindings.BindingRule, error) {
	scopeRef = strings.TrimSpace(scopeRef)
	if scopeRef == "" {
		return nil, nil
	}
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM binding_rules WHERE tenant_id = ? AND scope_kind = ? AND scope_ref = ? AND status = 'active'`, strings.TrimSpace(tenantID), string(kind), scopeRef).Scan(&doc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var rule bindings.BindingRule
	if err := json.Unmarshal(doc, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// profileSelectable reports whether an agent profile exists and is active in the tenant.
func (s *SQLiteStore) profileSelectable(ctx context.Context, tenantID, profileID string) (bool, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return false, nil
	}
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM agent_profiles WHERE tenant_id = ? AND profile_id = ?`, strings.TrimSpace(tenantID), profileID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "active", nil
}

// IsProfileSelectable reports whether an agent profile is active and selectable.
func (s *SQLiteStore) IsProfileSelectable(ctx context.Context, tenantID, profileID string) (bool, error) {
	return s.profileSelectable(ctx, tenantID, profileID)
}

// IsWorkspaceSelectable reports whether a workspace is active and selectable.
func (s *SQLiteStore) IsWorkspaceSelectable(ctx context.Context, tenantID, workspaceID string) (bool, error) {
	return s.workspaceSelectable(ctx, tenantID, workspaceID)
}

// EffectiveCapabilityVisibility resolves a capability's effective visibility for the
// active profile/workspace, applying strictest-wins across profile and workspace policy
// plus any higher-level limits the caller supplies (FR-015, FR-016, FR-017).
func (s *SQLiteStore) EffectiveCapabilityVisibility(ctx context.Context, tenantID, profileID, workspaceID, capabilityID string, limits []bindings.ScopeVisibility) (bindings.CapabilityDecision, error) {
	profilePolicies, workspacePolicies, err := s.CapabilityVisibilityForScopes(ctx, tenantID, profileID, workspaceID)
	if err != nil {
		return bindings.CapabilityDecision{}, err
	}
	return bindings.ResolveCapabilityVisibility(bindings.VisibilityInput{
		CapabilityID:    capabilityID,
		Limits:          limits,
		ProfilePolicy:   profilePolicies[strings.TrimSpace(capabilityID)],
		WorkspacePolicy: workspacePolicies[strings.TrimSpace(capabilityID)],
	}), nil
}

// bindingScopeAvailable performs a best-effort liveness check of the binding scope.
// Channel scope refs reference external connector identities, treated as available when
// well-formed; integration-account scope refs are checked against the integrations table.
func (s *SQLiteStore) bindingScopeAvailable(ctx context.Context, tenantID string, kind bindings.ScopeKind, scopeRef string) (bool, error) {
	scopeRef = strings.TrimSpace(scopeRef)
	if scopeRef == "" {
		return false, nil
	}
	if kind != bindings.ScopeIntegrationAccount {
		return true, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM integrations WHERE integration_id = ? OR account_key = ?`, scopeRef, scopeRef).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLiteStore) repairStatusFor(ctx context.Context, rule bindings.BindingRule) (bindings.RepairStatus, error) {
	profileSelectable := true
	if strings.TrimSpace(rule.SelectedProfileID) != "" {
		v, err := s.profileSelectable(ctx, rule.TenantID, rule.SelectedProfileID)
		if err != nil {
			return "", err
		}
		profileSelectable = v
	}
	workspaceSelectable := true
	if strings.TrimSpace(rule.SelectedWorkspaceID) != "" {
		v, err := s.workspaceSelectable(ctx, rule.TenantID, rule.SelectedWorkspaceID)
		if err != nil {
			return "", err
		}
		workspaceSelectable = v
	}
	scopeAvailable, err := s.bindingScopeAvailable(ctx, rule.TenantID, rule.ScopeKind, rule.ScopeRef)
	if err != nil {
		return "", err
	}
	return bindings.RepairStatusForReferences(rule.Status, profileSelectable, workspaceSelectable, scopeAvailable, true), nil
}

// --- capability visibility persistence ---

// SetCapabilityVisibility upserts a profile/workspace-scoped capability visibility policy
// (FR-018) with audit in the same transaction (FR-011).
func (s *SQLiteStore) SetCapabilityVisibility(ctx context.Context, actor identity.TenantContext, req bindings.SetVisibilityRequest) (bindings.CapabilityVisibilityPolicy, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.CapabilityVisibilityPolicy{}, "", bindings.ErrExplicitActorRequired
	}
	if err := bindings.ValidateCapabilityVisibilityMutation(bindings.CapabilityVisibilityMutationInput{ScopeKind: req.ScopeKind, ScopeRef: req.ScopeRef, CapabilityID: req.CapabilityID, Visibility: req.Visibility}); err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	policy := bindings.CapabilityVisibilityPolicy{
		PolicyID:         newStoreID("cvp"),
		TenantID:         actor.TenantID,
		ScopeKind:        req.ScopeKind,
		ScopeRef:         strings.TrimSpace(req.ScopeRef),
		CapabilityID:     strings.TrimSpace(req.CapabilityID),
		Visibility:       req.Visibility,
		ActorPrincipalID: actor.PrincipalID,
		ValidationStatus: bindings.ValidationValid,
		RedactionStatus:  bindings.RedactionRedacted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	// Preserve a stable policy id on upsert.
	if existingID, found, err := s.capabilityVisibilityID(ctx, actor.TenantID, req.ScopeKind, req.ScopeRef, req.CapabilityID); err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	} else if found {
		policy.PolicyID = existingID
	}
	doc, err := json.Marshal(policy)
	if err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO capability_visibility_policies (policy_id, tenant_id, scope_kind, scope_ref, capability_id, visibility, actor_principal_id, validation_status, redaction_status, created_at, updated_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(tenant_id, scope_kind, scope_ref, capability_id) DO UPDATE SET visibility = excluded.visibility, actor_principal_id = excluded.actor_principal_id, validation_status = excluded.validation_status, updated_at = excluded.updated_at, document_json = excluded.document_json`,
		policy.PolicyID, policy.TenantID, string(policy.ScopeKind), policy.ScopeRef, policy.CapabilityID, string(policy.Visibility), policy.ActorPrincipalID, string(policy.ValidationStatus), string(policy.RedactionStatus), policy.CreatedAt.Format(time.RFC3339Nano), policy.UpdatedAt.Format(time.RFC3339Nano), doc); err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, ActorPrincipalID: actor.PrincipalID, EventKind: "capability_visibility.changed", Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: defaultReason(req.ReasonCode, "user_set_capability_visibility"), SafeSummary: "Capability visibility set", OccurredAt: now}); err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.CapabilityVisibilityPolicy{}, "", err
	}
	return policy, auditID, nil
}

func (s *SQLiteStore) capabilityVisibilityID(ctx context.Context, tenantID string, scopeKind bindings.VisibilityScopeKind, scopeRef, capabilityID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT policy_id FROM capability_visibility_policies WHERE tenant_id = ? AND scope_kind = ? AND scope_ref = ? AND capability_id = ?`, tenantID, string(scopeKind), strings.TrimSpace(scopeRef), strings.TrimSpace(capabilityID)).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

// ListCapabilityVisibility returns the visibility policies for a profile/workspace scope.
func (s *SQLiteStore) ListCapabilityVisibility(ctx context.Context, tenantID string, scopeKind bindings.VisibilityScopeKind, scopeRef string) ([]bindings.CapabilityVisibilityPolicy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM capability_visibility_policies WHERE tenant_id = ? AND scope_kind = ? AND scope_ref = ? ORDER BY capability_id ASC`, strings.TrimSpace(tenantID), string(scopeKind), strings.TrimSpace(scopeRef))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]bindings.CapabilityVisibilityPolicy, 0)
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var p bindings.CapabilityVisibilityPolicy
		if err := json.Unmarshal(doc, &p); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// CapabilityVisibilityForScopes loads the profile- and workspace-scoped policy for each
// capability, keyed by capability id, for runtime resolution.
func (s *SQLiteStore) CapabilityVisibilityForScopes(ctx context.Context, tenantID, profileID, workspaceID string) (map[string]bindings.Visibility, map[string]bindings.Visibility, error) {
	profilePolicies := map[string]bindings.Visibility{}
	workspacePolicies := map[string]bindings.Visibility{}
	if strings.TrimSpace(profileID) != "" {
		items, err := s.ListCapabilityVisibility(ctx, tenantID, bindings.VisibilityScopeProfile, profileID)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range items {
			profilePolicies[p.CapabilityID] = p.Visibility
		}
	}
	if strings.TrimSpace(workspaceID) != "" {
		items, err := s.ListCapabilityVisibility(ctx, tenantID, bindings.VisibilityScopeWorkspace, workspaceID)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range items {
			workspacePolicies[p.CapabilityID] = p.Visibility
		}
	}
	return profilePolicies, workspacePolicies, nil
}

// --- tx helpers ---

func insertBindingRuleTx(ctx context.Context, tx *sql.Tx, rule bindings.BindingRule) error {
	doc, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO binding_rules (binding_id, tenant_id, scope_kind, scope_ref, selected_profile_id, selected_profile_version_id, selected_workspace_id, status, repair_status, validation_status, actor_principal_id, audit_event_id, previous_selection_summary, resulting_selection_summary, redaction_status, created_at, updated_at, disabled_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.BindingID, rule.TenantID, string(rule.ScopeKind), rule.ScopeRef, nullableString(rule.SelectedProfileID), nullableString(rule.SelectedProfileVersionID), nullableString(rule.SelectedWorkspaceID), string(rule.Status), string(rule.RepairStatus), string(rule.ValidationStatus), rule.ActorPrincipalID, rule.AuditEventID, nullableString(rule.PreviousSelectionSummary), nullableString(rule.ResultingSelectionSummary), string(rule.RedactionStatus), rule.CreatedAt.Format(time.RFC3339Nano), rule.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(rule.DisabledAt), doc)
	return err
}

func updateBindingRuleTx(ctx context.Context, tx *sql.Tx, rule bindings.BindingRule) error {
	doc, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE binding_rules SET scope_kind = ?, scope_ref = ?, selected_profile_id = ?, selected_profile_version_id = ?, selected_workspace_id = ?, status = ?, repair_status = ?, validation_status = ?, actor_principal_id = ?, resulting_selection_summary = ?, redaction_status = ?, updated_at = ?, disabled_at = ?, document_json = ? WHERE tenant_id = ? AND binding_id = ?`,
		string(rule.ScopeKind), rule.ScopeRef, nullableString(rule.SelectedProfileID), nullableString(rule.SelectedProfileVersionID), nullableString(rule.SelectedWorkspaceID), string(rule.Status), string(rule.RepairStatus), string(rule.ValidationStatus), rule.ActorPrincipalID, nullableString(rule.ResultingSelectionSummary), string(rule.RedactionStatus), rule.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(rule.DisabledAt), doc, rule.TenantID, rule.BindingID)
	return err
}

func bindingSelectionSummary(profileID, workspaceID string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(profileID) != "" {
		parts = append(parts, "profile")
	}
	if strings.TrimSpace(workspaceID) != "" {
		parts = append(parts, "workspace")
	}
	if len(parts) == 0 {
		return "no_selection"
	}
	return strings.Join(parts, "+")
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}
