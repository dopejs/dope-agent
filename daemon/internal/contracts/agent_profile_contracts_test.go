package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestAgentProfileSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/agent-profile-resource.schema.json":                   `{"profileId":"prof_1","tenantId":"ten_57","displayName":"Support Agent","displayIdentity":{"name":"Support","safeSummary":"Support"},"persona":{"tone":"direct","safeSummary":"Direct support persona"},"defaultProviderPreference":{"providerId":"openai_compatible","model":"gpt-test","validationState":"valid"},"safetyDefaults":{"approvalPosture":"ask","validationState":"valid"},"status":"active","activeVersionId":"profv_1","tenantDefault":true,"overlayReferenceCount":1,"createdAt":"2026-05-12T10:00:00Z","updatedAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/agent-profile-version-resource.schema.json":           `{"profileVersionId":"profv_1","profileId":"prof_1","tenantId":"ten_57","versionNumber":1,"changeKind":"created","changeSummary":"Created profile","snapshot":{"profileId":"prof_1","displayName":"Support Agent","displayIdentity":{},"persona":{},"defaultProviderPreference":{},"safetyDefaults":{},"status":"active","tenantDefault":true,"overlayReferenceCount":0,"createdAt":"2026-05-12T10:00:00Z","updatedAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"},"rollbackEligibility":"eligible","createdAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/agent-profile-overlay-reference.schema.json":          `{"overlayReferenceId":"ovr_1","profileId":"prof_1","profileVersionId":"profv_1","tenantId":"ten_57","referenceKind":"prompt","scope":"profile","referenceUri":"prompt://profile/support","safeDisplayLabel":"support","validationState":"valid","createdAt":"2026-05-12T10:00:00Z","updatedAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/agent-profile-runtime-projection.schema.json":         `{"runtimeProfileProjectionId":"rpp_1","tenantId":"ten_57","profileId":"prof_1","profileVersionId":"profv_1","selectionId":"sel_1","resourceKind":"thread","resourceId":"thr_1","threadId":"thr_1","selectionScope":"tenant_default","selectionReason":"user_activated","safeDisplayName":"Support Agent","safeSummary":"Direct support persona","configurationScope":"explicit_profile_configuration","deferredBindingClassification":"roadmap_58_deferred_binding_unapplied","occurredAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/agent-profile-list.response.schema.json":              `{"tenantId":"ten_57","page":{"limit":20,"order":"updated_at_desc"},"items":[{"profileId":"prof_1","displayName":"Support Agent","displayIdentity":{},"persona":{},"defaultProviderPreference":{},"safetyDefaults":{},"status":"active","tenantDefault":true,"overlayReferenceCount":0,"createdAt":"2026-05-12T10:00:00Z","updatedAt":"2026-05-12T10:00:00Z","redactionStatus":"redacted"}]}`,
		"schemas/events/agent-profile-lifecycle.event.schema.json":         `{"eventId":"evt_profile_1","category":"agent_profile","name":"agent_profile.archived","occurredAt":"2026-05-12T10:00:00Z","resource":{"kind":"agent_profile","id":"prof_1"},"payload":{"profileId":"prof_1","profileVersionId":"profv_2","outcome":"succeeded","reasonCode":"operator_retired_profile","permissionGate":"profiles.manage","safeSummary":"Profile retired","redactionStatus":"redacted"}}`,
		"schemas/events/agent-profile-version-created.event.schema.json":   `{"eventId":"evt_profile_2","category":"agent_profile","name":"agent_profile.version_created","occurredAt":"2026-05-12T10:00:00Z","resource":{"kind":"agent_profile_version","id":"profv_2"},"payload":{"profileId":"prof_1","profileVersionId":"profv_2","changeKind":"updated","versionNumber":2,"reasonCode":"user_updated_profile","redactionStatus":"redacted"}}`,
		"schemas/events/agent-profile-runtime-projected.event.schema.json": `{"eventId":"evt_profile_3","category":"agent_profile","name":"agent_profile.runtime_projected","occurredAt":"2026-05-12T10:00:00Z","resource":{"kind":"thread","id":"thr_1"},"payload":{"runtimeProfileProjectionId":"rpp_1","profileId":"prof_1","profileVersionId":"profv_1","selectionId":"sel_1","selectionScope":"tenant_default","selectionReason":"user_activated","safeDisplayName":"Support Agent","safeSummary":"Direct support persona","redactionStatus":"redacted"}}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
