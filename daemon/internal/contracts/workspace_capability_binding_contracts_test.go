package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestWorkspaceCapabilityBindingSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/workspace-resource.schema.json":                     `{"workspaceId":"ws_1","tenantId":"ten_58","displayName":"Personal Workspace","status":"active","isDefault":true,"repairStatus":"healthy","redactionStatus":"not_required","createdAt":"2026-06-03T10:00:00Z","updatedAt":"2026-06-03T10:00:00Z"}`,
		"schemas/api/binding-rule-resource.schema.json":                  `{"bindingId":"bnd_1","tenantId":"ten_58","scopeKind":"channel","scopeLabel":"discord:c1","selectedProfileId":"prof_1","selectedWorkspaceId":"ws_1","status":"active","repairStatus":"healthy","validationStatus":"valid","resultingSelectionSummary":"profile+workspace","lastMaterialChangeAt":"2026-06-03T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/capability-visibility-policy.schema.json":           `{"policyId":"cvp_1","tenantId":"ten_58","scopeKind":"workspace","scopeRef":"ws_1","capabilityId":"tool.shell","visibility":"hidden","validationStatus":"valid"}`,
		"schemas/api/binding-runtime-evidence.schema.json":               `{"projectionId":"brp_1","resourceKind":"thread","resourceId":"thr_1","selectedProfileId":"prof_1","selectedWorkspaceId":"ws_1","bindingScope":"channel","bindingId":"bnd_1","classification":"applied_binding","selectionReason":"explicit_binding_selection","capabilityVisibilitySummary":[{"capabilityId":"tool.shell","effective":"hidden","defaultEnabled":false,"offered":false,"executable":false,"reason":"hidden_by_policy","scope":"workspace"}],"occurredAt":"2026-06-03T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/effective-binding-selection.schema.json":            `{"outcome":"repair_required","bindingScope":"channel","selectedProfileId":"prof_1","selectedWorkspaceId":"ws_1","repairStatus":"invalid","repairReason":"selected_profile_unavailable"}`,
		"schemas/api/binding-repair-status.schema.json":                  `"needs_repair"`,
		"schemas/api/workspace-list.response.schema.json":                `{"tenantId":"ten_58","workspaces":[{"workspaceId":"ws_1","displayName":"Personal Workspace","status":"active","isDefault":true,"repairStatus":"healthy","redactionStatus":"not_required","createdAt":"2026-06-03T10:00:00Z","updatedAt":"2026-06-03T10:00:00Z"}]}`,
		"schemas/api/binding-list.response.schema.json":                  `{"tenantId":"ten_58","bindings":[{"bindingId":"bnd_1","scopeKind":"channel","scopeLabel":"discord:c1","status":"active","repairStatus":"healthy","validationStatus":"valid","lastMaterialChangeAt":"2026-06-03T10:00:00Z","redactionStatus":"redacted"}]}`,
		"schemas/api/create-binding.request.schema.json":                 `{"scopeKind":"channel","scopeRef":"discord:c1","selectedProfileId":"prof_1","selectedWorkspaceId":"ws_1"}`,
		"schemas/api/update-binding.request.schema.json":                 `{"disable":true,"reasonCode":"operator_disabled"}`,
		"schemas/api/create-workspace.request.schema.json":               `{"displayName":"Project X"}`,
		"schemas/api/update-workspace.request.schema.json":               `{"status":"archived"}`,
		"schemas/api/update-capability-visibility.request.schema.json":   `{"scopeKind":"workspace","scopeRef":"ws_1","capabilityId":"tool.shell","visibility":"hidden"}`,
		"schemas/events/binding-lifecycle.event.schema.json":             `{"eventId":"evt_b1","category":"binding","name":"binding.created","occurredAt":"2026-06-03T10:00:00Z","resource":{"kind":"workspace_capability_binding","id":"bnd_1"},"payload":{"bindingId":"bnd_1","actorPrincipalId":"prn_admin","outcome":"succeeded","reasonCode":"user_created_binding","permissionGate":"bindings.manage","safeSummary":"Binding created","redactionStatus":"redacted"}}`,
		"schemas/events/capability-visibility-changed.event.schema.json": `{"eventId":"evt_b2","category":"binding","name":"capability_visibility.changed","occurredAt":"2026-06-03T10:00:00Z","resource":{"kind":"capability_visibility_policy","id":"tool.shell"},"payload":{"actorPrincipalId":"prn_admin","scopeKind":"workspace","scopeRef":"ws_1","capabilityId":"tool.shell","visibility":"hidden","redactionStatus":"redacted"}}`,
		"schemas/events/binding-runtime-projected.event.schema.json":     `{"eventId":"evt_b3","category":"binding","name":"binding.runtime_projected","occurredAt":"2026-06-03T10:00:00Z","resource":{"kind":"thread","id":"thr_1"},"payload":{"projectionId":"brp_1","selectedProfileId":"prof_1","selectedWorkspaceId":"ws_1","bindingScope":"channel","bindingId":"bnd_1","classification":"applied_binding","selectionReason":"explicit_binding_selection","redactionStatus":"redacted"}}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}

// B22/FR-026: the profile runtime projection schema accepts the applied-binding
// classification once Roadmap 58 flips the planted deferred marker.
func TestProfileRuntimeProjectionAcceptsAppliedBindingClassification(t *testing.T) {
	t.Parallel()
	validator := contracts.NewValidator(schemaRootDir(t))
	applied := `{"runtimeProfileProjectionId":"rpp_1","tenantId":"ten_58","profileId":"prof_1","profileVersionId":"profv_1","selectionId":"sel_1","resourceKind":"thread","resourceId":"thr_1","threadId":"thr_1","selectionScope":"tenant_default","selectionReason":"user_activated","safeDisplayName":"Support Agent","safeSummary":"Direct support persona","configurationScope":"explicit_profile_configuration","deferredBindingClassification":"roadmap_58_applied_binding","occurredAt":"2026-06-03T10:00:00Z","redactionStatus":"redacted"}`
	if err := validator.ValidateRelative("schemas/api/agent-profile-runtime-projection.schema.json", []byte(applied)); err != nil {
		t.Fatalf("applied classification rejected: %v", err)
	}
}
