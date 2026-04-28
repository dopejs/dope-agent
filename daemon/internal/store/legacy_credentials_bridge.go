package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

const legacyMCPStatusDisabled = "disabled"

func (s *SQLiteStore) BridgeLegacyCredentialResources(ctx context.Context, input secrets.LegacyCredentialResourceBridgeInput) (secrets.LegacyCredentialResourceBridgeResult, error) {
	var result secrets.LegacyCredentialResourceBridgeResult
	if s == nil {
		return result, nil
	}
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return result, secrets.ErrTenantRequired
	}
	activeRefs := stringSet(input.ActiveSecretRefs)
	disabledRefs := stringSet(input.DisabledSecretRefs)
	now := time.Now().UTC()

	providerResult, err := s.bridgeLegacyProviderAuthStates(ctx, tenantID, now)
	if err != nil {
		return result, err
	}
	result.Bridged = append(result.Bridged, providerResult...)

	integrationResult, err := s.bridgeLegacyIntegrations(ctx, tenantID, now)
	if err != nil {
		return result, err
	}
	result.Bridged = append(result.Bridged, integrationResult...)

	connectorResult, connectorDisabled, err := s.bridgeLegacyConnectors(ctx, tenantID, activeRefs, disabledRefs, now)
	if err != nil {
		return result, err
	}
	result.Bridged = append(result.Bridged, connectorResult...)
	result.Disabled = append(result.Disabled, connectorDisabled...)

	mcpResult, mcpDisabled, err := s.bridgeLegacyMCPResources(ctx, tenantID, activeRefs, disabledRefs, now)
	if err != nil {
		return result, err
	}
	result.Bridged = append(result.Bridged, mcpResult...)
	result.Disabled = append(result.Disabled, mcpDisabled...)

	return result, nil
}

func (s *SQLiteStore) bridgeLegacyProviderAuthStates(ctx context.Context, tenantID string, now time.Time) ([]secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, status
		FROM provider_auth_states
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY provider_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list legacy provider auth states: %w", err)
	}

	type providerAuthRow struct {
		providerID string
		status     string
	}
	var rowsData []providerAuthRow
	for rows.Next() {
		var item providerAuthRow
		if err := rows.Scan(&item.providerID, &item.status); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan legacy provider auth state: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var items []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE provider_auth_states
			SET tenant_id = ?
			WHERE provider_id = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, row.providerID, tenantID); err != nil {
			return nil, fmt.Errorf("bridge provider auth state %s: %w", row.providerID, err)
		}
		items = append(items, secrets.BridgedCredentialResource{
			TenantID:     tenantID,
			ResourceKind: secrets.ResourceKindProviderAuthState,
			ResourceID:   row.providerID,
			Status:       row.status,
			UpdatedAt:    now,
		})
	}
	return items, nil
}

func (s *SQLiteStore) bridgeLegacyIntegrations(ctx context.Context, tenantID string, now time.Time) ([]secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT integration_id, readiness_status, document_json
		FROM integrations
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY integration_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list legacy integrations: %w", err)
	}

	type integrationRow struct {
		integrationID string
		status        string
		documentRaw   string
	}
	var rowsData []integrationRow
	for rows.Next() {
		var item integrationRow
		if err := rows.Scan(&item.integrationID, &item.status, &item.documentRaw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan legacy integration: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var items []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		document, err := patchJSONField([]byte(row.documentRaw), map[string]any{"tenantId": tenantID})
		if err != nil {
			return nil, fmt.Errorf("patch legacy integration %s: %w", row.integrationID, err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE integrations
			SET tenant_id = ?, document_json = ?
			WHERE integration_id = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, string(document), row.integrationID, tenantID); err != nil {
			return nil, fmt.Errorf("bridge integration %s: %w", row.integrationID, err)
		}
		items = append(items, secrets.BridgedCredentialResource{
			TenantID:     tenantID,
			ResourceKind: secrets.ResourceKindIntegration,
			ResourceID:   row.integrationID,
			Status:       row.status,
			UpdatedAt:    now,
		})
	}
	return items, nil
}

func (s *SQLiteStore) bridgeLegacyConnectors(ctx context.Context, tenantID string, activeRefs, disabledRefs map[string]struct{}, now time.Time) ([]secrets.BridgedCredentialResource, []secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, status, COALESCE(disabled_reason, ''), secret_refs_json
		FROM connectors
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY connector_id ASC
	`, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("list legacy connectors: %w", err)
	}

	type connectorRow struct {
		connectorID    string
		status         string
		disabledReason string
		refsRaw        string
	}
	var rowsData []connectorRow
	for rows.Next() {
		var item connectorRow
		if err := rows.Scan(&item.connectorID, &item.status, &item.disabledReason, &item.refsRaw); err != nil {
			_ = rows.Close()
			return nil, nil, fmt.Errorf("scan legacy connector: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	var bridged []secrets.BridgedCredentialResource
	var disabled []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		refs, err := parseSecretRefsJSON(row.refsRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("parse legacy connector refs %s: %w", row.connectorID, err)
		}
		status := row.status
		disabledReason := row.disabledReason
		if reason := legacyCredentialUnsafeReason(refs, activeRefs, disabledRefs); reason != "" {
			status = string(connectors.StatusDisabled)
			disabledReason = reason
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE connectors
			SET tenant_id = ?, status = ?, disabled_reason = NULLIF(?, ''), updated_at = ?
			WHERE connector_id = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, status, disabledReason, now.Format(time.RFC3339Nano), row.connectorID, tenantID); err != nil {
			return nil, nil, fmt.Errorf("bridge connector %s: %w", row.connectorID, err)
		}
		item := secrets.BridgedCredentialResource{
			TenantID:       tenantID,
			ResourceKind:   secrets.ResourceKindConnector,
			ResourceID:     row.connectorID,
			Status:         status,
			DisabledReason: disabledReason,
			SecretRefs:     refs,
			UpdatedAt:      now,
		}
		bridged = append(bridged, item)
		if disabledReason != "" && status == string(connectors.StatusDisabled) {
			disabled = append(disabled, item)
		}
	}
	return bridged, disabled, nil
}

func (s *SQLiteStore) bridgeLegacyMCPResources(ctx context.Context, tenantID string, activeRefs, disabledRefs map[string]struct{}, now time.Time) ([]secrets.BridgedCredentialResource, []secrets.BridgedCredentialResource, error) {
	bridged, disabled, err := s.bridgeLegacyMCPServers(ctx, tenantID, activeRefs, disabledRefs, now)
	if err != nil {
		return nil, nil, err
	}
	stateItems, err := s.bindLegacyMCPServerStates(ctx, tenantID, now)
	if err != nil {
		return nil, nil, err
	}
	bridged = append(bridged, stateItems...)
	toolItems, err := s.bindLegacyMCPTools(ctx, tenantID, now)
	if err != nil {
		return nil, nil, err
	}
	bridged = append(bridged, toolItems...)
	exposureItems, err := s.bindLegacyMCPToolExposureRules(ctx, tenantID, now)
	if err != nil {
		return nil, nil, err
	}
	bridged = append(bridged, exposureItems...)
	return bridged, disabled, nil
}

func (s *SQLiteStore) bridgeLegacyMCPServers(ctx context.Context, tenantID string, activeRefs, disabledRefs map[string]struct{}, now time.Time) ([]secrets.BridgedCredentialResource, []secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, enabled, document_json
		FROM mcp_servers
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY server_id ASC
	`, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("list legacy mcp servers: %w", err)
	}

	type mcpServerRow struct {
		serverID    string
		enabledInt  int
		documentRaw string
	}
	var rowsData []mcpServerRow
	for rows.Next() {
		var item mcpServerRow
		if err := rows.Scan(&item.serverID, &item.enabledInt, &item.documentRaw); err != nil {
			_ = rows.Close()
			return nil, nil, fmt.Errorf("scan legacy mcp server: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	var bridged []secrets.BridgedCredentialResource
	var disabled []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		document, refs, err := patchLegacyMCPServerDocument([]byte(row.documentRaw), tenantID, activeRefs, disabledRefs)
		if err != nil {
			return nil, nil, fmt.Errorf("patch legacy mcp server %s: %w", row.serverID, err)
		}
		enabled := row.enabledInt == 1
		disabledReason := legacyCredentialUnsafeReason(refs, activeRefs, disabledRefs)
		if disabledReason != "" {
			enabled = false
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mcp_servers
			SET tenant_id = ?, enabled = ?, document_json = ?, updated_at = ?
			WHERE server_id = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, boolToInt(enabled), string(document), now.Format(time.RFC3339Nano), row.serverID, tenantID); err != nil {
			return nil, nil, fmt.Errorf("bridge mcp server %s: %w", row.serverID, err)
		}
		if disabledReason != "" {
			if err := s.disableLegacyMCPServerState(ctx, row.serverID, tenantID, disabledReason, now); err != nil {
				return nil, nil, err
			}
		}
		status := "enabled"
		if !enabled {
			status = legacyMCPStatusDisabled
		}
		item := secrets.BridgedCredentialResource{
			TenantID:       tenantID,
			ResourceKind:   secrets.ResourceKindMCPServer,
			ResourceID:     row.serverID,
			Status:         status,
			DisabledReason: disabledReason,
			SecretRefs:     refs,
			UpdatedAt:      now,
		}
		bridged = append(bridged, item)
		if disabledReason != "" {
			disabled = append(disabled, item)
		}
	}
	return bridged, disabled, nil
}

func (s *SQLiteStore) disableLegacyMCPServerState(ctx context.Context, serverID, tenantID, reason string, now time.Time) error {
	var documentRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM mcp_server_states
		WHERE server_id = ?
	`, serverID).Scan(&documentRaw)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load legacy mcp server state %s: %w", serverID, err)
	}
	document := []byte(`{}`)
	if documentRaw.Valid && strings.TrimSpace(documentRaw.String) != "" {
		var patchErr error
		document, patchErr = patchJSONField([]byte(documentRaw.String), map[string]any{
			"status":       legacyMCPStatusDisabled,
			"healthReason": reason,
		})
		if patchErr != nil {
			return fmt.Errorf("patch legacy mcp server state %s: %w", serverID, patchErr)
		}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mcp_server_states (server_id, status, updated_at, document_json, tenant_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			status = excluded.status,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mcp_server_states.tenant_id, excluded.tenant_id)
	`, serverID, legacyMCPStatusDisabled, now.Format(time.RFC3339Nano), string(document), tenantID)
	if err != nil {
		return fmt.Errorf("disable legacy mcp server state %s: %w", serverID, err)
	}
	return nil
}

func (s *SQLiteStore) bindLegacyMCPServerStates(ctx context.Context, tenantID string, now time.Time) ([]secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, status, document_json
		FROM mcp_server_states
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY server_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list legacy mcp server states: %w", err)
	}

	type mcpStateRow struct {
		serverID    string
		status      string
		documentRaw string
	}
	var rowsData []mcpStateRow
	for rows.Next() {
		var item mcpStateRow
		if err := rows.Scan(&item.serverID, &item.status, &item.documentRaw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan legacy mcp server state: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var items []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		document, err := patchJSONField([]byte(row.documentRaw), map[string]any{"tenantId": tenantID})
		if err != nil {
			return nil, fmt.Errorf("patch legacy mcp server state %s: %w", row.serverID, err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mcp_server_states
			SET tenant_id = ?, document_json = ?, updated_at = ?
			WHERE server_id = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, string(document), now.Format(time.RFC3339Nano), row.serverID, tenantID); err != nil {
			return nil, fmt.Errorf("bridge mcp server state %s: %w", row.serverID, err)
		}
		items = append(items, secrets.BridgedCredentialResource{
			TenantID:     tenantID,
			ResourceKind: secrets.ResourceKindMCPServer,
			ResourceID:   row.serverID + ":state",
			Status:       row.status,
			UpdatedAt:    now,
		})
	}
	return items, nil
}

func (s *SQLiteStore) bindLegacyMCPTools(ctx context.Context, tenantID string, now time.Time) ([]secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, tool_name, discovery_status, document_json
		FROM mcp_tools
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY server_id ASC, tool_name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list legacy mcp tools: %w", err)
	}

	type mcpToolRow struct {
		serverID    string
		toolName    string
		status      string
		documentRaw string
	}
	var rowsData []mcpToolRow
	for rows.Next() {
		var item mcpToolRow
		if err := rows.Scan(&item.serverID, &item.toolName, &item.status, &item.documentRaw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan legacy mcp tool: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var items []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		document, err := patchJSONField([]byte(row.documentRaw), map[string]any{"tenantId": tenantID})
		if err != nil {
			return nil, fmt.Errorf("patch legacy mcp tool %s/%s: %w", row.serverID, row.toolName, err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mcp_tools
			SET tenant_id = ?, document_json = ?, updated_at = ?
			WHERE server_id = ? AND tool_name = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, string(document), now.Format(time.RFC3339Nano), row.serverID, row.toolName, tenantID); err != nil {
			return nil, fmt.Errorf("bridge mcp tool %s/%s: %w", row.serverID, row.toolName, err)
		}
		items = append(items, secrets.BridgedCredentialResource{
			TenantID:     tenantID,
			ResourceKind: secrets.ResourceKindMCPTool,
			ResourceID:   row.serverID + ":" + row.toolName,
			Status:       row.status,
			UpdatedAt:    now,
		})
	}
	return items, nil
}

func (s *SQLiteStore) bindLegacyMCPToolExposureRules(ctx context.Context, tenantID string, now time.Time) ([]secrets.BridgedCredentialResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, tool_name, runtime_surface, exposure_mode, document_json
		FROM mcp_tool_exposure_rules
		WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?
		ORDER BY server_id ASC, tool_name ASC, runtime_surface ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list legacy mcp tool exposure rules: %w", err)
	}

	type mcpExposureRow struct {
		serverID    string
		toolName    string
		surface     string
		status      string
		documentRaw string
	}
	var rowsData []mcpExposureRow
	for rows.Next() {
		var item mcpExposureRow
		if err := rows.Scan(&item.serverID, &item.toolName, &item.surface, &item.status, &item.documentRaw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan legacy mcp tool exposure rule: %w", err)
		}
		rowsData = append(rowsData, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var items []secrets.BridgedCredentialResource
	for _, row := range rowsData {
		document, err := patchJSONField([]byte(row.documentRaw), map[string]any{"tenantId": tenantID})
		if err != nil {
			return nil, fmt.Errorf("patch legacy mcp tool exposure rule %s/%s/%s: %w", row.serverID, row.toolName, row.surface, err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mcp_tool_exposure_rules
			SET tenant_id = ?, document_json = ?, updated_at = ?
			WHERE server_id = ? AND tool_name = ? AND runtime_surface = ? AND (tenant_id IS NULL OR tenant_id = '' OR tenant_id = ?)
		`, tenantID, string(document), now.Format(time.RFC3339Nano), row.serverID, row.toolName, row.surface, tenantID); err != nil {
			return nil, fmt.Errorf("bridge mcp tool exposure rule %s/%s/%s: %w", row.serverID, row.toolName, row.surface, err)
		}
		items = append(items, secrets.BridgedCredentialResource{
			TenantID:     tenantID,
			ResourceKind: secrets.ResourceKindMCPTool,
			ResourceID:   row.serverID + ":" + row.toolName + ":" + row.surface,
			Status:       row.status,
			UpdatedAt:    now,
		})
	}
	return items, nil
}

func patchLegacyMCPServerDocument(payload []byte, tenantID string, activeRefs, disabledRefs map[string]struct{}) ([]byte, []string, error) {
	var document map[string]any
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &document); err != nil {
			return nil, nil, err
		}
	}
	if document == nil {
		document = map[string]any{}
	}
	refs := secretRefsFromDocument(document)
	document["tenantId"] = tenantID
	if legacyCredentialUnsafeReason(refs, activeRefs, disabledRefs) != "" {
		document["enabled"] = false
	}
	next, err := json.Marshal(document)
	return next, refs, err
}

func patchJSONField(payload []byte, fields map[string]any) ([]byte, error) {
	var document map[string]any
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &document); err != nil {
			return nil, err
		}
	}
	if document == nil {
		document = map[string]any{}
	}
	for key, value := range fields {
		document[key] = value
	}
	return json.Marshal(document)
}

func parseSecretRefsJSON(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var refs []string
	if err := json.Unmarshal([]byte(raw), &refs); err != nil {
		return nil, err
	}
	return cleanLegacyRefs(refs), nil
}

func secretRefsFromDocument(document map[string]any) []string {
	raw, ok := document["secretRefs"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	refs := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			refs = append(refs, value)
		}
	}
	return cleanLegacyRefs(refs)
}

func legacyCredentialUnsafeReason(refs []string, activeRefs, disabledRefs map[string]struct{}) string {
	refs = cleanLegacyRefs(refs)
	if len(refs) == 0 {
		return ""
	}
	for _, ref := range refs {
		if _, ok := disabledRefs[ref]; ok {
			return "legacy_secret_ref_conflict"
		}
	}
	for _, ref := range refs {
		if _, ok := activeRefs[ref]; !ok {
			return "legacy_secret_ref_unavailable"
		}
	}
	return ""
}

func cleanLegacyRefs(refs []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		items = append(items, ref)
	}
	return items
}

func stringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = struct{}{}
		}
	}
	return set
}
