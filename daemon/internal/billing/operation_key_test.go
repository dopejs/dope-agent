package billing

import "testing"

func TestOperationKeysAreStableAndTenantScoped(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"run", RunOperationKey(fixtureTenantA, fixtureClient, fixtureRunID), "tenant:ten_finite:run:client_fixture"},
		{"workflow", WorkflowOperationKey(fixtureTenantA, fixtureRunID, "workflow_1", fixtureClient), "tenant:ten_finite:workflow:run_fixture:workflow_1"},
		{"tool", ToolCallOperationKey(fixtureTenantA, fixtureRunID, fixtureStepID, "tool_1", fixtureClient), "tenant:ten_finite:tool_call:run_fixture:step_fixture:tool_1"},
		{"live", LiveValidationOperationKey(fixtureTenantA, "validation_1", fixtureClient), "tenant:ten_finite:live_validation:validation_1"},
		{"integration", IntegrationOperationKey(fixtureTenantA, "mail", "op_1", fixtureClient), "tenant:ten_finite:integration:mail:op_1"},
		{"artifact", ArtifactOperationKey(fixtureTenantA, "", "artifact/key", fixtureClient), "tenant:ten_finite:artifact:artifact/key"},
		{"evaluation", EvaluationOperationKey(fixtureTenantA, "candidate_1", "attempt_1", fixtureClient), "tenant:ten_finite:evaluation:candidate_1:attempt_1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("key=%q, want %q", tt.got, tt.want)
			}
		})
	}
	if RunOperationKey(fixtureTenantA, fixtureClient, fixtureRunID) == RunOperationKey(fixtureTenantB, fixtureClient, fixtureRunID) {
		t.Fatal("operation keys must include tenant identity")
	}
}
