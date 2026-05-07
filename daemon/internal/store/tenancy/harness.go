package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Harness is the tenant-aware accessor for the cross-cutting harness +
// policy tables: connector_messages, sandbox_executions,
// consumer_policy_records, provider_preferences, mcp_tool_exposure_rules,
// and secret_scope_bindings.
//
// Roadmap 37 boundary (binding constraint, enforced by T089c): the
// helpers here ONLY (a) write/read tenant_id, (b) scope queries by
// tenant. They MUST NOT participate in secret value storage/retrieval,
// OAuth/provider auth lifecycle, secret reference resolution, connector
// /MCP/sandbox policy administration, or credential redaction. Those
// surfaces remain owned by Roadmap 37; their public signatures are
// snapshotted by T089c.
type Harness struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewHarness(s *store.SQLiteStore, emitter *audit.Emitter) *Harness {
	return &Harness{store: s, emitter: emitter}
}

func (a *Harness) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

// UpsertConnectorMessageForTenant binds the connector message row's
// tenant_id. Per T076b's resolution rule, tenant ownership comes from
// session_id → sessions.tenant_id when non-NULL, else run_id → runs.tenant_id,
// else default personal tenant; the call here uses the caller's
// resolved context tenant, which is correct for new (post-Roadmap-35)
// writes.
func (a *Harness) UpsertConnectorMessageForTenant(ctx context.Context, message imtypes.MessageRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertConnectorMessage(ctx, message); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "connector_messages", "delivery_id", message.DeliveryID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertConnectorMessageForTenant", "connector_message")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Harness) CreateConnectorMessageIfAbsentForTenant(ctx context.Context, message imtypes.MessageRecord) (imtypes.MessageRecord, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return imtypes.MessageRecord{}, false, err
	}
	message.TenantID = tenantID
	record, created, err := a.store.CreateConnectorMessageIfAbsent(ctx, message)
	if err != nil {
		return imtypes.MessageRecord{}, false, err
	}
	if err := a.store.BindRowTenant(ctx, "connector_messages", "delivery_id", record.DeliveryID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:CreateConnectorMessageIfAbsentForTenant", "connector_message")
			return imtypes.MessageRecord{}, false, ErrCrossTenantWrite
		}
		return imtypes.MessageRecord{}, false, err
	}
	return record, created, nil
}

func (a *Harness) UpsertSandboxExecutionForTenant(ctx context.Context, record store.SandboxExecutionRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertSandboxExecution(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "sandbox_executions", "execution_id", record.ExecutionID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertSandboxExecutionForTenant", "sandbox_execution")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Harness) UpsertConsumerPolicyRecordForTenant(ctx context.Context, record store.ConsumerPolicyRecordRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertConsumerPolicyRecord(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "consumer_policy_records", "policy_record_id", record.PolicyRecordID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertConsumerPolicyRecordForTenant", "consumer_policy_record")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Harness) UpsertProviderPreferenceForTenant(ctx context.Context, preference providers.Preference) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertProviderPreference(ctx, preference); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "provider_preferences", "provider_id", preference.ProviderID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertProviderPreferenceForTenant", "provider_preference")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// UpsertSecretScopeBindingForTenant binds the row's tenant_id. The
// secret_ref / secret value handling and OAuth/provider auth lifecycle
// remain owned by Roadmap 37 — this helper does not touch them.
func (a *Harness) UpsertSecretScopeBindingForTenant(ctx context.Context, record store.SecretScopeBindingRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertSecretScopeBinding(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "secret_scope_bindings", "binding_id", record.BindingID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertSecretScopeBindingForTenant", "secret_scope_binding")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// UpsertMCPToolExposureRuleForTenant binds the rule row's tenant_id.
// The composite PK (server_id, tool_name, runtime_surface) is opaque to
// BindRowTenant, so the helper updates by the composite key directly.
func (a *Harness) UpsertMCPToolExposureRuleForTenant(ctx context.Context, record store.MCPToolExposureRuleRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertMCPToolExposureRule(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindMCPToolExposureRuleTenant(ctx, record.ServerID, record.ToolName, record.RuntimeSurface, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMCPToolExposureRuleForTenant", "mcp_tool_exposure_rule")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
