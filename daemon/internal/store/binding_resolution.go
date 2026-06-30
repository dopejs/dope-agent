package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
)

// dbtx is the subset of *sql.DB / *sql.Tx used by transaction-aware resolution helpers, so
// the same query logic can run either standalone or inside a single snapshot transaction.
type dbtx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// BindingResolutionParams carries the work-start facts needed to resolve a binding selection
// in one consistent snapshot.
type BindingResolutionParams struct {
	TenantID                      string
	ChannelScopeRef               string
	AccountScopeRef               string
	TenantDefaultProfileID        string
	TenantDefaultProfileVersionID string
}

// ResolveBindingSelection resolves the effective binding selection for new work within a
// single transaction so a concurrent binding or capability-visibility mutation cannot
// interleave between the workspace, binding, availability, and visibility reads — each work
// item records exactly one resolved selection with no mixed pre/post-change state (FR-033).
// It applies the full deterministic precedence channel -> integration-account default ->
// tenant default (FR-006). Returns a zero selection when tenant context is absent.
func (s *SQLiteStore) ResolveBindingSelection(ctx context.Context, params BindingResolutionParams) (bindings.EffectiveBindingSelection, error) {
	tenantID := strings.TrimSpace(params.TenantID)
	if s == nil || s.db == nil || tenantID == "" {
		return bindings.EffectiveBindingSelection{}, nil
	}
	// A write-capable transaction is required because the default workspace is provisioned
	// lazily on first access; with SetMaxOpenConns(1) this also fully serializes the read set
	// against concurrent mutations, yielding a single consistent snapshot.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}
	defer func() { _ = tx.Rollback() }()

	ws, err := ensureDefaultWorkspaceTx(ctx, tx, tenantID)
	if err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}
	channelBinding, err := activeBindingTx(ctx, tx, tenantID, bindings.ScopeChannel, params.ChannelScopeRef)
	if err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}
	accountBinding, err := activeBindingTx(ctx, tx, tenantID, bindings.ScopeIntegrationAccount, params.AccountScopeRef)
	if err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}

	var oracleErr error
	resolution := bindings.ResolveSelection(bindings.ResolutionInput{
		ChannelBinding:                channelBinding,
		AccountBinding:                accountBinding,
		TenantDefaultProfileID:        params.TenantDefaultProfileID,
		TenantDefaultProfileVersionID: params.TenantDefaultProfileVersionID,
		TenantDefaultWorkspaceID:      ws.WorkspaceID,
		ProfileAvailable: func(id string) bool {
			ok, e := profileActiveTx(ctx, tx, tenantID, id)
			if e != nil {
				oracleErr = e
			}
			return ok
		},
		WorkspaceAvailable: func(id string) bool {
			if id == ws.WorkspaceID {
				return ws.Status == bindings.WorkspaceActive
			}
			ok, e := workspaceActiveTx(ctx, tx, tenantID, id)
			if e != nil {
				oracleErr = e
			}
			return ok
		},
	})
	if oracleErr != nil {
		return bindings.EffectiveBindingSelection{}, oracleErr
	}

	visibility, err := capabilityVisibilitySummaryTx(ctx, tx, tenantID, resolution.SelectedProfileID, resolution.SelectedWorkspaceID)
	if err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}
	resolution.CapabilityVisibility = visibility

	if err := tx.Commit(); err != nil {
		return bindings.EffectiveBindingSelection{}, err
	}
	return resolution, nil
}

// ensureDefaultWorkspaceTx mirrors EnsureDefaultWorkspace within an open transaction.
func ensureDefaultWorkspaceTx(ctx context.Context, q dbtx, tenantID string) (bindings.Workspace, error) {
	var doc []byte
	err := q.QueryRowContext(ctx, `SELECT document_json FROM workspaces WHERE tenant_id = ? AND is_default = 1`, tenantID).Scan(&doc)
	if err == nil {
		ws, _, decodeErr := decodeWorkspace(doc)
		return ws, decodeErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return bindings.Workspace{}, err
	}
	now := time.Now().UTC()
	ws := bindings.Workspace{
		WorkspaceID:      newStoreID("ws"),
		TenantID:         tenantID,
		DisplayName:      "Personal Workspace",
		Status:           bindings.WorkspaceActive,
		IsDefault:        true,
		OwnerPrincipalID: "system",
		RepairStatus:     bindings.RepairHealthy,
		RedactionStatus:  bindings.RedactionNotRequired,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	wsDoc, err := json.Marshal(ws)
	if err != nil {
		return bindings.Workspace{}, err
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO workspaces (workspace_id, tenant_id, display_name, status, is_default, owner_principal_id, repair_status, redaction_status, created_at, updated_at, archived_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ws.WorkspaceID, ws.TenantID, bindings.SafeLabel(ws.DisplayName), string(ws.Status), boolToInt(ws.IsDefault), ws.OwnerPrincipalID, string(ws.RepairStatus), string(ws.RedactionStatus), ws.CreatedAt.Format(time.RFC3339Nano), ws.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(ws.ArchivedAt), wsDoc); err != nil {
		return bindings.Workspace{}, err
	}
	return ws, nil
}

// activeBindingTx mirrors resolveActiveBinding within an open transaction.
func activeBindingTx(ctx context.Context, q dbtx, tenantID string, kind bindings.ScopeKind, scopeRef string) (*bindings.BindingRule, error) {
	scopeRef = strings.TrimSpace(scopeRef)
	if scopeRef == "" {
		return nil, nil
	}
	var doc []byte
	err := q.QueryRowContext(ctx, `SELECT document_json FROM binding_rules WHERE tenant_id = ? AND scope_kind = ? AND scope_ref = ? AND status = 'active'`, strings.TrimSpace(tenantID), string(kind), scopeRef).Scan(&doc)
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

// profileActiveTx mirrors profileSelectable within an open transaction.
func profileActiveTx(ctx context.Context, q dbtx, tenantID, profileID string) (bool, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return false, nil
	}
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM agent_profiles WHERE tenant_id = ? AND profile_id = ?`, strings.TrimSpace(tenantID), profileID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "active", nil
}

// workspaceActiveTx reports whether a non-default workspace is active within a transaction.
func workspaceActiveTx(ctx context.Context, q dbtx, tenantID, workspaceID string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return false, nil
	}
	var doc []byte
	err := q.QueryRowContext(ctx, `SELECT document_json FROM workspaces WHERE tenant_id = ? AND workspace_id = ?`, strings.TrimSpace(tenantID), workspaceID).Scan(&doc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	ws, _, err := decodeWorkspace(doc)
	if err != nil {
		return false, err
	}
	return ws.Status == bindings.WorkspaceActive, nil
}

// capabilityVisibilitySummaryTx builds the per-capability decision summary (profile +
// workspace policy, strictest-wins) within an open transaction.
func capabilityVisibilitySummaryTx(ctx context.Context, q dbtx, tenantID, profileID, workspaceID string) ([]bindings.CapabilityDecision, error) {
	profilePolicies, err := capabilityPoliciesTx(ctx, q, tenantID, bindings.VisibilityScopeProfile, profileID)
	if err != nil {
		return nil, err
	}
	workspacePolicies, err := capabilityPoliciesTx(ctx, q, tenantID, bindings.VisibilityScopeWorkspace, workspaceID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for capID := range profilePolicies {
		seen[capID] = struct{}{}
	}
	for capID := range workspacePolicies {
		seen[capID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	inputs := make([]bindings.VisibilityInput, 0, len(seen))
	for capID := range seen {
		inputs = append(inputs, bindings.VisibilityInput{
			CapabilityID:    capID,
			ProfilePolicy:   profilePolicies[capID],
			WorkspacePolicy: workspacePolicies[capID],
		})
	}
	return bindings.ResolveVisibilitySet(inputs), nil
}

func capabilityPoliciesTx(ctx context.Context, q dbtx, tenantID string, scopeKind bindings.VisibilityScopeKind, scopeRef string) (map[string]bindings.Visibility, error) {
	policies := map[string]bindings.Visibility{}
	if strings.TrimSpace(scopeRef) == "" {
		return policies, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT document_json FROM capability_visibility_policies WHERE tenant_id = ? AND scope_kind = ? AND scope_ref = ? ORDER BY capability_id ASC`, strings.TrimSpace(tenantID), string(scopeKind), strings.TrimSpace(scopeRef))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var p bindings.CapabilityVisibilityPolicy
		if err := json.Unmarshal(doc, &p); err != nil {
			return nil, err
		}
		policies[p.CapabilityID] = p.Visibility
	}
	return policies, rows.Err()
}
