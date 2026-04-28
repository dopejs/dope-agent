package tenancy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

const r37MCPToolsTable = "mcp_tools"

// R37Resources is the tenant-aware accessor for credential-bearing
// resources whose ownership moved from global/R35-boundary state into
// Roadmap 37: provider_auth_states, connectors, mcp_servers,
// mcp_server_states, and mcp_tools.
type R37Resources struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewR37Resources(s *store.SQLiteStore, emitter *audit.Emitter) *R37Resources {
	return &R37Resources{store: s, emitter: emitter}
}

func (a *R37Resources) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *R37Resources) UpsertProviderAuthStateForTenant(ctx context.Context, state providers.AuthState) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	state.TenantID = tenantID
	state.ProviderID = r37StorageKey(tenantID, state.ProviderID)
	if err := a.store.UpsertProviderAuthState(ctx, state); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "provider_auth_states", "provider_id", state.ProviderID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertProviderAuthStateForTenant", "provider_auth_state")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func r37StorageKey(tenantID, id string) string {
	return strings.TrimSpace(tenantID) + "::" + strings.TrimSpace(id)
}

func (a *R37Resources) UpsertConnectorForTenant(ctx context.Context, connector connectors.Connector) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.guardScalarTenant(ctx, "connectors", "connector_id", connector.ConnectorID, tenantID, "store:UpsertConnectorForTenant", "connector"); err != nil {
		return err
	}
	if err := a.store.UpsertConnector(ctx, connector); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "connectors", "connector_id", connector.ConnectorID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertConnectorForTenant", "connector")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *R37Resources) UpsertMCPServerForTenant(ctx context.Context, record store.MCPServerRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.guardScalarTenant(ctx, "mcp_servers", "server_id", record.ServerID, tenantID, "store:UpsertMCPServerForTenant", "mcp_server"); err != nil {
		return err
	}
	if err := a.store.UpsertMCPServer(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "mcp_servers", "server_id", record.ServerID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMCPServerForTenant", "mcp_server")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *R37Resources) UpsertMCPServerStateForTenant(ctx context.Context, record store.MCPServerStateRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.guardScalarTenant(ctx, "mcp_server_states", "server_id", record.ServerID, tenantID, "store:UpsertMCPServerStateForTenant", "mcp_server_state"); err != nil {
		return err
	}
	if err := a.store.UpsertMCPServerState(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "mcp_server_states", "server_id", record.ServerID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMCPServerStateForTenant", "mcp_server_state")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *R37Resources) UpsertMCPToolForTenant(ctx context.Context, record store.MCPToolRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.guardMCPToolTenant(ctx, record.ServerID, record.ToolName, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMCPToolForTenant", "mcp_tool")
			return ErrCrossTenantWrite
		}
		return err
	}
	if err := a.store.UpsertMCPTool(ctx, record); err != nil {
		return err
	}
	if err := a.bindMCPToolTenant(ctx, record.ServerID, record.ToolName, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMCPToolForTenant", "mcp_tool")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *R37Resources) ReplaceMCPToolsForTenant(ctx context.Context, serverID string, records []store.MCPToolRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.guardScalarTenant(ctx, "mcp_servers", "server_id", serverID, tenantID, "store:ReplaceMCPToolsForTenant", "mcp_server"); err != nil {
		return err
	}
	for _, record := range records {
		if err := a.guardMCPToolTenant(ctx, record.ServerID, record.ToolName, tenantID); err != nil {
			if errors.Is(err, store.ErrCrossTenantRow) {
				a.emit(ctx, "store:ReplaceMCPToolsForTenant", "mcp_tool")
				return ErrCrossTenantWrite
			}
			return err
		}
	}
	if err := a.store.ReplaceMCPTools(ctx, serverID, records); err != nil {
		return err
	}
	for _, record := range records {
		if err := a.bindMCPToolTenant(ctx, record.ServerID, record.ToolName, tenantID); err != nil {
			if errors.Is(err, store.ErrCrossTenantRow) {
				a.emit(ctx, "store:ReplaceMCPToolsForTenant", "mcp_tool")
				return ErrCrossTenantWrite
			}
			return err
		}
	}
	return nil
}

func (a *R37Resources) guardScalarTenant(ctx context.Context, table, pkColumn, pk, tenantID, surface, resourceKind string) error {
	owner, ok, err := a.store.LookupRowTenant(ctx, table, pkColumn, pk)
	if err != nil {
		return err
	}
	if ok && owner != "" && owner != tenantID {
		a.emit(ctx, surface, resourceKind)
		return ErrCrossTenantWrite
	}
	return nil
}

func (a *R37Resources) guardMCPToolTenant(ctx context.Context, serverID, toolName, tenantID string) error {
	var existing sql.NullString
	err := a.store.DB().QueryRowContext(ctx, `
		SELECT tenant_id FROM mcp_tools
		WHERE server_id = ? AND tool_name = ?
	`, serverID, toolName).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup tenant for %s: %w", r37MCPToolsTable, err)
	}
	if existing.Valid && existing.String != "" && existing.String != tenantID {
		return store.ErrCrossTenantRow
	}
	return nil
}

func (a *R37Resources) bindMCPToolTenant(ctx context.Context, serverID, toolName, tenantID string) error {
	if a == nil || a.store == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("bindMCPToolTenant: empty tenantID")
	}
	res, err := a.store.DB().ExecContext(ctx, `
		UPDATE mcp_tools
		SET tenant_id = ?
		WHERE server_id = ? AND tool_name = ? AND (tenant_id IS NULL OR tenant_id = ?)
	`, tenantID, serverID, toolName, tenantID)
	if err != nil {
		return fmt.Errorf("bind tenant for %s: %w", r37MCPToolsTable, err)
	}
	if rows, _ := res.RowsAffected(); rows != 0 {
		return nil
	}
	var existing sql.NullString
	err = a.store.DB().QueryRowContext(ctx, `
		SELECT tenant_id FROM mcp_tools
		WHERE server_id = ? AND tool_name = ?
	`, serverID, toolName).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup tenant for %s: %w", r37MCPToolsTable, err)
	}
	if existing.Valid && existing.String != "" && existing.String != tenantID {
		return store.ErrCrossTenantRow
	}
	return nil
}
