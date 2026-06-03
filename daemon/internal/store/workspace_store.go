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

// ErrWorkspaceNotFound is returned when a workspace id is absent for the tenant.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// EnsureDefaultWorkspace lazily and idempotently provisions one default personal
// workspace per tenant on first binding/resolution access (FR-002, FR-025). Concurrent
// first-access converges on a single record via the partial unique index.
func (s *SQLiteStore) EnsureDefaultWorkspace(ctx context.Context, tenantID string) (bindings.Workspace, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return bindings.Workspace{}, errors.New("tenant id is required")
	}
	if ws, found, err := s.defaultWorkspace(ctx, tenantID); err != nil || found {
		return ws, err
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
	if err := s.insertWorkspace(ctx, ws); err != nil {
		// A concurrent first-access won the unique-default race; load the winner.
		if ws2, found, lookupErr := s.defaultWorkspace(ctx, tenantID); lookupErr == nil && found {
			return ws2, nil
		}
		return bindings.Workspace{}, err
	}
	return ws, nil
}

func (s *SQLiteStore) defaultWorkspace(ctx context.Context, tenantID string) (bindings.Workspace, bool, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM workspaces WHERE tenant_id = ? AND is_default = 1`, tenantID).Scan(&doc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bindings.Workspace{}, false, nil
		}
		return bindings.Workspace{}, false, err
	}
	return decodeWorkspace(doc)
}

func (s *SQLiteStore) insertWorkspace(ctx context.Context, ws bindings.Workspace) error {
	doc, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO workspaces (workspace_id, tenant_id, display_name, status, is_default, owner_principal_id, repair_status, redaction_status, created_at, updated_at, archived_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ws.WorkspaceID, ws.TenantID, bindings.SafeLabel(ws.DisplayName), string(ws.Status), boolToInt(ws.IsDefault), ws.OwnerPrincipalID, string(ws.RepairStatus), string(ws.RedactionStatus), ws.CreatedAt.Format(time.RFC3339Nano), ws.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(ws.ArchivedAt), doc)
	return err
}

// CreateWorkspace creates a non-default tenant-scoped workspace (FR-032).
func (s *SQLiteStore) CreateWorkspace(ctx context.Context, actor identity.TenantContext, displayName string) (bindings.Workspace, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.Workspace{}, "", bindings.ErrExplicitActorRequired
	}
	if err := bindings.ValidateWorkspaceMutation(bindings.WorkspaceMutationInput{DisplayName: displayName, Status: bindings.WorkspaceActive}); err != nil {
		return bindings.Workspace{}, "", err
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	ws := bindings.Workspace{
		WorkspaceID:      newStoreID("ws"),
		TenantID:         actor.TenantID,
		DisplayName:      bindings.SafeLabel(displayName),
		Status:           bindings.WorkspaceActive,
		IsDefault:        false,
		OwnerPrincipalID: actor.PrincipalID,
		RepairStatus:     bindings.RepairHealthy,
		RedactionStatus:  bindings.RedactionNotRequired,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.Workspace{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertWorkspaceTx(ctx, tx, ws); err != nil {
		return bindings.Workspace{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, WorkspaceID: ws.WorkspaceID, ActorPrincipalID: actor.PrincipalID, EventKind: "workspace.created", Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: "user_created_workspace", SafeSummary: "Workspace created", OccurredAt: now}); err != nil {
		return bindings.Workspace{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.Workspace{}, "", err
	}
	return ws, auditID, nil
}

// UpdateWorkspaceStatus archives or disables a workspace (FR-032). No hard delete.
func (s *SQLiteStore) UpdateWorkspaceStatus(ctx context.Context, actor identity.TenantContext, workspaceID string, status bindings.WorkspaceStatus) (bindings.Workspace, string, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return bindings.Workspace{}, "", bindings.ErrExplicitActorRequired
	}
	switch status {
	case bindings.WorkspaceArchived, bindings.WorkspaceDisabled, bindings.WorkspaceActive:
	default:
		return bindings.Workspace{}, "", bindings.InvalidBindingReason("workspace_status_invalid")
	}
	ws, found, err := s.GetWorkspace(ctx, actor.TenantID, workspaceID)
	if err != nil {
		return bindings.Workspace{}, "", err
	}
	if !found {
		return bindings.Workspace{}, "", ErrWorkspaceNotFound
	}
	if ws.IsDefault && status != bindings.WorkspaceActive {
		return bindings.Workspace{}, "", bindings.InvalidBindingReason("default_workspace_not_retirable")
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_binding")
	ws.Status = status
	ws.UpdatedAt = now
	if status == bindings.WorkspaceArchived {
		ws.ArchivedAt = &now
		ws.RepairStatus = bindings.RepairDisabled
	} else if status == bindings.WorkspaceDisabled {
		ws.RepairStatus = bindings.RepairDisabled
	} else {
		ws.RepairStatus = bindings.RepairHealthy
		ws.ArchivedAt = nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return bindings.Workspace{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateWorkspaceTx(ctx, tx, ws); err != nil {
		return bindings.Workspace{}, "", err
	}
	if err := insertBindingAuditTx(ctx, tx, bindingAuditRow{AuditEventID: auditID, TenantID: actor.TenantID, WorkspaceID: ws.WorkspaceID, ActorPrincipalID: actor.PrincipalID, EventKind: "workspace." + string(status), Outcome: "succeeded", PermissionGate: string(identity.PermissionBindingsManage), ReasonCode: "user_updated_workspace", SafeSummary: "Workspace " + string(status), OccurredAt: now}); err != nil {
		return bindings.Workspace{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return bindings.Workspace{}, "", err
	}
	return ws, auditID, nil
}

// ListWorkspaces returns tenant workspaces, default first.
func (s *SQLiteStore) ListWorkspaces(ctx context.Context, tenantID string, limit int) ([]bindings.Workspace, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// Ensure the lazy default exists so listing always shows it (FR-025).
	if _, err := s.EnsureDefaultWorkspace(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM workspaces WHERE tenant_id = ? ORDER BY is_default DESC, updated_at DESC, workspace_id DESC LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]bindings.Workspace, 0)
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		ws, _, err := decodeWorkspace(doc)
		if err != nil {
			return nil, err
		}
		items = append(items, ws)
	}
	return items, rows.Err()
}

// GetWorkspace returns one workspace by id within the tenant.
func (s *SQLiteStore) GetWorkspace(ctx context.Context, tenantID, workspaceID string) (bindings.Workspace, bool, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM workspaces WHERE tenant_id = ? AND workspace_id = ?`, strings.TrimSpace(tenantID), strings.TrimSpace(workspaceID)).Scan(&doc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bindings.Workspace{}, false, nil
		}
		return bindings.Workspace{}, false, err
	}
	ws, _, err := decodeWorkspace(doc)
	if err != nil {
		return bindings.Workspace{}, false, err
	}
	return ws, true, nil
}

// workspaceSelectable reports whether a workspace exists and is active for the tenant.
func (s *SQLiteStore) workspaceSelectable(ctx context.Context, tenantID, workspaceID string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return false, nil
	}
	ws, found, err := s.GetWorkspace(ctx, tenantID, workspaceID)
	if err != nil || !found {
		return false, err
	}
	return ws.Status == bindings.WorkspaceActive, nil
}

func decodeWorkspace(doc []byte) (bindings.Workspace, bool, error) {
	var ws bindings.Workspace
	if err := json.Unmarshal(doc, &ws); err != nil {
		return bindings.Workspace{}, false, err
	}
	return ws, true, nil
}

func insertWorkspaceTx(ctx context.Context, tx *sql.Tx, ws bindings.Workspace) error {
	doc, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO workspaces (workspace_id, tenant_id, display_name, status, is_default, owner_principal_id, repair_status, redaction_status, created_at, updated_at, archived_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ws.WorkspaceID, ws.TenantID, bindings.SafeLabel(ws.DisplayName), string(ws.Status), boolToInt(ws.IsDefault), ws.OwnerPrincipalID, string(ws.RepairStatus), string(ws.RedactionStatus), ws.CreatedAt.Format(time.RFC3339Nano), ws.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(ws.ArchivedAt), doc)
	return err
}

func updateWorkspaceTx(ctx context.Context, tx *sql.Tx, ws bindings.Workspace) error {
	doc, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE workspaces SET display_name = ?, status = ?, repair_status = ?, redaction_status = ?, updated_at = ?, archived_at = ?, document_json = ? WHERE tenant_id = ? AND workspace_id = ?`,
		bindings.SafeLabel(ws.DisplayName), string(ws.Status), string(ws.RepairStatus), string(ws.RedactionStatus), ws.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(ws.ArchivedAt), doc, ws.TenantID, ws.WorkspaceID)
	return err
}
